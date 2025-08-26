package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds application configuration loaded from environment variables
// Provide sane defaults for local development.
type Config struct {
	AppName string
	Env     string // development, staging, production
	Port    string
	GinMode string

	// Database
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	DBSSLMode     string
	DBMaxConns    int32
	DBMinConns    int32
	DBMaxConnLife time.Duration

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Google Cloud Storage
	GCSBucket              string
	GCSCredentialsJSONPath string // optional; if empty, Application Default Credentials are used

	// JWT
	JWTAccessSecret  string
	JWTRefreshSecret string
	AccessTTL        time.Duration
	RefreshTTL       time.Duration

	// Cookies
	CookieDomain string
	CookieSecure bool

	// CORS
	CORSAllowedOrigins string // comma-separated

	// Migrations
	MigrationsDir string

	// Mailgun
	MailgunDomain string
	MailgunAPIKey string
	MailgunSender string

	// RabbitMQ
	RabbitMQURL        string
	RabbitMQEmailQueue string

	// Elasticsearch
	ElasticsearchAddrs string // comma-separated
	ElasticsearchUser  string
	ElasticsearchPass  string
	ESUsersIndex       string

	// Company/Links for emails
	CompanyName      string
	CompanyAddress   string
	LogoURL          string
	SupportURL       string
	PrivacyURL       string
	UnsubscribeURL   string
	ResetPasswordURL string
	VerifyEmailURL   string

	// Email sending toggle
	MailSendEnabled bool

	// Debug metrics (/api/debug/vars and /debug/vars)
	DebugMetricsEnabled bool

	// HTTP access log toggle (Gin logger)
	HTTPLogEnabled bool
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getbool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			log.Printf("invalid boolean for %s: %v, using default %v", key, err, def)
			return def
		}
		return b
	}
	return def
}

func getint(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("invalid int for %s: %v, using default %d", key, err, def)
			return def
		}
		return i
	}
	return def
}

func getdur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Printf("invalid duration for %s: %v, using default %v", key, err, def)
			return def
		}
		return d
	}
	return def
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		AppName: getenv("APP_NAME", "go-ddd-boilerplate"),
		Env:     getenv("APP_ENV", "development"),
		Port:    getenv("PORT", "8080"),
		GinMode: getenv("GIN_MODE", "release"),

		DBHost:        getenv("DB_HOST", "localhost"),
		DBPort:        getenv("DB_PORT", "5432"),
		DBUser:        getenv("DB_USER", "postgres"),
		DBPassword:    getenv("DB_PASSWORD", "postgres"),
		DBName:        getenv("DB_NAME", "appdb"),
		DBSSLMode:     getenv("DB_SSLMODE", "disable"),
		DBMaxConns:    int32(getint("DB_MAX_CONNS", 10)),
		DBMinConns:    int32(getint("DB_MIN_CONNS", 2)),
		DBMaxConnLife: getdur("DB_MAX_CONN_LIFETIME", time.Hour),

		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getenv("REDIS_PASSWORD", ""),
		RedisDB:       getint("REDIS_DB", 0),

		GCSBucket:              getenv("GCS_BUCKET", ""),
		GCSCredentialsJSONPath: getenv("GCS_CREDENTIALS_JSON", ""),

		JWTAccessSecret:  getenv("JWT_ACCESS_SECRET", "devaccesssecret"),
		JWTRefreshSecret: getenv("JWT_REFRESH_SECRET", "devrefreshsecret"),
		AccessTTL:        getdur("JWT_ACCESS_TTL", time.Hour),
		RefreshTTL:       getdur("JWT_REFRESH_TTL", 168*time.Hour),

		CookieDomain: getenv("COOKIE_DOMAIN", "localhost"),
		CookieSecure: getbool("COOKIE_SECURE", false),

		CORSAllowedOrigins: getenv("CORS_ALLOWED_ORIGINS", ""),

		MigrationsDir: getenv("MIGRATIONS_DIR", "db/migrations"),

		MailgunDomain: getenv("MAILGUN_DOMAIN", ""),
		MailgunAPIKey: getenv("MAILGUN_API_KEY", ""),
		MailgunSender: getenv("MAILGUN_SENDER", ""),

		RabbitMQURL:        getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQEmailQueue: getenv("RABBITMQ_EMAIL_QUEUE", "emails"),

		ElasticsearchAddrs: getenv("ELASTICSEARCH_ADDRS", "http://localhost:9200"),
		ElasticsearchUser:  getenv("ELASTICSEARCH_USERNAME", ""),
		ElasticsearchPass:  getenv("ELASTICSEARCH_PASSWORD", ""),
		ESUsersIndex:       getenv("ES_USERS_INDEX", "users"),

		CompanyName:      getenv("COMPANY_NAME", ""),
		CompanyAddress:   getenv("COMPANY_ADDRESS", ""),
		LogoURL:          getenv("LOGO_URL", ""),
		SupportURL:       getenv("SUPPORT_URL", ""),
		PrivacyURL:       getenv("PRIVACY_URL", ""),
		UnsubscribeURL:   getenv("UNSUBSCRIBE_URL", ""),
		ResetPasswordURL: getenv("RESET_PASSWORD_URL", "http://localhost:8080/reset-password"),
		VerifyEmailURL:   getenv("VERIFY_EMAIL_URL", "http://localhost:8080/verify-email"),

		// Email sending toggle (default true for backward compatibility)
		MailSendEnabled: getbool("MAIL_SEND_ENABLED", true),

		// Debug metrics toggle (default true to preserve existing behavior)
		DebugMetricsEnabled: getbool("DEBUG_METRICS_ENABLED", true),

		// HTTP access log toggle (default false; enable when needed)
		HTTPLogEnabled: getbool("HTTP_LOG_ENABLED", false),
	}
}

// PostgresDSN returns a DSN compatible with pgx
func (c *Config) PostgresDSN() string {
	// Example: postgres://user:password@host:port/dbname?sslmode=disable
	pwd := c.DBPassword
	return "postgres://" + c.DBUser + ":" + pwd + "@" + c.DBHost + ":" + c.DBPort + "/" + c.DBName + "?sslmode=" + c.DBSSLMode
}

// CORSOrigins returns the allowed origins as slice
func (c *Config) CORSOrigins() []string {
	parts := strings.Split(c.CORSAllowedOrigins, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			res = append(res, p)
		}
	}
	return res
}

// ESAddrs returns Elasticsearch addresses as a slice
func (c *Config) ESAddrs() []string {
	parts := strings.Split(c.ElasticsearchAddrs, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			res = append(res, p)
		}
	}
	return res
}
