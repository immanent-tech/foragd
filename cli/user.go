// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
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
	Block  BlockUserCmd  `cmd:"block"  help:"Block user"`
	List   ListUserCmd   `cmd:"list"   help:"List user details"`
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

type ListUserCmd struct {
	UserArgs
}

func (c *ListUserCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	user, err := service.GetUser(ctx, c.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	var output strings.Builder

	// Name/IDs.
	fmt.Fprintf(
		&output,
		"%s (Internal ID: %s External ID: %s)\n",
		user.GetNickname(),
		user.GetID(),
		user.GetExternalID(),
	)
	// Email.
	fmt.Fprintf(&output, "Email: %s", user.GetEmail())
	if !user.Metadata.EmailVerified {
		color.New(color.FgYellow).Fprintf(&output, " (unverified)\n")
	} else {
		fmt.Fprintf(&output, "\n")
	}
	// Blocked status.
	if user.Metadata.Blocked {
		color.New(color.FgRed).Fprintf(&output, "BLOCKED\n")
	}
	// Policy acceptance.
	if user.Metadata.PoliciesAccepted {
		color.New(color.FgYellow).Fprintf(&output, "Not accepted policies\n")
	}
	// Timestamps.
	fmt.Fprintf(&output, "Created at: %s\n", user.CreatedAt.Format(time.DateTime))
	fmt.Fprintf(&output, "Updated at: %s\n", user.UpdatedAt.Format(time.DateTime))
	fmt.Fprintf(&output, "Last Login: %s (count: %d)\n", user.LastLogin.Format(time.DateTime), user.LoginCount)
	// Subscription/trial details.
	switch {
	case user.InTrial():
		color.New(color.FgYellow).Fprint(&output, "In Trial")
		fmt.Fprintf(&output, " (Ends: %s)\n", user.CreatedAt.Add(models.DefaultTrialPeriod).Format(time.DateTime))
	case user.InTrialGracePeriod():
		color.New(color.FgYellow).Fprint(&output, "In Trial (Grace Period)")
		fmt.Fprintf(
			&output,
			" (Ends: %s)\n",
			user.CreatedAt.Add(models.DefaultTrialPeriod+7*24*time.Hour).Format(time.DateTime),
		)
	default:
		color.New(color.FgGreen).Fprintf(&output, "Active Subscription\n")
		fmt.Fprintf(&output, "Subscription Type: %s\n", *user.UserSubscriptionType)
		switch *user.UserSubscriptionType {
		case models.UserSubscriptionTypePaddle:
			if subscription, err := user.Subscription.AsPaddleSubscription(); err != nil {
				slogctx.FromCtx(ctx).Error("Cannot parse Paddle subscription.", slog.Any("error", err))
			} else {
				fmt.Fprintf(
					&output,
					"Subscription ID: %s Customer ID: %s\n",
					subscription.SubscriptionID,
					subscription.CustomerID,
				)
				fmt.Fprintf(
					&output,
					"Backend Status: %s\n",
					subscription.SubscriptionStatus,
				)
			}
		}
	}

	fmt.Fprintf(os.Stdout, "%s", output.String())

	return nil
}
