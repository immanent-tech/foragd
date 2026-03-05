// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"cloud.google.com/go/errorreporting"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
)

var errorClient *errorreporting.Client

var initErrorClient = func(ctx context.Context) error {
	err := sync.OnceValue(func() error {
		cfg, err := LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		errorClient, err = errorreporting.NewClient(ctx, cfg.ProjectID, errorreporting.Config{
			ServiceName:    cfg.Service,
			ServiceVersion: config.Version,
			OnError: func(err error) {
				slogctx.FromCtx(ctx).Error("Error reporting failed.", slog.Any("error", err))
			},
		})
		if err != nil {
			return fmt.Errorf("load error reporting client: %w", err)
		}
		return nil
	})()
	if err != nil {
		return err
	}
	return nil
}

// ReportError reports an error to the Cloud Console. The error client autopopulates the error context of the error. For
// more details about the context see: https://cloud.google.com/error-reporting/reference/rest/v1beta1/ErrorContext.
func ReportError(ctx context.Context, err error) {
	if err := initErrorClient(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to report error to google cloud console.", slog.Any("error", err))
		return
	}
	errorClient.Report(errorreporting.Entry{
		Error: err,
	})
}
