// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gerror

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"cloud.google.com/go/errorreporting"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	gcp "github.com/immanent-tech/foragd/providers/google"
)

var errorClient *errorreporting.Client

//nolint:sloglint // no context passed.
var Init = sync.OnceValue(func() error {
	cfg, err := gcp.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	errorClient, err = errorreporting.NewClient(context.Background(), cfg.ProjectID, errorreporting.Config{
		ServiceName:    cfg.Service,
		ServiceVersion: config.GetVersion(),
		OnError: func(err error) {
			slog.Error("Create new error client failed.", slog.Any("error", err))
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
func ReportError(ctx context.Context, rawErr error) {
	if errorClient == nil {
		slogctx.FromCtx(ctx).Warn("Unable to report error to google cloud console.")
		return
	}
	errorClient.Report(errorreporting.Entry{
		Error: rawErr,
	})
}
