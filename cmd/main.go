package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/oksasatya/go-ddd-clean-architecture/configs"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/container"
	pginfra "github.com/oksasatya/go-ddd-clean-architecture/internal/infrastructure/postgres"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/interface/middleware"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/router"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

func main() {
	_ = godotenv.Load() // load .env if present

	cfg := configs.Load()
	logger := helpers.NewLogger(cfg.AppName, cfg.Env)
	gin.SetMode(cfg.GinMode)

	ctx := context.Background()

	// Initialize Postgres pool
	pool, err := pginfra.NewPool(ctx, cfg.PostgresDSN(), cfg.DBMaxConns, cfg.DBMinConns, cfg.DBMaxConnLife)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Run migrations using database/sql with pgx stdlib
	if err := runMigrations(cfg.PostgresDSN(), cfg.MigrationsDir, logger); err != nil && !errors.Is(migrate.ErrNoChange, err) {
		log.Fatalf("migration failed: %v", err)
	}

	// Redis
	rdb := helpers.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	defer func() { _ = rdb.Close() }()

	// GCS (available for DI in services that need it)
	gcsClient, err := helpers.NewGCSClient(ctx, cfg.GCSCredentialsJSONPath)
	if err != nil {
		log.Fatalf("failed to init GCS client: %v", err)
	}
	defer func() { _ = gcsClient.Close() }()

	// JWT
	jwtManager := helpers.NewJWTManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.AccessTTL, cfg.RefreshTTL)

	// Provide infra singletons to container for registry auto-wiring
	container.SetConfig(cfg)
	container.SetLogger(logger)
	container.SetPGPool(pool)
	container.SetRedis(rdb)
	container.SetGCS(gcsClient)
	container.SetJWT(jwtManager)

	// Gin engine and global middleware
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestIDMiddleware())
	// CORS
	corsCfg := cors.Config{
		AllowOrigins:     cfg.CORSOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	r.Use(cors.New(corsCfg))
	if cfg.Env == "development" {
		r.Use(gin.Logger())
	}

	// Registry: auto-register modules using container
	reg := router.NewRegistry(r)
	router.InitModules(reg)
	reg.RegisterAll()

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		logger.Infof("server starting on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			logger.Fatalf("listen: %s\n", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server")

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Fatalf("server forced to shutdown: %v", err)
	}
	logger.Info("server exited properly")
}

func runMigrations(dsn string, migrationsDir string, logger *logrus.Logger) error {
	// Open sql DB via pgx stdlib
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", migrationsDir), "postgres", driver)
	if err != nil {
		return err
	}
	logger.Info("running migrations...")
	err = m.Up()
	if errors.Is(migrate.ErrNoChange, err) {
		logger.Info("no migrations to run")
		return nil
	}
	return err
}
