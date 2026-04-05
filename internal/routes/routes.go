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

	"github.com/goposta/posta/internal/config"
	cronpkg "github.com/goposta/posta/internal/cron"
	"github.com/goposta/posta/internal/handlers"
	"github.com/goposta/posta/internal/metrics"
	"github.com/goposta/posta/internal/middlewares"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/auth"
	"github.com/goposta/posta/internal/services/cache"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/eventbus"
	planpkg "github.com/goposta/posta/internal/services/plan"
	"github.com/goposta/posta/internal/services/ratelimit"
	"github.com/goposta/posta/internal/services/seeder"
	sessionpkg "github.com/goposta/posta/internal/services/session"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/goposta/posta/internal/services/tracking"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/services/workermon"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/goposta/posta/internal/worker"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/okapi"
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
	workspace         okapi.Middleware
	optionalWorkspace okapi.Middleware
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
	admin           *handlers.AdminHandler
	workspace       *handlers.WorkspaceHandler
	server          *handlers.ServerHandler
	event           *handlers.EventHandler
	analytics       *handlers.AnalyticsHandler
	setting         *handlers.SettingHandler
	userSetting     *handlers.UserSettingHandler
	userData        *handlers.UserDataHandler
	session         *handlers.SessionHandler
	cron            *handlers.CronHandler
	oauth           *handlers.OAuthHandler
	oauthAdmin      *handlers.OAuthAdminHandler
	subscriber      *handlers.SubscriberHandler
	subscriberList  *handlers.SubscriberListHandler
	campaign        *handlers.CampaignHandler
	tracking        *handlers.TrackingHandler
	bounceWebhook   *handlers.BounceWebhookHandler
	workspaceData   *handlers.WorkspaceDataHandler
	plan            *handlers.PlanHandler
}

func InitRoutes(app *okapi.Okapi, db *gorm.DB, redisClient *redis.Client, cfg *config.Config, producer *worker.Producer, cronManager *cronpkg.Manager, blobStore blob.Store, ctx context.Context) {
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
	workspaceRepo := repositories.NewWorkspaceRepository(db)

	// Session store (Redis-backed blacklist)
	sessionStore := sessionpkg.NewStore(redisClient)

	// Services
	bus := eventbus.New(eventRepo)
	auditLogger := audit.NewLogger(bus)
	apiKeyService := auth.NewAPIKeyService(apiKeyRepo)
	settingsProvider := settings.NewProvider(settingRepo)
	planRepo := repositories.NewPlanRepository(db)
	planService := planpkg.NewService(planRepo, settingsProvider)
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
	emailService.SetPlanLimits(&emailPlanAdapter{planService})
	emailService.SetVersionRepos(versionRepo, localizationRepo)
	emailService.SetContactRepo(contactRepo)
	emailService.SetDomainVerification(domainRepo, userRepo)
	if producer != nil {
		emailService.SetEnqueuer(producer)
	}
	if blobStore != nil {
		emailService.SetBlobStore(blobStore)
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
			loginLimiter:      loginLimiterMiddleware(cfg, limiter),
			apiKey:            middlewares.APIKeyAuthMiddleware(apiKeyService, userRepo, apiKeyRepo),
			workspace:         middlewares.RequireWorkspaceMiddleware(workspaceRepo),
			optionalWorkspace: middlewares.OptionalWorkspaceMiddleware(workspaceRepo),
		},
		h: routerHandlers{
			health:          handlers.NewHealthHandler(db, redisClient),
			user:            userHandler,
			email:           handlers.NewEmailHandler(emailService, emailRepo, bus, statsCache),
			apiKey:          handlers.NewAPIKeyHandler(apiKeyService, apiKeyRepo, userSettingRepo, auditLogger),
			template:        handlers.NewTemplateHandler(templateRepo, stylesheetRepo, versionRepo, localizationRepo, languageRepo, emailService),
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
			admin:           handlers.NewAdminHandler(db, statsCache, userRepo, apiKeyRepo, emailRepo, webhookDeliveryRepo, inspector, bus, userSeeder, cfg.EmbeddedWorker),
			workspace:       handlers.NewWorkspaceHandler(workspaceRepo, userRepo, db),
			server:          handlers.NewServerHandler(serverRepo, auditLogger),
			event:           handlers.NewEventHandler(eventRepo, bus),
			analytics:       handlers.NewAnalyticsHandler(repositories.NewAnalyticsRepository(db), statsCache),
			setting:         handlers.NewSettingHandler(settingRepo, auditLogger),
			userSetting:     handlers.NewUserSettingHandler(userSettingRepo),
			userData:        handlers.NewUserDataHandler(db, templateRepo, versionRepo, localizationRepo, stylesheetRepo, languageRepo, contactRepo, webhookRepo, suppressionRepo, userSettingRepo),
			session:         handlers.NewSessionHandler(sessionRepo, sessionStore),
		},
	}

	// Session management
	r.h.user.SetSessionRepo(sessionRepo)
	r.h.admin.SetSessionRepo(sessionRepo, sessionStore)

	// Plans
	r.h.plan = handlers.NewPlanHandler(planRepo, workspaceRepo, planService, auditLogger)
	r.h.admin.SetWorkspaceRepo(workspaceRepo, planRepo)
	r.h.workspace.SetPlanService(planService)
	r.h.apiKey.SetQuota(planService, db)
	r.h.domain.SetQuota(planService, db)
	r.h.smtp.SetQuota(planService, db)

	// Email content privacy
	r.h.email.SetSettings(settingsProvider)
	r.h.admin.SetEmailSettings(settingsProvider)

	if cronManager != nil {
		r.h.cron = handlers.NewCronHandler(cronManager)
	}

	// OAuth
	oauthProviderRepo := repositories.NewOAuthProviderRepository(db)
	oauthAccountRepo := repositories.NewOAuthAccountRepository(db)
	ssoRepo := repositories.NewWorkspaceSSORepository(db)
	oauthService := auth.NewOAuthService(oauthProviderRepo, oauthAccountRepo, userRepo)

	callbackBase := cfg.OAuthCallbackBaseURL
	if callbackBase == "" {
		callbackBase = cfg.AppWebURL
	}

	r.h.oauth = handlers.NewOAuthHandler(
		oauthService, oauthProviderRepo, oauthAccountRepo,
		userRepo, sessionRepo, cfg.JWTSecret,
		userSeeder, bus, redisClient, callbackBase, cfg.AppWebURL,
	)
	r.h.oauthAdmin = handlers.NewOAuthAdminHandler(oauthProviderRepo, ssoRepo)

	// Subscribers
	subscriberRepo := repositories.NewSubscriberRepository(db)
	subscriberListRepo := repositories.NewSubscriberListRepository(db)
	r.h.subscriber = handlers.NewSubscriberHandler(subscriberRepo)
	r.h.subscriberList = handlers.NewSubscriberListHandler(subscriberListRepo, subscriberRepo)

	// Campaigns
	campaignRepo := repositories.NewCampaignRepository(db)
	campaignMessageRepo := repositories.NewCampaignMessageRepository(db)
	r.h.campaign = handlers.NewCampaignHandler(
		campaignRepo, campaignMessageRepo,
		subscriberListRepo, subscriberRepo, templateRepo, producer,
	)

	// Tracking
	trackingRepo := repositories.NewTrackingRepository(db)
	trackingService := tracking.NewService(trackingRepo, cfg.AppWebURL, []byte(cfg.JWTSecret))
	r.h.tracking = handlers.NewTrackingHandler(trackingRepo, campaignMessageRepo, campaignRepo, subscriberRepo, trackingService)

	// Bounce webhook
	r.h.bounceWebhook = handlers.NewBounceWebhookHandler(subscriberRepo, emailRepo, campaignMessageRepo)

	// Workspace data export/import
	r.h.workspaceData = handlers.NewWorkspaceDataHandler(
		db, workspaceRepo, templateRepo, versionRepo, localizationRepo,
		stylesheetRepo, languageRepo, contactRepo, contactListRepo,
		webhookRepo, suppressionRepo, smtpRepo, domainRepo,
		subscriberRepo, subscriberListRepo,
	)

	// Auto-seed Google provider if configured
	if cfg.GoogleOAuthClientID != "" {
		if _, err := oauthProviderRepo.FindBySlug("google"); err != nil {
			_ = oauthProviderRepo.Create(&models.OAuthProvider{
				Name:         "Google",
				Slug:         "google",
				Type:         models.OAuthProviderGoogle,
				ClientID:     cfg.GoogleOAuthClientID,
				ClientSecret: cfg.GoogleOAuthClientSecret,
				Scopes:       "openid email profile",
				Enabled:      true,
				AutoRegister: true,
			})
		}
	}

	r.registerRoutes()
}

// emailPlanAdapter adapts planpkg.Service to the email.PlanLimitsProvider interface.
type emailPlanAdapter struct {
	svc *planpkg.Service
}

func (a *emailPlanAdapter) EffectiveLimits(workspaceID *uint) *email.PlanLimits {
	l := a.svc.EffectiveLimits(workspaceID)
	return &email.PlanLimits{
		HourlyRateLimit:     l.HourlyRateLimit,
		DailyRateLimit:      l.DailyRateLimit,
		MaxAttachmentSizeMB: l.MaxAttachmentSizeMB,
		MaxBatchSize:        l.MaxBatchSize,
	}
}

// workspaceHeaderRequired is a reusable route option documenting the required workspace header.
var workspaceHeaderRequired = okapi.DocHeader("X-Posta-Workspace-Id", "integer", "Workspace ID (required for workspace-scoped endpoints)", true)

// workspaceHeaderOptional is a reusable route option documenting the optional workspace header.
var workspaceHeaderOptional = okapi.DocHeader("X-Posta-Workspace-Id", "integer", "Workspace ID (optional, omit for personal mode)", false)

// loginLimiterMiddleware returns the login rate limit middleware when enabled,
// or a pass-through middleware when disabled.
func loginLimiterMiddleware(cfg *config.Config, limiter *ratelimit.RedisLimiter) okapi.Middleware {
	if !cfg.AuthRateLimitEnabled {
		return func(c *okapi.Context) error {
			return c.Next()
		}
	}
	return middlewares.LoginRateLimitMiddleware(limiter)
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
	r.app.Register(r.workspaceRoutes()...)
	r.app.Register(r.oauthRoutes()...)
	r.app.Register(r.trackingRoutes()...)
	r.app.Register(r.trackingAnalyticsRoutes()...)
	r.app.Register(r.bounceWebhookRoutes()...)
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
