/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/handlers"
)

func TestManager_HandleSetupImport(t *testing.T) {
	tests := []struct {
		name        string
		subSvc      handlers.SubscriptionsService
		userSvc     handlers.UserService
		emailSender handlers.EmailSender
		ctxSetup    func(ctx context.Context) context.Context
		want        int
	}{
		{
			name: "ok",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) { return nil, nil },
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			ctx := t.Context()
			if tt.ctxSetup != nil {
				ctx = tt.ctxSetup(ctx)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/import", nil)

			mgr.HandleSetupImport(tt.subSvc, tt.userSvc, tt.emailSender)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestManager_HandleStartImport(t *testing.T) {
	tests := []struct {
		name      string
		importSvc handlers.Importer
		ctxSetup  func(ctx context.Context) context.Context
		want      int
	}{
		{
			name: "ok",
			importSvc: &MoqImporter{
				StartImportFunc: func(ctx context.Context, file *models.OPMLFile) (string, error) {
					return "test", nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			ctx := t.Context()
			if tt.ctxSetup != nil {
				ctx = tt.ctxSetup(ctx)
			}

			// Set up OPML file upload.
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			f, err := os.Open("testdata/test.opml")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			part, err := writer.CreateFormFile("source", filepath.Base(f.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(part, f); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodPost, "/import", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set(htmx.HeaderRequest, "true")

			rec := httptest.NewRecorder()

			mgr.HandleStartImport(tt.importSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestManager_HandleImportStatus(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		importSvc handlers.Importer
		subSvc    handlers.SubscriptionsService
		ctxSetup  func(ctx context.Context) context.Context
		want      int
	}{
		{
			name: "no job id and no imports",
			importSvc: &MoqImporter{
				GetAllImportsFunc: func(ctx context.Context, userID models.UserID) ([]*models.ImportStatus, error) {
					return nil, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "no job id and previous imports",
			importSvc: &MoqImporter{
				GetAllImportsFunc: func(ctx context.Context, userID models.UserID) ([]*models.ImportStatus, error) {
					return []*models.ImportStatus{
						{
							CreatedAt: time.Now(),
							Status:    models.ImportStatusStatusDone,
						},
					}, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "error",
			importSvc: &MoqImporter{
				GetAllImportsFunc: func(ctx context.Context, userID models.UserID) ([]*models.ImportStatus, error) {
					return nil, errors.New("error!")
				},
			},
			want: http.StatusInternalServerError,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			ctx := t.Context()
			if tt.ctxSetup != nil {
				ctx = tt.ctxSetup(ctx)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/import/status", nil)

			mgr.HandleImportStatus(tt.importSvc, tt.subSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}

		})
	}
}

func TestManager_HandleImportStatusWithID(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		importSvc handlers.Importer
		subSvc    handlers.SubscriptionsService
		ctxSetup  func(ctx context.Context) context.Context
		jobID     string
		want      int
	}{
		{
			name:  "job id and no error",
			jobID: "test",
			importSvc: &MoqImporter{
				GetImportStatusFunc: func(ctx context.Context, jobID string) (*models.ImportStatus, []*models.ImportResult, error) {
					return &models.ImportStatus{
							Status: models.ImportStatusStatusPending,
						},
						nil,
						nil
				},
			},
			subSvc: &MoqSubscriptionsService{
				InvalidateFunc: func(userID models.UserID) {},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				ctx = withURLParams(t, ctx, map[string]string{"jobID": "test"})
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name:  "job id and error",
			jobID: "test",
			importSvc: &MoqImporter{
				GetImportStatusFunc: func(ctx context.Context, jobID string) (*models.ImportStatus, []*models.ImportResult, error) {
					return nil, nil, errors.New("error!")
				},
			},
			subSvc: &MoqSubscriptionsService{
				InvalidateFunc: func(userID models.UserID) {},
			},
			want: http.StatusInternalServerError,
			ctxSetup: func(ctx context.Context) context.Context {
				ctx = withURLParams(t, ctx, map[string]string{"jobID": "test"})
				return models.UserToCtx(ctx, &models.User{})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			ctx := t.Context()
			if tt.ctxSetup != nil {
				ctx = tt.ctxSetup(ctx)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/import/status/"+tt.jobID, nil)

			mgr.HandleImportStatus(tt.importSvc, tt.subSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
