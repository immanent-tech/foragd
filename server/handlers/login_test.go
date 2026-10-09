/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/auth0/go-auth0/v2/management"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/server/handlers"
)

func TestManager_HandleLogin(t *testing.T) {
	tests := []struct {
		name     string
		authFunc func(t *testing.T) *MoqAuthenticator
		mgrFunc  func(t *testing.T) *handlers.Manager
		wantCode int
	}{
		{
			name: "already authenticated",
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{}
				mock.IsAuthenticatedFunc = func(ctx context.Context) bool {
					return true
				}
				return mock
			},
			mgrFunc:  func(t *testing.T) *handlers.Manager { return setupManager(t) },
			wantCode: http.StatusFound,
		},
		{
			name: "generate auth URL success",
			authFunc: func(t *testing.T) *MoqAuthenticator {
				authURL := "https://auth.example.com/login"
				mock := &MoqAuthenticator{
					GenerateAuthURLFunc: func(signup bool) (*auth0.AuthURLResult, error) {
						return &auth0.AuthURLResult{
							State: "test-state",
							URL:   authURL,
						}, nil
					},
					IsAuthenticatedFunc: func(ctx context.Context) bool {
						return false
					},
					StoreAuthRequestFunc: func(ctx context.Context, r *auth0.AuthURLResult) {},
				}
				return mock
			},
			mgrFunc: func(t *testing.T) *handlers.Manager {
				mgr := setupManager(t)
				mgr.SessionMgr = &MoqSessionManager{
					PutFunc:        func(ctx context.Context, key string, value any) {},
					RenewTokenFunc: func(ctx context.Context) error { return nil },
				}
				return mgr
			},
			wantCode: http.StatusFound,
		},
		{
			name: "generate auth URL error",
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{
					GenerateAuthURLFunc: func(signup bool) (*auth0.AuthURLResult, error) {
						return nil, http.ErrNotSupported
					},
					IsAuthenticatedFunc: func(ctx context.Context) bool { return false },
				}
				return mock
			},
			mgrFunc: func(t *testing.T) *handlers.Manager {
				mgr := setupManager(t)
				mgr.SessionMgr = &MoqSessionManager{
					PutFunc:        func(ctx context.Context, key string, value any) {},
					GetFunc:        func(ctx context.Context, key string) any { return nil },
					RenewTokenFunc: func(ctx context.Context) error { return nil },
				}
				return mgr
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "renew token error",
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{
					GenerateAuthURLFunc: func(signup bool) (*auth0.AuthURLResult, error) {
						return &auth0.AuthURLResult{}, nil
					},
					IsAuthenticatedFunc: func(ctx context.Context) bool {
						return false
					},
				}
				return mock
			},
			mgrFunc: func(t *testing.T) *handlers.Manager {
				mgr := setupManager(t)
				mgr.SessionMgr = &MoqSessionManager{
					PutFunc: func(ctx context.Context, key string, value any) {},
					GetFunc: func(ctx context.Context, key string) any { return nil },
					RenewTokenFunc: func(ctx context.Context) error {
						return errors.New("failed!")
					},
				}
				return mgr
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := tt.mgrFunc(t)
			auth := tt.authFunc(t)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/login?subscription_plan=pro", nil)

			mgr.HandleLogin(auth)(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("HandleLogin got status %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestManager_HandleLoginCallback(t *testing.T) {
	tests := []struct {
		name        string
		formValues  map[string]string
		authFunc    func(t *testing.T) *MoqAuthenticator
		authMgrFunc func(t *testing.T) *MoqAuthManager
		userSvc     *MoqUserService
		mgrFunc     func(t *testing.T) *handlers.Manager
		sendFunc    func(t *testing.T) *MoqEmailSender
		wantCode    int
	}{
		{
			name: "error from auth0",
			formValues: map[string]string{
				"error":             "invalid_request",
				"error_description": "Invalid state parameter",
			},
			authMgrFunc: func(t *testing.T) *MoqAuthManager { return &MoqAuthManager{} },
			mgrFunc:     func(t *testing.T) *handlers.Manager { return setupManager(t) },
			wantCode:    http.StatusInternalServerError,
		},
		{
			name: "state validation error",
			formValues: map[string]string{
				"code":  "auth-code-here",
				"state": "invalid-state",
			},
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{}
				mock.ValidateStateFunc = func(ctx context.Context, got string) error {
					return errors.New("state mismatch")
				}
				return mock
			},
			authMgrFunc: func(t *testing.T) *MoqAuthManager { return &MoqAuthManager{} },
			mgrFunc:     func(t *testing.T) *handlers.Manager { return setupManager(t) },
			wantCode:    http.StatusInternalServerError,
		},
		{
			name: "perform exchange error",
			formValues: map[string]string{
				"code":  "auth-code-here",
				"state": "valid-state",
			},
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{}
				mock.ValidateStateFunc = func(ctx context.Context, got string) error {
					return nil
				}
				mock.PerformExchangeFunc = func(ctx context.Context, code string) (*models.UserProfileResponse, error) {
					return nil, http.ErrNotSupported
				}
				return mock
			},
			authMgrFunc: func(t *testing.T) *MoqAuthManager { return &MoqAuthManager{} },
			mgrFunc:     func(t *testing.T) *handlers.Manager { return setupManager(t) },
			wantCode:    http.StatusInternalServerError,
		},
		{
			name: "existing user with backend error",
			formValues: map[string]string{
				"code":  "auth-code-here",
				"state": "valid-state",
			},
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{}
				mock.ValidateStateFunc = func(ctx context.Context, got string) error {
					return nil
				}
				mock.PerformExchangeFunc = func(ctx context.Context, code string) (*models.UserProfileResponse, error) {
					return &models.UserProfileResponse{
						Subject: "external:123",
					}, nil
				}
				return mock
			},
			authMgrFunc: func(t *testing.T) *MoqAuthManager { return &MoqAuthManager{} },
			userSvc: &MoqUserService{
				GetUserByExternalIDFunc: func(ctx context.Context, id string) (*models.User, error) {
					return nil, http.ErrNotSupported
				},
			},
			mgrFunc: func(t *testing.T) *handlers.Manager {
				mgr := setupManager(t)
				mgr.SessionMgr = &MoqSessionManager{
					PutFunc: func(ctx context.Context, key string, value any) {},
				}
				return mgr
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "existing user sync success",
			formValues: map[string]string{
				"code":  "auth-code-here",
				"state": "valid-state",
			},
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{}
				mock.ValidateStateFunc = func(ctx context.Context, got string) error {
					return nil
				}
				mock.PerformExchangeFunc = func(ctx context.Context, code string) (*models.UserProfileResponse, error) {
					return &models.UserProfileResponse{
						Subject: "external:123",
					}, nil
				}
				mock.ConsumeReturnToFunc = func(ctx context.Context) (string, error) {
					return "/dashboard", nil
				}
				return mock
			},
			authMgrFunc: func(t *testing.T) *MoqAuthManager {
				return &MoqAuthManager{
					GetUserFunc: func(ctx context.Context, userID string) (*auth0.UserData, error) {
						return &auth0.UserData{
							GetUserResponseContent: &management.GetUserResponseContent{
								UserID: new("external:123"),
							},
						}, nil
					},
				}
			},
			userSvc: &MoqUserService{
				GetUserByExternalIDFunc: func(ctx context.Context, id string) (*models.User, error) {
					return &models.User{
						UserID:         "user-123",
						Email:          "user@example.com",
						ExternalUserID: "external:123",
					}, nil
				},
				UpdateUserFunc: func(ctx context.Context, user *models.User, updates map[string]any) error {
					return nil
				},
			},
			mgrFunc: func(t *testing.T) *handlers.Manager {
				mgr := setupManager(t)
				mgr.SessionMgr = &MoqSessionManager{
					PutFunc: func(ctx context.Context, key string, value any) {},
				}
				return mgr
			},
			wantCode: http.StatusFound,
		},
		{
			name: "new user creation",
			formValues: map[string]string{
				"code":  "auth-code-here",
				"state": "valid-state",
			},
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{}
				mock.ValidateStateFunc = func(ctx context.Context, got string) error {
					return nil
				}
				mock.PerformExchangeFunc = func(ctx context.Context, code string) (*models.UserProfileResponse, error) {
					return &models.UserProfileResponse{
						Subject: "external:123",
					}, nil
				}
				mock.ConsumeReturnToFunc = func(ctx context.Context) (string, error) {
					return "", errors.New("not set")
				}
				return mock
			},
			authMgrFunc: func(t *testing.T) *MoqAuthManager {
				return &MoqAuthManager{
					GetUserFunc: func(ctx context.Context, userID string) (*auth0.UserData, error) {
						return &auth0.UserData{
							GetUserResponseContent: &management.GetUserResponseContent{
								UserID:      new("external:123"),
								Email:       new("test@foragd.app"),
								LoginsCount: new(1),
							},
						}, nil
					},
				}
			},
			mgrFunc: func(t *testing.T) *handlers.Manager {
				mgr := setupManager(t)
				mgr.SessionMgr = &MoqSessionManager{
					PutFunc: func(ctx context.Context, key string, value any) {},
				}
				return mgr
			},
			userSvc: &MoqUserService{
				GetUserByExternalIDFunc: func(ctx context.Context, id string) (*models.User, error) {
					return nil, models.ErrNotFound
				},
				AddUserFunc: func(ctx context.Context, user *models.User) error {
					return nil
				},
			},
			sendFunc: func(t *testing.T) *MoqEmailSender {
				return &MoqEmailSender{
					SendFunc: func(ctx context.Context, options ...resend.EmailOption) error { return nil },
				}
			},
			wantCode: http.StatusFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := tt.mgrFunc(t)
			var auth *MoqAuthenticator
			if tt.authFunc != nil {
				auth = tt.authFunc(t)
			}
			userSvc := tt.userSvc
			authMgr := tt.authMgrFunc(t)
			sender := &MoqEmailSender{}
			if tt.sendFunc != nil {
				sender = tt.sendFunc(t)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/login/callback", nil)
			req.Form = make(url.Values)
			for k, v := range tt.formValues {
				req.Form.Add(k, v)
			}

			mgr.HandleLoginCallback(userSvc, authMgr, auth, sender)(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("HandleLoginCallback got status %d, want %d", rec.Code, tt.wantCode)
			}

			if rec.Code == http.StatusFound {
				location := rec.Header().Get("Location")
				t.Logf("redirected to: %s", location)
			}
		})
	}
}

func TestManager_HandleRefreshToken(t *testing.T) {
	tests := []struct {
		name       string
		authFunc   func(t *testing.T) *MoqAuthenticator
		wantCode   int
		wantHeader bool
	}{
		{
			name: "token fresh",
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{}
				mock.EnsureFreshFunc = func(ctx context.Context) error {
					return nil
				}
				return mock
			},
			wantCode:   http.StatusNoContent,
			wantHeader: false,
		},
		{
			name: "session expired",
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{}
				mock.EnsureFreshFunc = func(ctx context.Context) error {
					return auth0.ErrSessionExpired
				}
				return mock
			},
			wantCode:   http.StatusUnauthorized,
			wantHeader: true,
		},
		{
			name: "error refreshing token",
			authFunc: func(t *testing.T) *MoqAuthenticator {
				mock := &MoqAuthenticator{}
				mock.EnsureFreshFunc = func(ctx context.Context) error {
					return http.ErrNotSupported
				}
				return mock
			},
			wantCode:   http.StatusServiceUnavailable,
			wantHeader: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			auth := tt.authFunc(t)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/refresh-token", nil)

			mgr.HandleRefreshToken(auth)(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("HandleRefreshToken got status %d, want %d", rec.Code, tt.wantCode)
			}

			if tt.wantHeader {
				if location := rec.Header().Get("HX-Redirect"); location != "/login" {
					t.Errorf("expected HX-Redirect to /login, got %s", location)
				}
			}
		})
	}
}

// Helper function to create a mock Authenticator with ValidateState and PerformExchange.
func makeAuthenticatorForCallback(
	t *testing.T,
	validateStateError error,
	performExchange *models.UserProfileResponse,
) *MoqAuthenticator {
	t.Helper()
	mock := &MoqAuthenticator{}
	mock.ValidateStateFunc = func(ctx context.Context, got string) error {
		return validateStateError
	}
	mock.PerformExchangeFunc = func(ctx context.Context, code string) (*models.UserProfileResponse, error) {
		return performExchange, nil
	}
	return mock
}
