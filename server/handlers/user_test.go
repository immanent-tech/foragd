/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/server/handlers"
)

func TestManager_ValidateUserSubscriptionLimits(t *testing.T) {
	tests := []struct {
		name       string
		subSvc     handlers.SubscriptionsService
		userSvc    handlers.UserService
		senderSvc  handlers.EmailSender
		ctx        func(ctx context.Context) context.Context
		nextCalled bool // whether the next handler should be called.
	}{
		{
			name: "already over subscription limit",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					subscriptions := make(models.Subscriptions, 0)
					for range models.MaxSubscriptions + 1 {
						subscriptions = append(subscriptions, &models.Subscription{
							Type: models.SubscriptionTypeFeed,
						})
					}
					return subscriptions, nil
				},
			},
			userSvc: &MoqUserService{
				UpdateUserFunc: func(ctx context.Context, user *models.User, updates map[string]any) error { return nil },
			},
			ctx: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{
					Metadata: models.UserMetadata{
						SubscriptionLimit: &models.UserLimit{
							Exceeded: true,
						},
					},
				})
			},
			nextCalled: false,
		},
		{
			name: "already over email newsletter limit",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					subscriptions := make(models.Subscriptions, 0)
					for range models.MaxEmailNewsletters + 1 {
						subscriptions = append(subscriptions, &models.Subscription{
							Type: models.SubscriptionTypeEmail,
						})
					}
					return subscriptions, nil
				},
			},
			userSvc: &MoqUserService{
				UpdateUserFunc: func(ctx context.Context, user *models.User, updates map[string]any) error { return nil },
			},
			ctx: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{
					Metadata: models.UserMetadata{
						NewsletterLimit: &models.UserLimit{
							Exceeded: true,
						},
					},
				})
			},
			nextCalled: false,
		},
		{
			name: "corrected subscription limit",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					subscriptions := make(models.Subscriptions, 0)
					for range models.MaxSubscriptions - 100 {
						subscriptions = append(subscriptions, &models.Subscription{
							Type: models.SubscriptionTypeFeed,
						})
					}
					return subscriptions, nil
				},
			},
			userSvc: &MoqUserService{
				UpdateUserFunc: func(ctx context.Context, user *models.User, updates map[string]any) error { return nil },
			},
			ctx: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{
					Metadata: models.UserMetadata{
						SubscriptionLimit: &models.UserLimit{
							Exceeded: true,
						},
					},
				})
			},
			nextCalled: true,
		},
		{
			name: "corrected email newsletter limit",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					subscriptions := make(models.Subscriptions, 0)
					for range models.MaxEmailNewsletters - 10 {
						subscriptions = append(subscriptions, &models.Subscription{
							Type: models.SubscriptionTypeEmail,
						})
					}
					return subscriptions, nil
				},
			},
			userSvc: &MoqUserService{
				UpdateUserFunc: func(ctx context.Context, user *models.User, updates map[string]any) error { return nil },
			},
			ctx: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{
					Metadata: models.UserMetadata{
						NewsletterLimit: &models.UserLimit{
							Exceeded: true,
						},
					},
				})
			},
			nextCalled: true,
		},
		{
			name: "over subscription limit",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					subscriptions := make(models.Subscriptions, 0)
					for range models.MaxSubscriptions + 1 {
						subscriptions = append(subscriptions, &models.Subscription{
							Type: models.SubscriptionTypeFeed,
						})
					}
					return subscriptions, nil
				},
			},
			userSvc: &MoqUserService{
				UpdateUserFunc: func(ctx context.Context, user *models.User, updates map[string]any) error { return nil },
			},
			senderSvc: &MoqEmailSender{
				SendFunc: func(ctx context.Context, options ...resend.EmailOption) error { return nil },
			},
			ctx: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
			nextCalled: false,
		},
		{
			name: "over email newsletter limit",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					subscriptions := make(models.Subscriptions, 0)
					for range models.MaxEmailNewsletters + 1 {
						subscriptions = append(subscriptions, &models.Subscription{
							Type: models.SubscriptionTypeEmail,
						})
					}
					return subscriptions, nil
				},
			},
			userSvc: &MoqUserService{
				UpdateUserFunc: func(ctx context.Context, user *models.User, updates map[string]any) error { return nil },
			},
			senderSvc: &MoqEmailSender{
				SendFunc: func(ctx context.Context, options ...resend.EmailOption) error { return nil },
			},
			ctx: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
			nextCalled: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)

			// Set up context.
			ctx := t.Context()
			if tt.ctx != nil {
				ctx = tt.ctx(ctx)
			}

			// Set up middleware.
			var gotReq *http.Request
			h := mgr.ValidateSubscriptionLimits(tt.userSvc, tt.subSvc, tt.senderSvc)(
				http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
					gotReq = req
				}),
			)
			// Set up request.
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/subscriptions", nil)

			// Call middleware handler.
			h.ServeHTTP(rec, req.WithContext(ctx))

			if tt.nextCalled {
				if gotReq == nil {
					t.Fatal("next handler should be called")
				}
			} else {
				// For other pages, check redirect.
				if rec.Code != http.StatusForbidden && rec.Code != http.StatusTooManyRequests {
					t.Fatalf("wrong response code: got %d", rec.Code)
				}
			}
		})
	}
}
