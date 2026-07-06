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
var initClient = sync.OnceValue(func() error {
	cfg, err := gcp.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	errorClient, err = errorreporting.NewClient(context.Background(), cfg.ProjectID, errorreporting.Config{
		ServiceName:    cfg.Service,
		ServiceVersion: config.GetVersion(),
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
	if err := initClient(); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to report error to google cloud console.",
			slog.Any("error", err))
		return
	}
	errorClient.Report(errorreporting.Entry{
		Error: rawErr,
	})
}
