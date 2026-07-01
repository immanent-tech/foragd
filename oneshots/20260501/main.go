package main

import (
	"context"
	"log/slog"
	"slices"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/service"
)

func main() {
	ctx := context.TODO()

	if err := elastic.Connect(); err != nil {
		panic(err)
	}

	resp, err := elastic.Search[*models.User](
		ctx,
		schema.UsersIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](
			query.Bool(
				query.Filter(
					// Account should be older than 30 days.
					query.Before("created_at", time.Now().UTC().Add(-30*24*time.Hour)),
					query.Bool(
						// Match any of these.
						query.Should(
							// Account was created but has never logged in.
							query.NumberRange("login_count", query.IntLessThan(2)),
							query.Bool(
								query.MustNot(
									query.Exists("last_login"),
								),
							),
							// Last login is more than 30 days ago.
							query.Before("last_login", time.Now().UTC().Add(-30*24*time.Hour)),
						),
					),
				),
				query.MustNot(
					// Should not already be pending deletion.
					query.Term("metadata.pending_deletion", true),
				),
			),
		),
	)
	if err != nil {
		panic(err)
	}

	for user := range slices.Values(resp.Results) {
		// Create and send email to user.
		email, err := resend.NewTemplatedEmail(
			"inactive-account-deletion-notice",
			resend.WithTo(user.GetEmail()),
			resend.WithTag(resend.TagCategory, resend.TagCategoryAccount),
		)
		if err != nil {
			panic(err)
		}
		if err := resend.SendEmail(ctx, resend.WithExistingEmail(email)); err != nil {
			panic(err)
		}
		metadata := user.Metadata
		metadata.PendingDeletion = new(true)
		if err := service.UpdateUser(ctx, user, map[string]any{
			"metadata": metadata,
		}); err != nil {
			panic(err)
		}
		slogctx.FromCtx(ctx).Info("Sent email and marked account for deletion.",
			slog.String("user_id", user.GetID()),
			slog.String("user_email", user.GetEmail()),
		)
	}
}
