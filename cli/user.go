// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
)

type UserArgs struct {
	UserID models.UserID `arg:"" help:"ID of object to delete"`
}

// UserCmd contains sub commands for managing users.
type UserCmd struct {
	Delete DeleteUserCmd `cmd:"delete" help:"Delete user"`
	Block  BlockUserCmd  `cmd:"delete" help:"Block user"`
}

type DeleteUserCmd struct {
	UserArgs
}

type BlockUserCmd struct {
	UserArgs

	Value bool `help:"Block status."`
}

func (c *DeleteUserCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	user, err := service.GetUser(ctx, c.UserID)
	if err != nil {
		return fmt.Errorf("unable to delete user: %w", err)
	}

	// Delete the user.
	if err := elastic.DeleteDoc(ctx, schema.UsersIndexRW(), user.GetID()); err != nil {
		return fmt.Errorf("unable to delete user %s: %w", user.GetID(), err)
	}
	// Delete the user's subscriptions.
	if err := elastic.DeleteDocs(ctx, schema.SubscriptionsIndexRW(), query.Term("user_id", user.GetID())); err != nil {
		return fmt.Errorf("unable to delete user %s: %w", user.GetID(), err)
	}
	// Delete any scheduled jobs for the user.
	if err := elastic.DeleteDocs(
		ctx,
		schema.SchedulerIndexRW(),
		query.Term("job_data.user_id", user.GetID()),
	); err != nil {
		slogctx.FromCtx(ctx).Warn("Could not delete scheduled jobs for user.",
			slog.String("user_id", user.GetID()),
			slog.Any("error", err),
		)
	}

	// Delete from Auth0 backend
	if err := auth0.DeleteUser(ctx, user.GetExternalID()); err != nil {
		return fmt.Errorf("unable to delete user %s: %w", user.GetID(), err)
	}

	slogctx.FromCtx(ctx).Info("Deleted user.",
		slog.String("user_id", user.GetID()),
	)

	return nil
}

func (c *BlockUserCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	user, err := service.GetUser(ctx, c.UserID)
	if err != nil {
		return fmt.Errorf("unable to delete user: %w", err)
	}

	metadata := user.Metadata
	metadata.Blocked = c.Value

	if err := service.UpdateUser(ctx, user, map[string]any{
		"metadata": metadata,
	}); err != nil {
		return fmt.Errorf("unable to set blocked status of user %s: %w", user.GetID(), err)
	}

	slogctx.FromCtx(ctx).Info("Set blocked status of user.",
		slog.String("user_id", user.GetID()),
		slog.Bool("blocked", c.Value),
	)

	return nil
}
