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
	"path/filepath"
	"syscall"
	"time"

	"cloud.google.com/go/storage"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/oksasatya/go-ddd-clean-architecture/config"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/container"
	pginfra "github.com/oksasatya/go-ddd-clean-architecture/internal/infrastructure/postgres"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/interface/middleware"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/router"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/validation"
)

func main() {
	_ = godotenv.Load() // load .env if present

	cfg := config.Load()
	logger := helpers.NewLogger(cfg.AppName, cfg.Env)
	gin.SetMode(cfg.GinMode)

	// Initialize custom validator (uses JSON field names, alias tags)
	validation.Init()

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
	var gcsClient *storage.Client
	if cfg.GCSCredentialsJSONPath != "" {
		gcsClient, err = helpers.NewGCSClient(ctx, cfg.GCSCredentialsJSONPath)
		if err != nil {
			log.Fatalf("failed to init GCS client: %v", err)
		}
		container.SetGCS(gcsClient)
		defer func() { _ = gcsClient.Close() }()
	} else {
		logger.Warn("GCS client not initialized (GCSCredentialsJSONPath is empty)")
	}

	// JWT
	jwtManager := helpers.NewJWTManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.AccessTTL, cfg.RefreshTTL)

	// RabbitMQ publisher for email queue
	var rabbitPub *helpers.RabbitPublisher
	if cfg.RabbitMQURL != "" {
		rabbitPub, err = helpers.NewRabbitPublisher(cfg.RabbitMQURL, cfg.RabbitMQEmailQueue)
		if err != nil {
			logger.WithError(err).Warn("failed to connect to RabbitMQ; email enqueue will be unavailable")
		} else {
			defer rabbitPub.Close()
		}
	}

	// Mailgun client (used by background worker; also exposed for any direct sends if needed)
	var mgClient *mailer.Mailgun
	if cfg.MailgunDomain != "" && cfg.MailgunAPIKey != "" && cfg.MailgunSender != "" {
		mgClient = mailer.NewMailgun(cfg.MailgunDomain, cfg.MailgunAPIKey, cfg.MailgunSender)
	} else {
		logger.Warn("Mailgun not fully configured; worker will fail to send emails")
	}

	// Elasticsearch client
	var esClient *elasticsearch.Client
	if len(cfg.ESAddrs()) > 0 {
		if c, esErr := helpers.NewESClient(cfg.ESAddrs(), cfg.ElasticsearchUser, cfg.ElasticsearchPass); esErr != nil {
			logger.WithError(esErr).Warn("failed to init Elasticsearch client")
		} else {
			esClient = c
		}
	}

	// Provide infra singletons to container for registry auto-wiring
	container.SetConfig(cfg)
	container.SetLogger(logger)
	container.SetPGPool(pool)
	container.SetRedis(rdb)
	container.SetGCS(gcsClient)
	container.SetJWT(jwtManager)
	container.SetRabbitPub(rabbitPub)
	container.SetMailgun(mgClient)
	container.SetES(esClient)

	// Gin engine and global middleware
	r := gin.New()
	r.Use(gin.Recovery())

	// Trusted proxies: for dev use none; in prod, whitelist Cloudflare ranges
	if cfg.Env == "production" {
		_ = r.SetTrustedProxies([]string{
			// Cloudflare IPv4
			"173.245.48.0/20",
			"103.21.244.0/22",
			"103.22.200.0/22",
			"103.31.4.0/22",
			"141.101.64.0/18",
			"108.162.192.0/18",
			"190.93.240.0/20",
			"188.114.96.0/20",
			"197.234.240.0/22",
			"198.41.128.0/17",
			"162.158.0.0/15",
			"104.16.0.0/13",
			"104.24.0.0/14",
			"172.64.0.0/13",
			"131.0.72.0/22",
			// Cloudflare IPv6
			"2400:cb00::/32",
			"2606:4700::/32",
			"2803:f800::/32",
			"2405:b500::/32",
			"2405:8100::/32",
			"2a06:98c0::/29",
			"2c0f:f248::/32",
		})
	} else {
		_ = r.SetTrustedProxies(nil)
	}

	// Request ID then Real IP extraction
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.RealIP())
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

	// Temporarily disable rate limiter
	// r.Use(middleware.RateLimit(
	// 	rdb,
	// 	300,
	// 	time.Minute,
	// 	middleware.KeyByIPAndPath(),
	// 	middleware.AllowPrivateIP(),
	// ))

	// Example routes to show client vs real IP
	r.GET("/ip", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"client_ip": c.ClientIP(),
			"real_ip":   c.GetString("real_ip"),
		})
	})
	r.GET("/api/ip", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"client_ip": c.ClientIP(),
			"real_ip":   c.GetString("real_ip"),
		})
	})

	r.GET("/api/check", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
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

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Fatalf("server forced to shutdown: %v", err)
	}
	logger.Info("server exited properly")
}

func runMigrations(dsn string, migrationsDir string, logger *logrus.Logger) error {
	// Resolve migrationsDir to an absolute path and verify it exists
	absDir, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("resolve migrations dir: %w", err)
	}
	if _, statErr := os.Stat(absDir); os.IsNotExist(statErr) {
		// Try relative to the executable directory (useful when running compiled binary)
		exePath, exeErr := os.Executable()
		if exeErr == nil {
			exeDir := filepath.Dir(exePath)
			alt := filepath.Join(exeDir, migrationsDir)
			if _, altErr := os.Stat(alt); altErr == nil {
				absDir = alt
			} else {
				logger.Errorf("migrations dir not found: %s (also tried %s)", absDir, alt)
				return fmt.Errorf("migrations dir not found: %s", absDir)
			}
		} else {
			logger.Errorf("migrations dir not found: %s", absDir)
			return fmt.Errorf("migrations dir not found: %s", absDir)
		}
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		return err
	}
	srcURL := fmt.Sprintf("file://%s", filepath.ToSlash(absDir))
	m, err := migrate.NewWithDatabaseInstance(srcURL, "postgres", driver)
	if err != nil {
		return err
	}
	logger.Infof("running migrations from %s", srcURL)
	err = m.Up()
	if errors.Is(migrate.ErrNoChange, err) {
		logger.Info("no migrations to run")
		return nil
	}
	return err
}
