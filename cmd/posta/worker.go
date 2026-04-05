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
	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/metrics"
	"github.com/goposta/posta/internal/services/crypto"
	"github.com/goposta/posta/internal/services/tracking"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/goposta/posta/internal/worker"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

func runWorker() error {
	cfg := config.New()
	_ = cfg.InitWorker()
	cfg.InitStorage()

	db := cfg.Database.DB
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	// Initialize SMTP password encryption
	crypto.Init(cfg.EncryptionKey)

	// Initialize blob storage for attachment retrieval
	var blobStore blob.Store
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
		blobStore = bs
		logger.Info("blob storage initialized", "provider", cfg.BlobProvider)
	}

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

	logger.Info("Posta worker started",
		"version", config.Version,
		"concurrency", cfg.WorkerConcurrency,
	)

	return srv.Run(mux)
}
