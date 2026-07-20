/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package config

import (
	"crypto/tls"
	"fmt"
	"strings"

	errorhandlers "github.com/goposta/posta/internal/error_handlers"
	"github.com/goposta/posta/internal/storage"
	"github.com/hibiken/asynq"
	goutils "github.com/jkaninda/go-utils"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Config struct {
	Database             DatabaseConfig
	Redis                RedisConfig
	JWTSecret            string
	Env                  string
	Port                 int
	DevMode              bool
	RateLimitHourly      int
	RateLimitDaily       int
	AuthRateLimitEnabled bool

	// Email verification (POST /api/v1/emails/verify). Results are cached in
	// Redis to avoid re-checking the same address/domain on every call.
	EmailVerifyEnabled         bool
	EmailVerifyCacheTTLHours   int
	EmailVerifyMXCacheTTLHours int
	EmailVerifyRateHourly      int // per-user hourly cap; 0 disables
	AdminEmail                 string
	AdminPassword              string
	OpenAPIDocs                bool
	// AllowDowngrade lets the server boot even when the binary's version is
	// older than the version recorded in the database. Off by default.
	AllowDowngrade  bool
	securitySchemes okapi.SecuritySchemes

	MetricsEnabled bool

	PlanEnforcement   bool
	WorkspaceOnlyMode bool

	// WebDir overrides where the dashboard is served from. The UI is normally
	// embedded in the binary (internal/web); setting POSTA_WEB_DIR serves it from
	// this directory instead, for frontend development or a customized build.
	WebDir      string
	AppWebURL   string
	ApiBaseURL  string
	CORSOrigins string

	// Worker settings
	EmbeddedWorker    bool
	WorkerConcurrency int
	WorkerMaxRetries  int

	// AutoSuppressOnReject adds a recipient to the suppression list when an
	// outbound send is permanently rejected (5xx at RCPT TO, e.g. 550 user
	// unknown), and stops retrying that message.
	AutoSuppressOnReject bool

	// Webhook settings
	WebhookMaxRetries  int
	WebhookTimeoutSecs int
	WebhookProxyURL    string

	// OAuth settings
	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
	OAuthCallbackBaseURL    string

	// System SMTP for platform notifications (daily reports, invitations, etc.)
	SystemSMTP SystemSMTPConfig

	EmailVerificationRequired bool

	// Encryption key for SMTP password encryption (if empty, base64 encoding is used)
	EncryptionKey string

	// Blob storage settings (S3-compatible or filesystem)
	BlobProvider    string
	BlobS3Endpoint  string
	BlobS3Region    string
	BlobS3Bucket    string
	BlobS3AccessKey string
	BlobS3SecretKey string
	BlobS3UseSSL    bool
	BlobS3PathStyle bool
	BlobFSPath      string

	// Inbound email settings
	InboundEnabled        bool
	InboundSMTPHost       string
	InboundSMTPPort       int
	InboundMaxMessageSize int64
	InboundMaxAttachSize  int64
	InboundWebhookSecret  string
	InboundHostname       string
	InboundTLSMode        string
	InboundTLSCertFile    string
	InboundTLSKeyFile     string
	InboundSMTPRateLimit  int // per-IP max sessions per window; 0 disables
	InboundSMTPRateWindow int // rate-limit window in seconds
}
type SystemSMTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	Encryption string // none, ssl, starttls
}

func (s SystemSMTPConfig) IsConfigured() bool {
	return s.Host != "" && s.From != ""
}

type DatabaseConfig struct {
	DB       *gorm.DB
	host     string
	user     string
	password string
	name     string
	port     int
	sslMode  string
	url      string
}
type RedisConfig struct {
	Client   *redis.Client
	Addr     string
	Username string
	Password string
	DB       int
	// URL, when set, overrides the discrete fields above (e.g. redis://user:pass@host:6379/2).
	URL string
	// set for rediss:// URLs
	TLSConfig *tls.Config
}

// newRedisConfig reads Redis settings from the env; POSTA_REDIS_URL, if set,
// is parsed and overrides the discrete POSTA_REDIS_* vars (mirrors POSTA_DB_URL).
func newRedisConfig() RedisConfig {
	rc := RedisConfig{
		Addr:     goutils.Env("POSTA_REDIS_ADDR", "localhost:6379"),
		Username: goutils.Env("POSTA_REDIS_USERNAME", ""),
		Password: goutils.Env("POSTA_REDIS_PASSWORD", ""),
		DB:       goutils.EnvInt("POSTA_REDIS_DB", 0),
		URL:      goutils.Env("POSTA_REDIS_URL", ""),
	}
	if rc.URL != "" {
		opt, err := redis.ParseURL(rc.URL)
		if err != nil {
			logger.Fatal("invalid POSTA_REDIS_URL", "error", err)
		}
		rc.Addr = opt.Addr
		rc.Username = opt.Username
		rc.Password = opt.Password
		rc.DB = opt.DB
		rc.TLSConfig = opt.TLSConfig
	}
	return rc
}

// RedisOptions returns the go-redis client options.
func (r RedisConfig) RedisOptions() *redis.Options {
	return &redis.Options{
		Addr:      r.Addr,
		Username:  r.Username,
		Password:  r.Password,
		DB:        r.DB,
		TLSConfig: r.TLSConfig,
	}
}

// AsynqRedisOpt returns the Asynq Redis connection options.
func (r RedisConfig) AsynqRedisOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:      r.Addr,
		Username:  r.Username,
		Password:  r.Password,
		DB:        r.DB,
		TLSConfig: r.TLSConfig,
	}
}

type JWTConfig struct {
	Secret   string
	Issuer   string
	Audience string
}

type LogConfig struct {
	Level string
}

func New() *Config {
	if err := godotenv.Load(); err != nil {
		logger.Debug("no .env file found, using environment variables")
	}
	return &Config{
		Database: DatabaseConfig{
			host:     goutils.Env("POSTA_DB_HOST", "localhost"),
			user:     goutils.Env("POSTA_DB_USER", "posta"),
			password: goutils.Env("POSTA_DB_PASSWORD", "posta"),
			name:     goutils.Env("POSTA_DB_NAME", "posta"),
			port:     goutils.EnvInt("POSTA_DB_PORT", 5432),
			sslMode:  goutils.Env("POSTA_DB_SSL_MODE", "disable"),
			url:      goutils.Env("POSTA_DB_URL", ""),
		},
		Redis:                newRedisConfig(),
		Port:                 goutils.EnvInt("POSTA_PORT", 9000),
		Env:                  goutils.Env("POSTA_ENV", "dev"),
		JWTSecret:            goutils.Env("POSTA_JWT_SECRET", "change-me-in-production"),
		DevMode:              goutils.EnvBool("POSTA_DEV_MODE", false),
		RateLimitHourly:      goutils.EnvInt("POSTA_RATE_LIMIT_HOURLY", 100),
		RateLimitDaily:       goutils.EnvInt("POSTA_RATE_LIMIT_DAILY", 1000),
		AuthRateLimitEnabled: goutils.EnvBool("POSTA_AUTH_RATE_LIMIT_ENABLED", true),

		EmailVerifyEnabled:         goutils.EnvBool("POSTA_EMAIL_VERIFY_ENABLED", true),
		EmailVerifyCacheTTLHours:   goutils.EnvInt("POSTA_EMAIL_VERIFY_CACHE_TTL_HOURS", 168),
		EmailVerifyMXCacheTTLHours: goutils.EnvInt("POSTA_EMAIL_VERIFY_MX_CACHE_TTL_HOURS", 24),
		EmailVerifyRateHourly:      goutils.EnvInt("POSTA_EMAIL_VERIFY_RATE_HOURLY", 1000),
		AdminEmail:                 goutils.Env("POSTA_ADMIN_EMAIL", "admin@example.com"),
		AdminPassword:              goutils.Env("POSTA_ADMIN_PASSWORD", "admin1234"),
		OpenAPIDocs:                goutils.EnvBool("POSTA_OPENAPI_DOCS", true),
		AllowDowngrade:             goutils.EnvBool("POSTA_ALLOW_DOWNGRADE", false),
		securitySchemes:            okapi.SecuritySchemes{},

		MetricsEnabled:    goutils.EnvBool("POSTA_METRICS_ENABLED", false),
		PlanEnforcement:   goutils.EnvBool("POSTA_PLAN_ENFORCEMENT", false),
		WorkspaceOnlyMode: goutils.EnvBool("POSTA_WORKSPACE_ONLY_MODE", false),
		WebDir:            goutils.Env("POSTA_WEB_DIR", ""),
		AppWebURL:         goutils.Env("POSTA_WEB_URL", ""),
		ApiBaseURL:        goutils.Env("POSTA_API_URL", ""),

		CORSOrigins: goutils.Env("POSTA_CORS_ORIGINS", "*"),

		EmbeddedWorker:       goutils.EnvBool("POSTA_EMBEDDED_WORKER", false),
		WorkerConcurrency:    goutils.EnvInt("POSTA_WORKER_CONCURRENCY", 10),
		WorkerMaxRetries:     goutils.EnvInt("POSTA_WORKER_MAX_RETRIES", 5),
		AutoSuppressOnReject: goutils.EnvBool("POSTA_AUTO_SUPPRESS_ON_REJECT", true),

		WebhookMaxRetries:  goutils.EnvInt("POSTA_WEBHOOK_MAX_RETRIES", 3),
		WebhookTimeoutSecs: goutils.EnvInt("POSTA_WEBHOOK_TIMEOUT_SECS", 10),
		WebhookProxyURL:    goutils.Env("POSTA_WEBHOOK_PROXY_URL", ""),

		GoogleOAuthClientID:     goutils.Env("POSTA_GOOGLE_OAUTH_CLIENT_ID", ""),
		GoogleOAuthClientSecret: goutils.Env("POSTA_GOOGLE_OAUTH_CLIENT_SECRET", ""),
		OAuthCallbackBaseURL:    goutils.Env("POSTA_OAUTH_CALLBACK_URL", ""),

		SystemSMTP: SystemSMTPConfig{
			Host:       goutils.Env("POSTA_SYSTEM_SMTP_HOST", ""),
			Port:       goutils.EnvInt("POSTA_SYSTEM_SMTP_PORT", 587),
			Username:   goutils.Env("POSTA_SYSTEM_SMTP_USERNAME", ""),
			Password:   goutils.Env("POSTA_SYSTEM_SMTP_PASSWORD", ""),
			From:       goutils.Env("POSTA_SYSTEM_SMTP_FROM", ""),
			Encryption: goutils.Env("POSTA_SYSTEM_SMTP_ENCRYPTION", "starttls"),
		},

		EmailVerificationRequired: goutils.EnvBool("POSTA_EMAIL_VERIFICATION_REQUIRED", false),

		EncryptionKey: goutils.Env("POSTA_ENCRYPTION_KEY", ""),

		BlobProvider:    goutils.Env("POSTA_BLOB_PROVIDER", ""),
		BlobS3Endpoint:  goutils.Env("POSTA_BLOB_S3_ENDPOINT", ""),
		BlobS3Region:    goutils.Env("POSTA_BLOB_S3_REGION", "us-east-1"),
		BlobS3Bucket:    goutils.Env("POSTA_BLOB_S3_BUCKET", ""),
		BlobS3AccessKey: goutils.Env("POSTA_BLOB_S3_ACCESS_KEY", ""),
		BlobS3SecretKey: goutils.Env("POSTA_BLOB_S3_SECRET_KEY", ""),
		BlobS3UseSSL:    goutils.EnvBool("POSTA_BLOB_S3_USE_SSL", true),
		BlobS3PathStyle: goutils.EnvBool("POSTA_BLOB_S3_PATH_STYLE", false),
		BlobFSPath:      goutils.Env("POSTA_BLOB_FS_PATH", "data/attachments"),

		InboundEnabled:        goutils.EnvBool("POSTA_INBOUND_ENABLED", false),
		InboundSMTPHost:       goutils.Env("POSTA_INBOUND_SMTP_HOST", "0.0.0.0"),
		InboundSMTPPort:       goutils.EnvInt("POSTA_INBOUND_SMTP_PORT", 2525),
		InboundMaxMessageSize: int64(goutils.EnvInt("POSTA_INBOUND_MAX_MESSAGE_SIZE", 26214400)),
		InboundMaxAttachSize:  int64(goutils.EnvInt("POSTA_INBOUND_MAX_ATTACH_SIZE", 10485760)),
		InboundWebhookSecret:  goutils.Env("POSTA_INBOUND_WEBHOOK_SECRET", ""),
		InboundHostname:       goutils.Env("POSTA_INBOUND_HOSTNAME", "posta.local"),
		InboundTLSMode:        goutils.Env("POSTA_INBOUND_TLS_MODE", "none"),
		InboundTLSCertFile:    goutils.Env("POSTA_INBOUND_TLS_CERT_FILE", ""),
		InboundTLSKeyFile:     goutils.Env("POSTA_INBOUND_TLS_KEY_FILE", ""),
		InboundSMTPRateLimit:  goutils.EnvInt("POSTA_INBOUND_SMTP_RATE_LIMIT", 60),
		InboundSMTPRateWindow: goutils.EnvInt("POSTA_INBOUND_SMTP_RATE_WINDOW", 60),
	}
}
func (c *Config) validate() error {
	if c.InboundEnabled && c.InboundTLSMode != "" && c.InboundTLSMode != "none" {
		if c.InboundTLSMode != "starttls" {
			return fmt.Errorf("unsupported POSTA_INBOUND_TLS_MODE %q (use none or starttls)", c.InboundTLSMode)
		}
		if c.InboundTLSCertFile == "" || c.InboundTLSKeyFile == "" {
			return fmt.Errorf("POSTA_INBOUND_TLS_MODE=%s requires POSTA_INBOUND_TLS_CERT_FILE and POSTA_INBOUND_TLS_KEY_FILE", c.InboundTLSMode)
		}
	}
	return nil
}
func (c *Config) validateWorker() error {

	return nil
}
func (c *Config) Initialize(app *okapi.Okapi) error {
	if err := c.validate(); err != nil {
		return err
	}
	// Initialize global logger
	l := c.initLogger()
	// Dev mode
	if c.DevMode {
		app.WithDebug()
	}
	// Set Port
	app.WithPort(c.Port)
	app.WithLogger(l.Logger)
	_ = goutils.SetEnv("ENV", c.Env)
	corsOrigins := strings.Split(c.CORSOrigins, ",")
	for i := range corsOrigins {
		corsOrigins[i] = strings.TrimSpace(corsOrigins[i])
	}
	apiServers := okapi.Servers{}
	if c.AppWebURL != "" {
		apiServers = append(apiServers, okapi.Server{URL: c.AppWebURL})
	}
	if c.ApiBaseURL != "" {
		apiServers = append(apiServers, okapi.Server{URL: c.ApiBaseURL})
	}
	app.WithCORS(okapi.Cors{
		AllowedOrigins:   corsOrigins,
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID", "X-Posta-Workspace-Id"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	})

	if c.OpenAPIDocs {
		// Dashboard / JWT-authenticated endpoints.
		c.securitySchemes = append(c.securitySchemes, okapi.SecurityScheme{
			Name:         "BearerAuth",
			Description:  "Bearer token issued by /auth/login. Send as: `Authorization: Bearer <JWT>`.",
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		})
		// API-key authenticated endpoints.
		c.securitySchemes = append(c.securitySchemes, okapi.SecurityScheme{
			Name:         "ApiKeyAuth",
			Description:  "Long-lived API key. Send as: `Authorization: Bearer <API_KEY>`. Manage keys under Settings → API Keys.",
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "Posta API Key",
		})
		app.WithOpenAPIDocs(okapi.OpenAPI{
			Title:       "Posta API",
			Version:     Version,
			Description: "Self-hosted email delivery platform for developers and teams.",
			Favicon:     "/favicon.png",
			License: okapi.License{
				Name: "Apache-2.0",
				URL:  "http://www.apache.org/licenses/LICENSE-2.0",
			},
			Contact: okapi.Contact{
				Name:  "Support",
				URL:   "https://goposta.dev/",
				Email: "jonas@goposta.dev",
			},
			Servers:         apiServers,
			SecuritySchemes: c.securitySchemes,
			UI:              okapi.ScalarUI,
			StrictDocUI:     true,
		})
	}
	app.WithErrorHandler(errorhandlers.CustomErrorHandler())
	return nil
}
func (c *Config) InitWorker() error {
	// Initialize global logger
	c.initLogger()
	if err := c.validateWorker(); err != nil {
		return err
	}
	return nil
}
func (c *Config) initLogger() *logger.Logger {
	if c.DevMode {
		return logger.New(logger.WithDebugLevel())
	}
	return logger.New(logger.WithJSONFormat(), logger.WithInfoLevel())
}

func (c *Config) InitStorage() {
	var dsn string
	if c.Database.url != "" {
		dsn = c.Database.url
	} else {
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s", c.Database.host, c.Database.user, c.Database.password, c.Database.name, c.Database.port, c.Database.sslMode)
	}
	dbConn, err := storage.ConnectPostgres(dsn)
	if err != nil {
		logger.Fatal("failed to connect to database", "error", err)
	}
	c.Database.DB = dbConn

	redisClient, err := storage.NewRedis(c.Redis.RedisOptions())
	if err != nil {
		logger.Fatal("failed to connect to redis", "error", err)
	}
	c.Redis.Client = redisClient

}
