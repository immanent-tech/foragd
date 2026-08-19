// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package youtube

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

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

// SearchResult represents a search result retrieved from Youtube. It contains the id, title, description, published
// date and an image.
type SearchResult struct {
	ID          string
	Type        string
	Title       string
	Description string
	PublishedAt string
	Image       *models.RemoteImage
}

const (
	// TypeChannel is a channel search result.
	TypeChannel = "youtube#channel"
	// TypePlaylist is a playlist search result.
	TypePlaylist = "youtube#playlist"
)

// SourceURL returns the appropriate source URL for a search result.
func (r SearchResult) SourceURL() string {
	switch r.Type {
	case TypeChannel:
		return "https://www.youtube.com/feeds/videos.xml?channel_id=" + r.ID
	case TypePlaylist:
		return "https://www.youtube.com/feeds/videos.xml?playlist_id=" + r.ID
	default:
		return ""
	}
}

// FindVideos will search Youtube for channels/playlists matching the given query string and return a slice of matches.
func FindVideos(ctx context.Context, query string) ([]SearchResult, error) {
	if err := initClient(ctx); err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	// Search for channels or playlists that match the query.
	resp, err := client.Search.List([]string{"snippet"}).
		Type("channel", "playlist").
		Q(query).
		MaxResults(5).
		Do()
	if err != nil {
		return nil, fmt.Errorf("search channels: %w", err)
	}

	// Extract and format into our custom result format.
	results := make([]SearchResult, 0, len(resp.Items))
	for result := range slices.Values(resp.Items) {
		details := SearchResult{
			Type:        result.Id.Kind,
			Title:       result.Snippet.ChannelTitle,
			Description: result.Snippet.Description,
			PublishedAt: result.Snippet.PublishedAt,
		}
		switch result.Id.Kind {
		case TypeChannel:
			details.ID = result.Id.ChannelId
		case TypePlaylist:
			details.ID = result.Id.PlaylistId
		default:
			slogctx.Debug(ctx, "Unsupported result.",
				slog.String("type", result.Id.Kind))
			continue
		}
		if result.Snippet.Thumbnails != nil {
			details.Image = models.NewRemoteImage(result.Snippet.Thumbnails.Default.Url, details.Title)
		}
		results = append(results, details)
	}

	// Remove duplicates.
	results = slices.CompactFunc(results, func(a, b SearchResult) bool {
		return a.ID == b.ID
	})

	return results, nil
}

func FetchVideosSince(ctx context.Context, channelID string, since time.Time) ([]*youtube.Video, error) {
	if err := initClient(ctx); err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	// 1 unit
	uploadsID, err := getUploadsPlaylistID(ctx, channelID)
	if err != nil {
		return nil, err
	}

	// 1 unit per 50 videos
	videoIDs, err := getVideosSince(ctx, uploadsID, since)
	if err != nil {
		return nil, err
	}

	if len(videoIDs) == 0 {
		return nil, nil
	}

	// 1 unit per 50 videos
	return getVideoDetails(ctx, videoIDs)
}

func getUploadsPlaylistID(ctx context.Context, channelID string) (string, error) {
	if err := initClient(ctx); err != nil {
		return "", fmt.Errorf("init client: %w", err)
	}

	resp, err := client.Channels.List([]string{"contentDetails"}).
		Id(channelID).
		Do()
	if err != nil {
		return "", err
	}
	if len(resp.Items) == 0 {
		return "", errors.New("channel not found")
	}
	return resp.Items[0].ContentDetails.RelatedPlaylists.Uploads, nil
}

func getVideosSince(ctx context.Context, uploadsPlaylistID string, since time.Time) ([]string, error) {
	if err := initClient(ctx); err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	var videoIDs []string
	pageToken := ""

	for {
		call := client.PlaylistItems.List([]string{"contentDetails"}).
			PlaylistId(uploadsPlaylistID).
			MaxResults(50). // max allowed
			PageToken(pageToken)

		resp, err := call.Do()
		if err != nil {
			return nil, err
		}

		done := false
		for _, item := range resp.Items {
			publishedAt, err := time.Parse(time.RFC3339, item.ContentDetails.VideoPublishedAt)
			if err != nil {
				continue
			}

			// Playlist is newest-first, so stop once we go past our date
			if publishedAt.Before(since) {
				done = true
				break
			}

			videoIDs = append(videoIDs, item.ContentDetails.VideoId)
		}

		if done || resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return videoIDs, nil
}

func getVideoDetails(ctx context.Context, videoIDs []string) ([]*youtube.Video, error) {
	if err := initClient(ctx); err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	var videos []*youtube.Video

	// videos.list accepts up to 50 IDs per call
	for i := 0; i < len(videoIDs); i += 50 {
		end := i + 50
		if end > len(videoIDs) {
			end = len(videoIDs)
		}
		batch := videoIDs[i:end]

		resp, err := client.Videos.List([]string{"snippet", "contentDetails", "statistics"}).
			Id(batch...).
			Do()
		if err != nil {
			return nil, fmt.Errorf("list videos: %w", err)
		}
		videos = append(videos, resp.Items...)
	}

	return videos, nil
}
