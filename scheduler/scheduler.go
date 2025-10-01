// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package scheduler contains code for the scheduler backend that handles managing background jobs for the application.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
)

const (
	defaultOutdatedThreshold = 50 * time.Second
)

var ErrScheduler = errors.New("scheduler encountered an error")

// Manager contains data for managing a scheduler instance.
type Manager struct {
	id        string
	db        *elastic.API
	queue     quartz.JobQueue
	scheduler quartz.Scheduler
}

var manager *Manager

// Run starts the scheduler manager.
func Run(ctx context.Context, env string) error {
	// Load the config.
	err := config.Load(configPrefix, configEnvPrefix, cfg)
	if err != nil {
		return fmt.Errorf("unable to load config: %w", err)
	}

	esClient, err := elastic.Connect(ctx, env)
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	ctx = elastic.SetupIndexAliases(ctx)

	jobQueue, err := NewJobQueue(ctx, esClient)
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	// logger := &logger{Logger: slogctx.FromCtx(ctx)}
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(defaultOutdatedThreshold),
		quartz.WithQueue(jobQueue, &sync.Mutex{}),
		// quartz.WithLogger(logger),
	)
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	manager = &Manager{
		id:        models.NewID(models.SchedulerPFX),
		db:        esClient,
		queue:     jobQueue,
		scheduler: scheduler,
	}

	// Setup get new feeds job.
	job, err := NewGetNewFeedsJob()
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}
	_, err = manager.scheduler.GetScheduledJob(job.JobDetail().JobKey())
	if err != nil && errors.Is(err, quartz.ErrJobNotFound) {
		err = manager.scheduler.ScheduleJob(job.JobDetail(), job.Trigger())
		if err != nil {
			return fmt.Errorf("failed to start scheduler: %w", err)
		}
	}
	// Check for new feeds on startup.
	err = job.JobDetail().Job().Execute(ctx)
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	scheduler.Start(ctx)

	slogctx.FromCtx(ctx).DebugContext(ctx, "Scheduler started.")

	// Start a webserver for health probes.
	svr := &http.Server{
		Addr:        net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		ReadTimeout: cfg.ReadTimeout,
		// WriteTimeout: ServerConfig.WriteTimeout,
		IdleTimeout: cfg.IdleTimeout,
	}
	http.HandleFunc("/startupProbe", func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprint(res, "Started.")
	})
	var wg sync.WaitGroup
	wg.Add(1)
	// Listen for shutdown events and process them.
	go func() {
		wg.Done()
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		err := svr.Shutdown(context.Background())
		// Can't do much here except for logging any errors
		if err != nil {
			slog.Error("Error occurred when trying to shut down server.",
				slog.Any("error", err),
			)
		}
	}()
	err = svr.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) { // graceful shutdown
		wg.Wait()
	} else if err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	<-ctx.Done()
	scheduler.Stop()
	slogctx.FromCtx(ctx).DebugContext(ctx, "Scheduler stopped.")
	return nil
}
