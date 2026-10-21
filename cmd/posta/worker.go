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

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/cron/jobs"
	"github.com/goposta/posta/internal/metrics"
	"github.com/goposta/posta/internal/services/crypto"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/services/inbound"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/goposta/posta/internal/services/tracking"
	"github.com/goposta/posta/internal/services/workermon"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/goposta/posta/internal/worker"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

func runWorker() error {
	cfg := config.New()
	if err := cfg.InitWorker(); err != nil {
		logger.Fatal("worker configuration is not usable", "error", err)
	}
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
	handler.SetCampaignMessageRepo(repositories.NewCampaignMessageRepository(db))
	handler.SetCampaignRepo(repositories.NewCampaignRepository(db))
	handler.SetSuppressionRepo(repositories.NewSuppressionRepository(db))
	handler.SetBounceRepo(repositories.NewBounceRepository(db))
	handler.SetAutoSuppress(cfg.AutoSuppressOnReject)
	handler.SetStamper(email.NewStamper("Posta", config.Version, []byte(cfg.JWTSecret)))
	handler.OnSent(func() {
		metrics.IncrementEmailSent()
		metrics.DecrementEmailQueued()
	})
	handler.OnFailed(func() {
		metrics.IncrementEmailFailed()
		metrics.DecrementEmailQueued()
	})

	exhaustedHandler := worker.NewExhaustedErrorHandler(
		repositories.NewEmailRepository(db), dispatcher, func() {
			metrics.IncrementEmailFailed()
			metrics.DecrementEmailQueued()
		},
	)

	errorHandlers := []asynq.ErrorHandler{exhaustedHandler}
	if cfg.InboundEnabled {
		inboundExhausted := worker.NewInboundExhaustedErrorHandler(
			repositories.NewInboundEmailRepository(db), metrics.IncrementInboundFailed,
		)
		errorHandlers = append(errorHandlers, inboundExhausted)
	}

	srv := asynq.NewServer(
		cfg.Redis.AsynqRedisOpt(),
		asynq.Config{
			Concurrency: cfg.WorkerConcurrency,
			Queues: map[string]int{
				worker.QueueTransactional: 6,
				worker.QueueBulk:          3,
				worker.QueueLow:           1,
			},
			ErrorHandler: worker.ChainErrorHandlers(errorHandlers...),
		},
	)

	// Campaign processor
	campaignProducer := worker.NewProducer(cfg.Redis.AsynqRedisOpt(), cfg.WorkerMaxRetries)
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

	// Notification service + daily report handler
	notifier := notification.NewService(
		cfg.SystemSMTP, "Posta", cfg.AppWebURL,
		repositories.NewUserRepository(db),
		repositories.NewUserSettingRepository(db),
		repositories.NewWorkspaceRepository(db),
	)
	notifier.SetSMTPRepo(repositories.NewSMTPRepository(db))
	dailyReportHandler := worker.NewDailyReportHandler(
		notifier,
		repositories.NewAnalyticsRepository(db),
		repositories.NewBounceRepository(db),
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TypeEmailSend, handler.ProcessTask)
	mux.HandleFunc(worker.TypeCampaignStart, campaignProcessor.HandleCampaignStart)
	mux.HandleFunc(worker.TypeCampaignBatch, campaignProcessor.HandleCampaignBatch)
	mux.HandleFunc(jobs.TypeDailyReport, dailyReportHandler.ProcessTask)

	if cfg.InboundEnabled {
		inboundHandler := worker.NewInboundProcessHandler(
			repositories.NewInboundEmailRepository(db),
			newWebhookDispatcher(db, cfg),
			cfg.ApiBaseURL,
			[]byte(cfg.JWTSecret),
		)
		inboundHandler.OnForwarded(metrics.IncrementInboundForwarded)
		inboundHandler.OnFailed(metrics.IncrementInboundFailed)
		mux.HandleFunc(worker.TypeInboundProcess, inboundHandler.ProcessTask)

		parseProducer := worker.NewProducer(cfg.Redis.AsynqRedisOpt(), cfg.WorkerMaxRetries)
		parseBus := eventbus.New(repositories.NewEventRepository(db))
		parseSvc := inbound.NewService(
			repositories.NewInboundEmailRepository(db),
			repositories.NewDomainRepository(db),
			repositories.NewSuppressionRepository(db),
			inbound.Config{
				MaxMessageSize:    cfg.InboundMaxMessageSize,
				MaxAttachmentSize: cfg.InboundMaxAttachSize,
			},
		)
		if blobStore != nil {
			parseSvc.SetBlobStore(blobStore)
		}
		parseSvc.SetEnqueuer(parseProducer)
		parseSvc.SetEventBus(parseBus)
		attachThreadRouter(parseSvc, db, cfg, parseProducer)
		parseHandler := worker.NewInboundParseHandler(
			repositories.NewInboundEmailRepository(db),
			parseSvc,
			parseProducer,
		)
		mux.HandleFunc(worker.TypeInboundParse, parseHandler.ProcessTask)
	}

	if cfg.MessagesEnabled {
		messageHandler := worker.NewMessageProcessHandler(
			repositories.NewMessageRepository(db),
			repositories.NewFormRepository(db),
			repositories.NewWorkspaceRepository(db),
			newWebhookDispatcher(db, cfg),
			notifier,
			cfg.AppWebURL,
		)
		messageHandler.OnNotified(metrics.IncrementMessageNotification)
		messageHandler.SetRealtime(realtime.NewRedis(cfg.Redis.Client))
		mux.HandleFunc(worker.TypeMessageProcess, messageHandler.ProcessTask)
		mux.HandleFunc(worker.TypeMessageReply, messageHandler.ProcessReplyTask)
	}

	// Publish worker's build info
	workermon.StartHeartbeat(context.Background(), cfg.Redis.Client, config.Version, config.CommitID)

	var healthSrv *worker.HealthServer
	if cfg.WorkerHealthEnabled {
		addr := fmt.Sprintf(":%d", cfg.WorkerHealthPort)
		healthSrv = worker.NewHealthServer(addr, db, cfg.Redis.Client)
		stop := healthSrv.Start()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			stop(ctx)
		}()
		logger.Info("worker health endpoints listening",
			"addr", addr, "paths", "/healthz /readyz /metrics")
	}

	logger.Info("Posta worker started",
		"version", config.Version,
		"concurrency", cfg.WorkerConcurrency,
	)

	// Readiness flips before Run blocks and back once it returns, so a draining
	// or stopped worker stops advertising itself as ready and an orchestrator
	// routes nothing new at it.
	if healthSrv != nil {
		healthSrv.SetProcessing(true)
		defer healthSrv.SetProcessing(false)
	}

	return srv.Run(mux)
}
