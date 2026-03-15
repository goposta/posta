/*
 *  MIT License
 *
 * Copyright (c) 2026 Jonas Kaninda
 *
 *  Permission is hereby granted, free of charge, to any person obtaining a copy
 *  of this software and associated documentation files (the "Software"), to deal
 *  in the Software without restriction, including without limitation the rights
 *  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *  copies of the Software, and to permit persons to whom the Software is
 *  furnished to do so, subject to the following conditions:
 *
 *  The above copyright notice and this permission notice shall be included in all
 *  copies or substantial portions of the Software.
 *
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *  SOFTWARE.
 */

package main

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi/okapicli"
	"github.com/jkaninda/posta/internal/config"
	cronpkg "github.com/jkaninda/posta/internal/cron"
	"github.com/jkaninda/posta/internal/cron/jobs"
	"github.com/jkaninda/posta/internal/metrics"
	"github.com/jkaninda/posta/internal/routes"
	"github.com/jkaninda/posta/internal/services/retry"
	"github.com/jkaninda/posta/internal/services/seeder"
	"github.com/jkaninda/posta/internal/services/settings"
	"github.com/jkaninda/posta/internal/services/webhook"
	"github.com/jkaninda/posta/internal/storage"
	"github.com/jkaninda/posta/internal/storage/migration"
	"github.com/jkaninda/posta/internal/storage/repositories"
	"github.com/jkaninda/posta/internal/worker"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type serverResources struct {
	producer    *worker.Producer
	cronManager *cronpkg.Manager
	db          *gorm.DB
	redis       *redis.Client
}

func runServer(cli *okapicli.CLI) {
	cfg := config.New()
	app := cli.Okapi()
	if err := cfg.Initialize(app); err != nil {
		logger.Fatal("Failed to initialize Posta", "error", err)
	}
	res := &serverResources{}
	if err := cli.RunServer(&okapicli.RunOptions{
		ShutdownTimeout: 30 * time.Second,
		OnStart: func() {
			cfg.InitStorage()
			res.db = cfg.Database.DB
			res.redis = cfg.Redis.Client

			if err := migration.Run(res.db); err != nil {
				logger.Fatal("failed to run migrations", "error", err)
			}

			if err := storage.SeedAdmin(res.db, cfg.AdminEmail, cfg.AdminPassword); err != nil {
				logger.Fatal("failed to seed admin user", "error", err)
			}

			seedDefaults(res.db, cfg.AdminEmail)

			if cfg.EmbeddedWorker && !cfg.DevMode {
				res.producer = worker.NewProducer(cfg.Redis.Addr, cfg.Redis.Password, cfg.WorkerMaxRetries)
				startEmbeddedWorker(res.db, cfg)
			}

			if !cfg.DevMode {
				res.cronManager = initCronManager(res.db, cfg)
			}

			routes.InitRoutes(app, res.db, res.redis, cfg, res.producer, res.cronManager, context.Background())

			if res.cronManager != nil {
				res.cronManager.Start(context.Background())
			}

			if !cfg.DevMode {
				startRetryWorker(res.db, cfg, res.producer)
			}

			if cfg.DevMode {
				logger.Info("running in development mode - emails will NOT be sent")
			}
		},
		OnStarted: func() {
			logger.Info("Posta Server started", "version", config.Version, "port", cfg.Port)
		},
		OnShutdown: func() {
			shutdownServer(res)
		},
	}); err != nil {
		logger.Fatal("server error", "error", err)
	}
}

// seedDefaults seeds default templates, stylesheets, and languages for the admin user.
func seedDefaults(db *gorm.DB, adminEmail string) {
	userRepo := repositories.NewUserRepository(db)
	admin, err := userRepo.FindByEmail(adminEmail)
	if err != nil || admin == nil {
		return
	}
	s := seeder.New(
		repositories.NewTemplateRepository(db),
		repositories.NewStyleSheetRepository(db),
		repositories.NewTemplateVersionRepository(db),
		repositories.NewTemplateLocalizationRepository(db),
		repositories.NewLanguageRepository(db),
	)
	s.SeedUserDefaults(admin.ID, admin.Name)
}

// newWebhookDispatcher creates a configured webhook dispatcher with metrics hooks.
func newWebhookDispatcher(db *gorm.DB, cfg *config.Config) *webhook.Dispatcher {
	webhookRepo := repositories.NewWebhookRepository(db)
	whDeliveryRepo := repositories.NewWebhookDeliveryRepository(db)
	dispatcher := webhook.NewDispatcher(webhookRepo)
	dispatcher.SetDeliveryRepo(whDeliveryRepo)
	dispatcher.SetConfig(webhookConfig(cfg))
	dispatcher.OnDeliverySuccess(func() { metrics.IncrementWebhookDelivery("success") })
	dispatcher.OnDeliveryFailed(func() { metrics.IncrementWebhookDelivery("failed") })
	dispatcher.OnDeliveryDone(metrics.ObserveWebhookDeliveryDuration)
	return dispatcher
}

// startEmbeddedWorker starts an in-process asynq worker for email delivery.
func startEmbeddedWorker(db *gorm.DB, cfg *config.Config) {
	dispatcher := newWebhookDispatcher(db, cfg)

	handler := worker.NewEmailSendHandler(
		repositories.NewEmailRepository(db),
		repositories.NewSMTPRepository(db),
		repositories.NewServerRepository(db),
		repositories.NewDomainRepository(db),
		repositories.NewContactRepository(db),
		dispatcher,
	)
	handler.OnSent(metrics.IncrementEmailSent)
	handler.OnFailed(metrics.IncrementEmailFailed)

	exhaustedHandler := worker.NewExhaustedErrorHandler(
		repositories.NewEmailRepository(db), dispatcher, metrics.IncrementEmailFailed,
	)

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password},
		asynq.Config{
			Concurrency: cfg.WorkerConcurrency,
			Queues: map[string]int{
				worker.QueueTransactional: 6,
				worker.QueueBulk:          3,
				worker.QueueLow:           1,
			},
			ErrorHandler: exhaustedHandler,
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TypeEmailSend, handler.ProcessTask)

	go func() {
		if err := srv.Run(mux); err != nil {
			logger.Error("embedded worker error", "error", err)
		}
	}()

	logger.Info("async email delivery enabled (embedded worker)")
}

// initCronManager creates and registers scheduled jobs.
func initCronManager(db *gorm.DB, cfg *config.Config) *cronpkg.Manager {
	cronClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})
	settingsProvider := settings.NewProvider(repositories.NewSettingRepository(db))

	manager := cronpkg.NewManager(cronClient)
	manager.Register(jobs.NewRetentionCleanupJob(
		repositories.NewEmailRepository(db),
		repositories.NewEventRepository(db),
		repositories.NewWebhookDeliveryRepository(db),
		settingsProvider,
	))
	manager.Register(jobs.NewDailyReportJob(repositories.NewUserSettingRepository(db)))
	return manager
}

// startRetryWorker starts the background worker that retries failed emails.
func startRetryWorker(db *gorm.DB, cfg *config.Config, producer *worker.Producer) {
	dispatcher := newWebhookDispatcher(db, cfg)
	retryWorker := retry.NewWorker(
		repositories.NewEmailRepository(db),
		repositories.NewSMTPRepository(db),
		dispatcher,
		5*time.Minute,
	)
	if producer != nil {
		retryWorker.SetEnqueuer(producer)
	}
	retryWorker.OnSent(metrics.IncrementEmailSent)
	retryWorker.OnFailed(metrics.IncrementEmailFailed)
	retryWorker.OnRetry(metrics.IncrementEmailRetry)
	retryWorker.Start(context.Background())
}

// shutdownServer gracefully closes all server resources.
func shutdownServer(res *serverResources) {
	logger.Info("Posta Server shutting down gracefully...")

	if res.cronManager != nil {
		res.cronManager.Stop()
	}
	if res.producer != nil {
		if err := res.producer.Close(); err != nil {
			logger.Error("failed to close producer", "error", err)
		}
	}
	if res.redis != nil {
		if err := res.redis.Close(); err != nil {
			logger.Error("failed to close Redis", "error", err)
		}
	}
	if res.db != nil {
		if sqlDB, err := res.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}

	logger.Info("Posta Server shut down complete")
}

// webhookConfig builds a webhook.Config from the application configuration.
func webhookConfig(cfg *config.Config) webhook.Config {
	return webhook.Config{
		MaxRetries: cfg.WebhookMaxRetries,
		Timeout:    time.Duration(cfg.WebhookTimeoutSecs) * time.Second,
	}
}
