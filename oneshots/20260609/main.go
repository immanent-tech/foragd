package main

import (
	"context"
	"log/slog"
	"slices"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/logging"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
)

func main() {
	ctx := slogctx.NewCtx(context.Background(), logging.New())

	if err := elastic.Connect(); err != nil {
		panic(err)
	}

	elasticSvc, err := service.LoadElasticService()
	if err != nil {
		panic(err)
	}

	slogctx.FromCtx(ctx).Info("Get all users.")
	users, err := elastic.SearchAll[*models.User](
		ctx,
		elasticSvc.GetIndexRO(service.UsersIndex),
		query.MatchAll(),
		5000,
	)
	if err != nil {
		panic(err)
	}

	slogctx.FromCtx(ctx).Info("Update user objects.")
	for user := range slices.Values(users) {
		if user.Subscription != nil {
			if _, err := user.Subscription.AsPaddleSubscription(); err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to retrieve user's paddle subscription.",
					slog.String("user_id", user.GetID()),
					slog.Any("error", err),
				)
			}
			slogctx.FromCtx(ctx).Info("Adding subscription type to user.",
				slog.String("user_id", user.GetID()),
			)
			user.UserSubscriptionType = new(models.UserSubscriptionTypePaddle)
		}
	}

	newIndexName := elastic.GenerateIndexName("users")
	if _, err := elastic.CreateIndexIfNotExists(ctx, "users"); err != nil {
		panic(err)
	}
	if err := elastic.UpdateIndexAlias(ctx, elasticSvc.GetIndexRW(service.UsersIndex), newIndexName); err != nil {
		panic(err)
	}
	// results, err := elastic.BulkUpdate(ctx, schema.UsersIndexRW(), users...)
	// if err != nil {
	// 	godump.Dump(results)
	// 	panic(err)
	// }
	if err = elastic.UpdateIndexAlias(ctx, elasticSvc.GetIndexRO(service.UsersIndex), newIndexName); err != nil {
		panic(err)
	}

}
