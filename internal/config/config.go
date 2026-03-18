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
	"fmt"
	"strings"

	goutils "github.com/jkaninda/go-utils"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	errorhandlers "github.com/jkaninda/posta/internal/error_handlers"
	"github.com/jkaninda/posta/internal/storage"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Config struct {
	Database        DatabaseConfig
	Redis           RedisConfig
	JWTSecret       string
	Env             string
	Port            int
	DevMode         bool
	RateLimitHourly int
	RateLimitDaily  int
	AdminEmail      string
	AdminPassword   string
	OpenAPIDocs     bool
	MetricsEnabled  bool
	WebDir          string
	AppWebURL       string
	CORSOrigins     string

	// Worker settings
	EmbeddedWorker    bool
	WorkerConcurrency int
	WorkerMaxRetries  int

	// Webhook settings
	WebhookMaxRetries  int
	WebhookTimeoutSecs int
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
	Password string
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
		Redis: RedisConfig{
			Addr:     goutils.Env("POSTA_REDIS_ADDR", "localhost:6379"),
			Password: goutils.Env("POSTA_REDIS_PASSWORD", ""),
		},
		Port:            goutils.EnvInt("POSTA_PORT", 9000),
		Env:             goutils.Env("POSTA_ENV", "dev"),
		JWTSecret:       goutils.Env("POSTA_JWT_SECRET", "change-me-in-production"),
		DevMode:         goutils.EnvBool("POSTA_DEV_MODE", false),
		RateLimitHourly: goutils.EnvInt("POSTA_RATE_LIMIT_HOURLY", 100),
		RateLimitDaily:  goutils.EnvInt("POSTA_RATE_LIMIT_DAILY", 1000),
		AdminEmail:      goutils.Env("POSTA_ADMIN_EMAIL", "admin@example.com"),
		AdminPassword:   goutils.Env("POSTA_ADMIN_PASSWORD", "admin1234"),
		OpenAPIDocs:     goutils.EnvBool("POSTA_OPENAPI_DOCS", true),
		MetricsEnabled:  goutils.EnvBool("POSTA_METRICS_ENABLED", false),
		WebDir:          goutils.Env("POSTA_WEB_DIR", "web/dist"),
		AppWebURL:       goutils.Env("POSTA_WEB_URL", ""),

		CORSOrigins: goutils.Env("POSTA_CORS_ORIGINS", "*"),

		EmbeddedWorker:    goutils.EnvBool("POSTA_EMBEDDED_WORKER", false),
		WorkerConcurrency: goutils.EnvInt("POSTA_WORKER_CONCURRENCY", 10),
		WorkerMaxRetries:  goutils.EnvInt("POSTA_WORKER_MAX_RETRIES", 5),

		WebhookMaxRetries:  goutils.EnvInt("POSTA_WEBHOOK_MAX_RETRIES", 3),
		WebhookTimeoutSecs: goutils.EnvInt("POSTA_WEBHOOK_TIMEOUT_SECS", 10),
	}
}
func (c *Config) validate() error {

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
	app.WithCORS(okapi.Cors{
		AllowedOrigins:   corsOrigins,
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	})
	if c.OpenAPIDocs {
		app.WithOpenAPIDocs(okapi.OpenAPI{
			Title:   "Posta API",
			Version: "v1",
			License: okapi.License{
				Name: "MIT",
			},
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

	redisClient, err := storage.NewRedis(c.Redis.Addr, c.Redis.Password)
	if err != nil {
		logger.Fatal("failed to connect to redis", "error", err)
	}
	c.Redis.Client = redisClient

}
