// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gerror

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"cloud.google.com/go/errorreporting"

	"github.com/immanent-tech/go-base/config"

	gcp "github.com/immanent-tech/foragd/providers/google"
)

var errorClient *errorreporting.Client

//nolint:sloglint // no context passed.
var initClient = sync.OnceValue(func() error {
	cfg, err := gcp.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return fmt.Errorf("load app config: %w", err)
	}

	errorClient, err = errorreporting.NewClient(context.Background(), cfg.ProjectID, errorreporting.Config{
		ServiceName:    cfg.Service,
		ServiceVersion: appCfg.Version,
		OnError: func(err error) {
			slog.Error("Could not report error to cloud console", slog.Any("error", err))
		},
	})
	if err != nil {
		return fmt.Errorf("load error reporting client: %w", err)
	}

	slog.Info("GCP error client created.")
	return nil
})

// ReportError reports an error to the Cloud Console. The error client auto populates the error context of the error. For
// more details about the context see: https://cloud.google.com/error-reporting/reference/rest/v1beta1/ErrorContext.
func ReportError(rawErr error) {
	if err := initClient(); err != nil {
		slog.Warn("Unable to report error to google cloud console.",
			slog.Any("error", err))
		return
	}
	errorClient.Report(errorreporting.Entry{
		Error: rawErr,
	})
}

func CloseClient() {
	if errorClient == nil {
		// Not initialised/used.
		return
	}
	done := make(chan error, 1)
	go func() { done <- errorClient.Close() }()

	select {
	case err := <-done:
		if err != nil {
			slog.Error("Error client graceful shutdown failed.", slog.Any("error", err))
			return
		}
		slog.Info("GCP error client shutdown.")
	case <-time.After(15 * time.Second):
		slog.Error("Error client shutdown timed out.")
	}
}
