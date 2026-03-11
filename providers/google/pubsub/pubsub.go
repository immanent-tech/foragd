// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/grpc/codes"

	"github.com/googleapis/gax-go/v2/apierror"

	gcp "github.com/immanent-tech/foragd/providers/google"
)

var client *pubsub.Client

// GetClient returns the pubsub client.
func GetClient(ctx context.Context) (*pubsub.Client, error) {
	if err := sync.OnceValue(func() error {
		cfg, err := gcp.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		client, err = pubsub.NewClient(ctx, cfg.ProjectID)
		if err != nil {
			return fmt.Errorf("new client: %w", err)
		}
		return nil
	})(); err != nil {
		return nil, fmt.Errorf("get pubsub client: %w", err)
	}
	return client, nil
}

// TopicExists returns a boolean indicating whether the given pubsub topic exists. If there was a problem, it will
// return a non-nil error as the second return value.
func TopicExists(ctx context.Context, topic string) (bool, error) {
	var err error
	client, err = GetClient(ctx)
	if err != nil {
		return false, fmt.Errorf("get client: %w", err)
	}

	if _, err := client.TopicAdminClient.GetTopic(ctx, &pubsubpb.GetTopicRequest{
		Topic: topic,
	}); err != nil {
		if apiErr, ok := errors.AsType[*apierror.APIError](err); ok {
			if apiErr.GRPCStatus().Code() == codes.NotFound {
				return false, nil
			}
		}
		return false, fmt.Errorf("get topic: %w", err)
	}

	return true, nil
}

// CreateTopic creates the topic with the given name.
func CreateTopic(ctx context.Context, topic string) error {
	var err error
	client, err = GetClient(ctx)
	if err != nil {
		return fmt.Errorf("get client: %w", err)
	}

	req := &pubsubpb.Topic{
		Name: topic,
	}
	_, err = client.TopicAdminClient.CreateTopic(ctx, req)
	if err != nil {
		return fmt.Errorf("create topic: %w", err)
	}
	return nil
}
