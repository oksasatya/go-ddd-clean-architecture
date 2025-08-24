package container

import (
	"cloud.google.com/go/storage"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/oksasatya/go-ddd-clean-architecture/config"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer"
)

// app-level container to share constructed components across packages
// Router can auto-wire modules from these singletons.

var (
	cfg         *config.Config
	logger      *logrus.Logger
	pgPool      *pgxpool.Pool
	redisClient *redis.Client
	gcsClient   *storage.Client

	jwtManager *helpers.JWTManager

	mailgunClient *mailer.Mailgun
	rabbitPub     *helpers.RabbitPublisher
	esClient      *elasticsearch.Client
)

func SetConfig(c *config.Config)   { cfg = c }
func GetConfig() *config.Config    { return cfg }
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

func SetMailgun(m *mailer.Mailgun)            { mailgunClient = m }
func GetMailgun() *mailer.Mailgun             { return mailgunClient }
func SetRabbitPub(p *helpers.RabbitPublisher) { rabbitPub = p }
func GetRabbitPub() *helpers.RabbitPublisher  { return rabbitPub }
func SetES(c *elasticsearch.Client)           { esClient = c }
func GetES() *elasticsearch.Client            { return esClient }
