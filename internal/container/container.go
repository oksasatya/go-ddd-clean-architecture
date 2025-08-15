package container

import (
	"cloud.google.com/go/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/oksasatya/go-ddd-clean-architecture/configs"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

// Simple app-level container to share constructed components across packages
// Router can auto-wire modules from these singletons.

var (
	cfg         *configs.Config
	logger      *logrus.Logger
	pgPool      *pgxpool.Pool
	redisClient *redis.Client
	gcsClient   *storage.Client

	jwtManager *helpers.JWTManager
)

func SetConfig(c *configs.Config)  { cfg = c }
func GetConfig() *configs.Config   { return cfg }
func SetLogger(l *logrus.Logger)   { logger = l }
func GetLogger() *logrus.Logger    { return logger }
func SetPGPool(p *pgxpool.Pool)    { pgPool = p }
func GetPGPool() *pgxpool.Pool     { return pgPool }
func SetRedis(r *redis.Client)     { redisClient = r }
func GetRedis() *redis.Client      { return redisClient }
func SetGCS(s *storage.Client)     { gcsClient = s }
func GetGCS() *storage.Client      { return gcsClient }
func SetJWT(m *helpers.JWTManager) { jwtManager = m }
func GetJWT() *helpers.JWTManager {
	if jwtManager != nil {
		return jwtManager
	}
	return helpers.DefaultJWT()
}
