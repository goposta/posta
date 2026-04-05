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

package main

import (
	"context"
	"time"

	"github.com/goposta/posta/internal/config"
	cronpkg "github.com/goposta/posta/internal/cron"
	"github.com/goposta/posta/internal/cron/jobs"
	"github.com/goposta/posta/internal/metrics"
	"github.com/goposta/posta/internal/routes"
	"github.com/goposta/posta/internal/services/crypto"
	"github.com/goposta/posta/internal/services/retry"
	"github.com/goposta/posta/internal/services/seeder"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/goposta/posta/internal/services/tracking"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/migration"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/goposta/posta/internal/worker"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi/okapicli"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type serverResources struct {
	producer    *worker.Producer
	cronManager *cronpkg.Manager
	blobStore   blob.Store
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

			// Initialize SMTP password encryption
			crypto.Init(cfg.EncryptionKey)

			if err := storage.SeedAdmin(res.db, cfg.AdminEmail, cfg.AdminPassword); err != nil {
				logger.Fatal("failed to seed admin user", "error", err)
			}

			seedDefaults(res.db, cfg.AdminEmail)

			// Initialize blob storage (S3 or filesystem) for attachments
			if cfg.BlobProvider != "" {
				bs, err := blob.New(blob.Config{
					Provider:          cfg.BlobProvider,
					S3Endpoint:        cfg.BlobS3Endpoint,
					S3Region:          cfg.BlobS3Region,
					S3Bucket:          cfg.BlobS3Bucket,
					S3AccessKeyID:     cfg.BlobS3AccessKey,
					S3SecretAccessKey: cfg.BlobS3SecretKey,
					S3UseSSL:          cfg.BlobS3UseSSL,
					S3ForcePathStyle:  cfg.BlobS3PathStyle,
					FSBasePath:        cfg.BlobFSPath,
				})
				if err != nil {
					logger.Fatal("failed to initialize blob storage", "error", err)
				}
				res.blobStore = bs
				logger.Info("blob storage initialized", "provider", cfg.BlobProvider)
			}

			if cfg.EmbeddedWorker && !cfg.DevMode {
				res.producer = worker.NewProducer(cfg.Redis.Addr, cfg.Redis.Password, cfg.WorkerMaxRetries)
				startEmbeddedWorker(res.db, cfg, res.blobStore)
			}

			if !cfg.DevMode {
				res.cronManager = initCronManager(res.db, cfg)
			}

			routes.InitRoutes(app, res.db, res.redis, cfg, res.producer, res.cronManager, res.blobStore, context.Background())

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
func startEmbeddedWorker(db *gorm.DB, cfg *config.Config, blobStore blob.Store) {
	dispatcher := newWebhookDispatcher(db, cfg)

	handler := worker.NewEmailSendHandler(
		repositories.NewEmailRepository(db),
		repositories.NewSMTPRepository(db),
		repositories.NewServerRepository(db),
		repositories.NewDomainRepository(db),
		repositories.NewContactRepository(db),
		dispatcher,
	)
	if blobStore != nil {
		handler.SetBlobStore(blobStore)
	}
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

	// Campaign processor
	campaignProducer := worker.NewProducer(cfg.Redis.Addr, cfg.Redis.Password, cfg.WorkerMaxRetries)
	trackingRepo := repositories.NewTrackingRepository(db)
	trackingService := tracking.NewService(trackingRepo, cfg.AppWebURL, []byte(cfg.JWTSecret))
	campaignDispatcher := newWebhookDispatcher(db, cfg)
	campaignProcessor := worker.NewCampaignProcessor(
		repositories.NewCampaignRepository(db),
		repositories.NewCampaignMessageRepository(db),
		repositories.NewSubscriberListRepository(db),
		repositories.NewSubscriberRepository(db),
		repositories.NewEmailRepository(db),
		repositories.NewTemplateRepository(db),
		repositories.NewTemplateVersionRepository(db),
		repositories.NewTemplateLocalizationRepository(db),
		trackingService,
		campaignProducer,
		campaignDispatcher,
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TypeEmailSend, handler.ProcessTask)
	mux.HandleFunc(worker.TypeCampaignStart, campaignProcessor.HandleCampaignStart)
	mux.HandleFunc(worker.TypeCampaignBatch, campaignProcessor.HandleCampaignBatch)

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
		repositories.NewTrackingRepository(db),
		settingsProvider,
	))
	manager.Register(jobs.NewDailyReportJob(repositories.NewUserSettingRepository(db)))
	manager.Register(jobs.NewAccountCleanupJob(repositories.NewUserRepository(db)))
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
		ProxyURL:   cfg.WebhookProxyURL,
	}
}
