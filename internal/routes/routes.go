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

package routes

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/config"
	cronpkg "github.com/jkaninda/posta/internal/cron"
	"github.com/jkaninda/posta/internal/handlers"
	"github.com/jkaninda/posta/internal/metrics"
	"github.com/jkaninda/posta/internal/middlewares"
	"github.com/jkaninda/posta/internal/services/audit"
	"github.com/jkaninda/posta/internal/services/auth"
	"github.com/jkaninda/posta/internal/services/cache"
	"github.com/jkaninda/posta/internal/services/email"
	"github.com/jkaninda/posta/internal/services/eventbus"
	"github.com/jkaninda/posta/internal/services/ratelimit"
	"github.com/jkaninda/posta/internal/services/seeder"
	sessionpkg "github.com/jkaninda/posta/internal/services/session"
	"github.com/jkaninda/posta/internal/services/settings"
	"github.com/jkaninda/posta/internal/services/webhook"
	"github.com/jkaninda/posta/internal/services/workermon"
	"github.com/jkaninda/posta/internal/storage/repositories"
	"github.com/jkaninda/posta/internal/worker"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Router struct {
	app *okapi.Okapi
	cfg *config.Config
	v1  *okapi.Group
	mw  routerMiddleware
	h   routerHandlers
}

type routerMiddleware struct {
	jwtAuth           okapi.JWTAuth
	jwtAdminAuth      okapi.JWTAuth
	jwtAdminQueryAuth okapi.JWTAuth
	loginLimiter      okapi.Middleware
	apiKey            okapi.Middleware
}

type routerHandlers struct {
	health          *handlers.HealthHandler
	user            *handlers.UserHandler
	email           *handlers.EmailHandler
	apiKey          *handlers.APIKeyHandler
	template        *handlers.TemplateHandler
	version         *handlers.TemplateVersionHandler
	localization    *handlers.TemplateLocalizationHandler
	language        *handlers.LanguageHandler
	stylesheet      *handlers.StyleSheetHandler
	smtp            *handlers.SMTPHandler
	webhook         *handlers.WebhookHandler
	webhookDelivery *handlers.WebhookDeliveryHandler
	dashboard       *handlers.DashboardHandler
	domain          *handlers.DomainHandler
	bounce          *handlers.BounceHandler
	suppression     *handlers.SuppressionHandler
	contact         *handlers.ContactHandler
	contactList     *handlers.ContactListHandler
	admin           *handlers.AdminHandler
	server          *handlers.ServerHandler
	event           *handlers.EventHandler
	analytics       *handlers.AnalyticsHandler
	setting         *handlers.SettingHandler
	userSetting     *handlers.UserSettingHandler
	userData        *handlers.UserDataHandler
	session         *handlers.SessionHandler
	cron            *handlers.CronHandler
}

func InitRoutes(app *okapi.Okapi, db *gorm.DB, redisClient *redis.Client, cfg *config.Config, producer *worker.Producer, cronManager *cronpkg.Manager, ctx context.Context) {
	// Repositories
	userRepo := repositories.NewUserRepository(db)
	apiKeyRepo := repositories.NewAPIKeyRepository(db)
	emailRepo := repositories.NewEmailRepository(db)
	templateRepo := repositories.NewTemplateRepository(db)
	smtpRepo := repositories.NewSMTPRepository(db)
	serverRepo := repositories.NewServerRepository(db)
	webhookRepo := repositories.NewWebhookRepository(db)
	domainRepo := repositories.NewDomainRepository(db)
	bounceRepo := repositories.NewBounceRepository(db)
	suppressionRepo := repositories.NewSuppressionRepository(db)
	stylesheetRepo := repositories.NewStyleSheetRepository(db)
	versionRepo := repositories.NewTemplateVersionRepository(db)
	localizationRepo := repositories.NewTemplateLocalizationRepository(db)
	languageRepo := repositories.NewLanguageRepository(db)
	contactRepo := repositories.NewContactRepository(db)
	contactListRepo := repositories.NewContactListRepository(db)
	eventRepo := repositories.NewEventRepository(db)
	settingRepo := repositories.NewSettingRepository(db)
	userSettingRepo := repositories.NewUserSettingRepository(db)
	sessionRepo := repositories.NewSessionRepository(db)

	// Session store (Redis-backed blacklist)
	sessionStore := sessionpkg.NewStore(redisClient)

	// Services
	bus := eventbus.New(eventRepo)
	auditLogger := audit.NewLogger(bus)
	apiKeyService := auth.NewAPIKeyService(apiKeyRepo)
	settingsProvider := settings.NewProvider(settingRepo)
	limiter := ratelimit.NewRedisLimiter(redisClient, cfg.RateLimitHourly, cfg.RateLimitDaily)
	limiter.SetSettings(settingsProvider)
	webhookDeliveryRepo := repositories.NewWebhookDeliveryRepository(db)
	dispatcher := webhook.NewDispatcher(webhookRepo)
	dispatcher.SetDeliveryRepo(webhookDeliveryRepo)
	dispatcher.SetConfig(webhook.Config{
		MaxRetries: cfg.WebhookMaxRetries,
		Timeout:    time.Duration(cfg.WebhookTimeoutSecs) * time.Second,
	})
	dispatcher.OnDeliverySuccess(func() { metrics.IncrementWebhookDelivery("success") })
	dispatcher.OnDeliveryFailed(func() { metrics.IncrementWebhookDelivery("failed") })
	dispatcher.OnDeliveryDone(metrics.ObserveWebhookDeliveryDuration)
	emailService := email.NewService(emailRepo, smtpRepo, templateRepo, suppressionRepo, limiter, dispatcher, cfg.DevMode)
	emailService.SetSettings(settingsProvider)
	emailService.SetVersionRepos(versionRepo, localizationRepo)
	emailService.SetContactRepo(contactRepo)
	emailService.SetDomainVerification(domainRepo, userRepo)
	if producer != nil {
		emailService.SetEnqueuer(producer)
	}
	emailService.OnSent(metrics.IncrementEmailSent)
	emailService.OnFailed(metrics.IncrementEmailFailed)
	emailService.OnQueued(metrics.IncrementEmailQueued)

	// Cache
	statsCache := cache.New(redisClient)

	// Handlers
	userSeeder := seeder.New(templateRepo, stylesheetRepo, versionRepo, localizationRepo, languageRepo)
	userHandler := handlers.NewUserHandler(userRepo, cfg.JWTSecret, userSeeder, bus)
	userHandler.SetSettings(settingsProvider)
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})

	// Seed default platform settings
	go seeder.SeedDefaultSettings(settingRepo)

	// Start worker connection monitor
	wm := workermon.New(inspector, bus, 15*time.Second)
	wm.Start(ctx)

	r := &Router{
		app: app,
		cfg: cfg,
		v1:  app.Group("/api/v1"),
		mw: routerMiddleware{
			jwtAuth:           middlewares.JWTAuth(cfg, sessionStore),
			jwtAdminAuth:      middlewares.JWTAdminAuth(cfg, sessionStore),
			jwtAdminQueryAuth: middlewares.JWTAdminQueryAuth(cfg, sessionStore),
			loginLimiter:      middlewares.LoginRateLimitMiddleware(limiter),
			apiKey:            middlewares.APIKeyAuthMiddleware(apiKeyService, userRepo, apiKeyRepo),
		},
		h: routerHandlers{
			health:          handlers.NewHealthHandler(db, redisClient),
			user:            userHandler,
			email:           handlers.NewEmailHandler(emailService, emailRepo, bus, statsCache),
			apiKey:          handlers.NewAPIKeyHandler(apiKeyService, apiKeyRepo, userSettingRepo, auditLogger),
			template:        handlers.NewTemplateHandler(templateRepo, stylesheetRepo, versionRepo, localizationRepo, emailService),
			version:         handlers.NewTemplateVersionHandler(templateRepo, versionRepo),
			localization:    handlers.NewTemplateLocalizationHandler(templateRepo, versionRepo, localizationRepo, stylesheetRepo),
			language:        handlers.NewLanguageHandler(languageRepo),
			stylesheet:      handlers.NewStyleSheetHandler(stylesheetRepo),
			smtp:            handlers.NewSMTPHandler(smtpRepo, domainRepo, auditLogger),
			webhook:         handlers.NewWebhookHandler(webhookRepo, auditLogger),
			webhookDelivery: handlers.NewWebhookDeliveryHandler(webhookDeliveryRepo),
			dashboard:       handlers.NewDashboardHandler(db, statsCache, webhookDeliveryRepo),
			domain:          handlers.NewDomainHandler(domainRepo),
			bounce:          handlers.NewBounceHandler(bounceRepo, suppressionRepo, emailRepo),
			suppression:     handlers.NewSuppressionHandler(suppressionRepo),
			contact:         handlers.NewContactHandler(contactRepo, suppressionRepo),
			contactList:     handlers.NewContactListHandler(contactListRepo),
			admin:           handlers.NewAdminHandler(db, statsCache, userRepo, apiKeyRepo, emailRepo, webhookDeliveryRepo, inspector, bus, userSeeder, cfg.EmbeddedWorker),
			server:          handlers.NewServerHandler(serverRepo, auditLogger),
			event:           handlers.NewEventHandler(eventRepo, bus),
			analytics:       handlers.NewAnalyticsHandler(repositories.NewAnalyticsRepository(db), statsCache),
			setting:         handlers.NewSettingHandler(settingRepo, auditLogger),
			userSetting:     handlers.NewUserSettingHandler(userSettingRepo),
			userData:        handlers.NewUserDataHandler(db, templateRepo, versionRepo, localizationRepo, stylesheetRepo, languageRepo, contactRepo, contactListRepo, webhookRepo, suppressionRepo, userSettingRepo),
			session:         handlers.NewSessionHandler(sessionRepo, sessionStore),
		},
	}

	// Session management
	r.h.user.SetSessionRepo(sessionRepo)

	// Email content privacy
	r.h.email.SetSettings(settingsProvider)
	r.h.admin.SetEmailSettings(settingsProvider)

	if cronManager != nil {
		r.h.cron = handlers.NewCronHandler(cronManager)
	}

	r.registerRoutes()
}

func (r *Router) registerRoutes() {
	// Request ID middleware
	r.app.Use(okapi.RequestID())

	// Prometheus metrics
	if r.cfg.MetricsEnabled {
		r.app.Use(metrics.PrometheusMiddleware())
		r.app.Get("/metrics", metrics.MetricsHandler(), okapi.DocHide())
	}

	// Register all route definitions
	r.app.Register(r.healthRoutes()...)
	r.app.Register(r.infoRoute())
	r.app.Register(r.authRoutes()...)
	r.app.Register(r.apiAuthRoutes()...)
	r.app.Register(r.userRoutes()...)
	r.app.Register(r.adminRoutes()...)
	r.app.Register(r.adminSSERoutes()...)

	// Dashboard UI (static files + SPA fallback)
	webDir := r.cfg.WebDir
	if info, err := os.Stat(webDir); err == nil && info.IsDir() {
		r.app.Static("/assets", filepath.Join(webDir, "assets"))

		indexPath := filepath.Join(webDir, "index.html")
		r.app.NoRoute(func(c *okapi.Context) error {
			path := c.Request().URL.Path
			if strings.HasPrefix(path, "/api/v1/") || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") || strings.HasPrefix(path, "/metrics") || strings.HasPrefix(path, "/docs") {
				return c.AbortNotFound("not found")
			}
			filePath := filepath.Join(webDir, filepath.Clean(path))
			if stat, err := os.Stat(filePath); err == nil && !stat.IsDir() {
				http.ServeFile(c.ResponseWriter(), c.Request(), filePath)
				return nil
			}
			http.ServeFile(c.ResponseWriter(), c.Request(), indexPath)
			return nil
		})
	}
}
