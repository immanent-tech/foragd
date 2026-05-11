// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package youtube

import (
	"context"
	"fmt"
	"slices"
	"sync"

	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"github.com/immanent-tech/foragd/models"
	gcp "github.com/immanent-tech/foragd/providers/google"
)

var client *youtube.Service

var initClient = func(ctx context.Context) error {
	err := sync.OnceValue(func() error {
		cfg, err := gcp.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		client, err = youtube.NewService(ctx, option.WithAPIKey(cfg.APIKey))
		if err != nil {
			return fmt.Errorf("load youtube client: %w", err)
		}
		slogctx.FromCtx(ctx).Info("Youtube client created.")
		return nil
	})()
	if err != nil {
		return err
	}
	return nil
}

// Channel represents a Youtube channel. It contains the id, title, description, published date and an image.
type Channel struct {
	ID          string
	Title       string
	Description string
	PublishedAt string
	Image       *models.RemoteImage
}

// FindChannels will search Youtube for channels matching the given query string and return a slice of matches.
func FindChannels(ctx context.Context, query string) ([]Channel, error) {
	if err := initClient(ctx); err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}
	// Then e.g.:
	resp, err := client.Search.List([]string{"snippet"}).
		Type("channel").
		Q(query).
		MaxResults(5).
		Do()
	if err != nil {
		return nil, fmt.Errorf("search channels: %w", err)
	}

	results := make([]Channel, 0, len(resp.Items))

	for result := range slices.Values(resp.Items) {
		details := Channel{
			ID:          result.Id.ChannelId,
			Title:       result.Snippet.ChannelTitle,
			Description: result.Snippet.Description,
			PublishedAt: result.Snippet.PublishedAt,
		}
		if result.Snippet.Thumbnails != nil {
			details.Image = models.NewRemoteImage(result.Snippet.Thumbnails.Default.Url, details.Title)
		}
		results = append(results, details)
	}

	results = slices.CompactFunc(results, func(a, b Channel) bool {
		return a.ID == b.ID
	})

	return results, nil
}
