// Copyright (c) 2026 Jonas Kaninda.
//
// This file is part of Posta.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"

	errorhandlers "github.com/goposta/posta/internal/error_handlers"
	"github.com/goposta/posta/internal/netguard"
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

	// Email verification results are cached in Redis per address/domain to avoid re-checking.
	EmailVerifyEnabled         bool
	EmailVerifyCacheTTLHours   int
	EmailVerifyMXCacheTTLHours int
	EmailVerifyRateHourly      int // per-user hourly cap; 0 disables
	AdminEmail                 string
	AdminPassword              string
	OpenAPIDocs                bool
	// AllowDowngrade lets the server boot on a binary older than the version recorded in the database.
	AllowDowngrade  bool
	securitySchemes okapi.SecuritySchemes

	MetricsEnabled bool

	// UpdateCheck only reads GitHub's public release list; it never upgrades and sends nothing about this install.
	UpdateCheck bool

	PlanEnforcement bool

	// WebDir (POSTA_WEB_DIR) serves the dashboard from disk instead of the embedded UI.
	WebDir     string
	AppWebURL  string
	ApiBaseURL string
	// CORSOrigins left empty derives the allowed origins from AppWebURL and ApiBaseURL; see AllowedCORSOrigins.
	CORSOrigins string

	// Forwarded IP headers are honoured only from TrustedProxies; empty trusts any peer, letting clients spoof their IP.
	TrustedProxies []string

	EmbeddedWorker    bool
	WorkerConcurrency int
	WorkerMaxRetries  int
	// The probe listener defaults to POSTA_PORT: server and worker run in separate containers, so nothing collides.
	WorkerHealthEnabled bool
	WorkerHealthPort    int

	// AutoSuppressOnReject suppresses the recipient and stops retrying on a permanent 5xx RCPT TO rejection.
	AutoSuppressOnReject bool

	WebhookMaxRetries  int
	WebhookTimeoutSecs int
	WebhookProxyURL    string
	// WebhookNetwork blocks private and reserved addresses for webhook URLs by default.
	WebhookNetwork netguard.Policy
	// SMTPNetwork allows private addresses by default, since a relay on the same host is common.
	SMTPNetwork netguard.Policy

	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
	OAuthCallbackBaseURL    string

	SystemSMTP SystemSMTPConfig

	EmailVerificationRequired bool

	// Encryption key for SMTP password encryption (if empty, base64 encoding is used)
	EncryptionKey string

	// License is installed at boot when none is stored: the token itself, or a file holding it.
	License     string
	LicenseFile string

	BlobProvider    string
	BlobS3Endpoint  string
	BlobS3Region    string
	BlobS3Bucket    string
	BlobS3AccessKey string
	BlobS3SecretKey string
	BlobS3UseSSL    bool
	BlobS3PathStyle bool
	BlobFSPath      string

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

	MessagesEnabled           bool
	MessagesMaxBodyBytes      int64
	MessagesMaxAttachSize     int64
	MessagesIPRateLimit       int
	MessagesIPRateWindow      int
	MessagesPerFormHourly     int
	MessagesPerEmailHourly    int
	MessagesPerWorkspaceDaily int
	MessagesInboundDomain     string

	ChatEnabled         bool
	ChatMaxStreams      int
	ChatMaxMessageBytes int
	ChatIPRateLimit     int
	// ChatUnansweredMinutes 0 disables the alert to the assignee or admins about an unanswered visitor.
	ChatUnansweredMinutes int
	// ChatRetentionDays keeps closed and spam conversations; a widget can only lower it, and 0 keeps them forever.
	ChatRetentionDays int

	SMTPRelayEnabled        bool
	SMTPRelayHost           string
	SMTPRelayPort           int
	SMTPRelayHostname       string
	SMTPRelayTLSMode        string
	SMTPRelayTLSCertFile    string
	SMTPRelayTLSKeyFile     string
	SMTPRelayMaxMessageSize int64
	SMTPRelayRateLimit      int
	SMTPRelayRateWindow     int
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

// POSTA_REDIS_URL, if set, overrides the discrete POSTA_REDIS_* vars (mirrors POSTA_DB_URL).
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

func (r RedisConfig) RedisOptions() *redis.Options {
	return &redis.Options{
		Addr:      r.Addr,
		Username:  r.Username,
		Password:  r.Password,
		DB:        r.DB,
		TLSConfig: r.TLSConfig,
	}
}

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
	// Resolved first so the worker's probe listener can default to it.
	port := goutils.EnvInt("POSTA_PORT", 9000)

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
		Port:                 port,
		Env:                  goutils.Env("POSTA_ENV", "production"),
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

		MetricsEnabled:  goutils.EnvBool("POSTA_METRICS_ENABLED", false),
		UpdateCheck:     goutils.EnvBool("POSTA_UPDATE_CHECK", true),
		PlanEnforcement: goutils.EnvBool("POSTA_PLAN_ENFORCEMENT", false),
		WebDir:          goutils.Env("POSTA_WEB_DIR", ""),
		AppWebURL:       goutils.Env("POSTA_WEB_URL", ""),
		ApiBaseURL:      goutils.Env("POSTA_API_URL", ""),

		CORSOrigins:    goutils.Env("POSTA_CORS_ORIGINS", ""),
		TrustedProxies: splitList(goutils.Env("POSTA_TRUSTED_PROXIES", "")),

		EmbeddedWorker:       goutils.EnvBool("POSTA_EMBEDDED_WORKER", false),
		WorkerConcurrency:    goutils.EnvInt("POSTA_WORKER_CONCURRENCY", 10),
		WorkerMaxRetries:     goutils.EnvInt("POSTA_WORKER_MAX_RETRIES", 5),
		WorkerHealthEnabled:  goutils.EnvBool("POSTA_WORKER_HEALTH_ENABLED", true),
		WorkerHealthPort:     goutils.EnvInt("POSTA_WORKER_HEALTH_PORT", port),
		AutoSuppressOnReject: goutils.EnvBool("POSTA_AUTO_SUPPRESS_ON_REJECT", true),

		WebhookMaxRetries:  goutils.EnvInt("POSTA_WEBHOOK_MAX_RETRIES", 3),
		WebhookTimeoutSecs: goutils.EnvInt("POSTA_WEBHOOK_TIMEOUT_SECS", 10),
		WebhookProxyURL:    goutils.Env("POSTA_WEBHOOK_PROXY_URL", ""),
		WebhookNetwork:     networkPolicy("POSTA_WEBHOOK", false),
		SMTPNetwork:        networkPolicy("POSTA_SMTP", true),

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
		License:       goutils.Env("POSTA_LICENSE", ""),
		LicenseFile:   goutils.Env("POSTA_LICENSE_FILE", ""),

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

		MessagesEnabled:           goutils.EnvBool("POSTA_MESSAGES_ENABLED", false),
		MessagesMaxBodyBytes:      int64(goutils.EnvInt("POSTA_MESSAGES_MAX_BODY_BYTES", 65536)),
		MessagesMaxAttachSize:     int64(goutils.EnvInt("POSTA_MESSAGES_MAX_ATTACH_SIZE", 5242880)),
		MessagesIPRateLimit:       goutils.EnvInt("POSTA_MESSAGES_IP_RATE_LIMIT", 20),
		MessagesIPRateWindow:      goutils.EnvInt("POSTA_MESSAGES_IP_RATE_WINDOW", 3600),
		MessagesPerFormHourly:     goutils.EnvInt("POSTA_MESSAGES_PER_FORM_HOURLY", 200),
		MessagesPerEmailHourly:    goutils.EnvInt("POSTA_MESSAGES_PER_EMAIL_HOURLY", 5),
		MessagesPerWorkspaceDaily: goutils.EnvInt("POSTA_MESSAGES_PER_WORKSPACE_DAILY", 1000),
		MessagesInboundDomain:     goutils.Env("POSTA_MESSAGES_INBOUND_DOMAIN", ""),

		ChatEnabled:           goutils.EnvBool("POSTA_CHAT_ENABLED", false),
		ChatMaxStreams:        goutils.EnvInt("POSTA_CHAT_MAX_STREAMS", 5000),
		ChatMaxMessageBytes:   goutils.EnvInt("POSTA_CHAT_MAX_MESSAGE_BYTES", 4096),
		ChatIPRateLimit:       goutils.EnvInt("POSTA_CHAT_IP_RATE_LIMIT", 60),
		ChatUnansweredMinutes: goutils.EnvInt("POSTA_CHAT_UNANSWERED_MINUTES", 5),
		ChatRetentionDays:     goutils.EnvInt("POSTA_CHAT_RETENTION_DAYS", 365),

		SMTPRelayEnabled:        goutils.EnvBool("POSTA_SMTP_RELAY_ENABLED", false),
		SMTPRelayHost:           goutils.Env("POSTA_SMTP_RELAY_HOST", "0.0.0.0"),
		SMTPRelayPort:           goutils.EnvInt("POSTA_SMTP_RELAY_PORT", 2526),
		SMTPRelayHostname:       goutils.Env("POSTA_SMTP_RELAY_HOSTNAME", "posta.local"),
		SMTPRelayTLSMode:        goutils.Env("POSTA_SMTP_RELAY_TLS_MODE", "none"),
		SMTPRelayTLSCertFile:    goutils.Env("POSTA_SMTP_RELAY_TLS_CERT_FILE", ""),
		SMTPRelayTLSKeyFile:     goutils.Env("POSTA_SMTP_RELAY_TLS_KEY_FILE", ""),
		SMTPRelayMaxMessageSize: int64(goutils.EnvInt("POSTA_SMTP_RELAY_MAX_MESSAGE_SIZE", 26214400)),
		SMTPRelayRateLimit:      goutils.EnvInt("POSTA_SMTP_RELAY_RATE_LIMIT", 60),
		SMTPRelayRateWindow:     goutils.EnvInt("POSTA_SMTP_RELAY_RATE_WINDOW", 60),
	}
}

// removedEnvVars no longer do anything; setting one only warns so the operator can clean up.
var removedEnvVars = map[string]string{
	"POSTA_WORKSPACE_ONLY_MODE": "every resource is workspace-scoped unconditionally; the flag no longer does anything",
}

func warnRemovedEnvVars() {
	for name, why := range removedEnvVars {
		if _, ok := os.LookupEnv(name); ok {
			logger.Warn("ignoring removed setting", "variable", name, "reason", why)
		}
	}
}

func (c *Config) validate() error {
	warnRemovedEnvVars()
	if c.MessagesEnabled && c.MessagesInboundDomain != "" && !c.InboundEnabled {
		return fmt.Errorf("POSTA_MESSAGES_INBOUND_DOMAIN requires POSTA_INBOUND_ENABLED=true")
	}
	if c.InboundEnabled && c.InboundTLSMode != "" && c.InboundTLSMode != "none" {
		if c.InboundTLSMode != "starttls" {
			return fmt.Errorf("unsupported POSTA_INBOUND_TLS_MODE %q (use none or starttls)", c.InboundTLSMode)
		}
		if c.InboundTLSCertFile == "" || c.InboundTLSKeyFile == "" {
			return fmt.Errorf("POSTA_INBOUND_TLS_MODE=%s requires POSTA_INBOUND_TLS_CERT_FILE and POSTA_INBOUND_TLS_KEY_FILE", c.InboundTLSMode)
		}
	}
	if c.SMTPRelayEnabled {
		switch c.SMTPRelayTLSMode {
		case "", "none":
		case "starttls":
			if c.SMTPRelayTLSCertFile == "" || c.SMTPRelayTLSKeyFile == "" {
				return fmt.Errorf("POSTA_SMTP_RELAY_TLS_MODE=starttls requires POSTA_SMTP_RELAY_TLS_CERT_FILE and POSTA_SMTP_RELAY_TLS_KEY_FILE")
			}
		default:
			return fmt.Errorf("unsupported POSTA_SMTP_RELAY_TLS_MODE %q (use none or starttls)", c.SMTPRelayTLSMode)
		}
	}
	// okapi drops an invalid list and trusts no proxy, collapsing every client onto the proxy IP, so refuse to start.
	for _, entry := range c.TrustedProxies {
		if !validProxyEntry(entry) {
			return fmt.Errorf("invalid POSTA_TRUSTED_PROXIES entry %q (expected a CIDR block or IP address)", entry)
		}
	}
	return c.ValidateSecurity()
}
func (c *Config) validateWorker() error {
	warnRemovedEnvVars()
	return c.ValidateWorker()
}

// OpenAPISecuritySchemes are referenced by name from routes via WithSecurity.
func OpenAPISecuritySchemes() okapi.SecuritySchemes {
	return okapi.SecuritySchemes{
		{
			Name:         "BearerAuth",
			Description:  "Bearer token issued by /auth/login. Send as: `Authorization: Bearer <JWT>`.",
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		},
		{
			Name:         "ApiKeyAuth",
			Description:  "Long-lived API key. Send as: `Authorization: Bearer <API_KEY>`. Manage keys under Settings → API Keys.",
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "Posta API Key",
		},
	}
}

func (c *Config) Initialize(app *okapi.Okapi) error {
	if err := c.validate(); err != nil {
		return err
	}
	l := c.initLogger()
	c.WarnInsecureConfig()
	if c.DevMode {
		app.WithDebug()
	}
	app.WithPort(c.Port)
	app.WithLogger(l.Logger)
	app.WithTrustedProxies(c.TrustedProxies...)
	app.WithMaxRequestBody(maxRequestBody)
	_ = goutils.SetEnv("ENV", c.Env)
	apiServers := okapi.Servers{}
	if c.AppWebURL != "" {
		apiServers = append(apiServers, okapi.Server{URL: c.AppWebURL})
	}
	if c.ApiBaseURL != "" {
		apiServers = append(apiServers, okapi.Server{URL: c.ApiBaseURL})
	}
	// Must come before WithCORS: see ChatPreflight.
	app.Use(ChatPreflight)
	app.WithCORS(okapi.Cors{
		AllowedOrigins:   c.AllowedCORSOrigins(),
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID", "X-Posta-Workspace-Id"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	})

	if c.OpenAPIDocs {
		c.securitySchemes = append(c.securitySchemes, OpenAPISecuritySchemes()...)
		app.WithOpenAPIDocs(okapi.OpenAPI{
			Title:       "Posta API",
			Version:     Version,
			Description: "Self-hosted email delivery platform for developers and teams.",
			Favicon:     "/favicon.png",
			License: okapi.License{
				Name: "AGPL-3.0-or-later",
				URL:  "https://www.gnu.org/licenses/agpl-3.0.html",
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
	c.initLogger()
	c.WarnInsecureWorkerConfig()
	return c.validateWorker()
}

// PublicAPIURL is POSTA_API_URL when the API has its own host, else POSTA_WEB_URL, which serves the API too.
func (c *Config) PublicAPIURL() string {
	if c.ApiBaseURL != "" {
		return strings.TrimSuffix(c.ApiBaseURL, "/")
	}
	return strings.TrimSuffix(c.AppWebURL, "/")
}

// networkPolicy fails on an unparseable allowed network rather than run with a policy other than the configured one.
func networkPolicy(prefix string, allowPrivate bool) netguard.Policy {
	p, err := netguard.FromEnv(prefix, allowPrivate)
	if err != nil {
		logger.Fatal("invalid network setting", "error", err)
	}
	return p
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

// AllowedCORSOrigins falls back to the POSTA_WEB_URL/POSTA_API_URL origins; empty means same-origin only.
func (c *Config) AllowedCORSOrigins() []string {
	if origins := splitList(c.CORSOrigins); len(origins) > 0 {
		return origins
	}
	var origins []string
	for _, raw := range []string{c.AppWebURL, c.ApiBaseURL} {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		origin := u.Scheme + "://" + u.Host
		if !slices.Contains(origins, origin) {
			origins = append(origins, origin)
		}
	}
	return origins
}

// okapi's 8 MB default is too small: 25 MB of attachments arrive base64-encoded (~33 MB) plus the bodies.
const maxRequestBody = 40 << 20

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// validProxyEntry mirrors what okapi.WithTrustedProxies accepts.
func validProxyEntry(entry string) bool {
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return true
	}
	return net.ParseIP(entry) != nil
}
