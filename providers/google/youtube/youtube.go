// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package youtube

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"github.com/immanent-tech/foragd/models"
	gcp "github.com/immanent-tech/foragd/providers/google"
)

var initClient = sync.OnceValues(func() (*youtube.Service, error) {
	cfg, err := gcp.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	client, err := youtube.NewService(context.Background(), option.WithAPIKey(cfg.APIKey))
	if err != nil {
		return nil, fmt.Errorf("load youtube client: %w", err)
	}
	return client, nil
})

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

// Find will search Youtube for channels/playlists matching the given query string and return a slice of matches.
func Find(ctx context.Context, query string, count int64) ([]SearchResult, error) {
	client, err := initClient()
	if err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	// Search for channels or playlists that match the query.
	resp, err := client.Search.List([]string{"snippet"}).
		Type("channel", "playlist").
		Q(query).
		MaxResults(count).
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

func CreateFeeds(ctx context.Context, results ...SearchResult) (models.Feeds, error) {
	// Filter channel results.
	channels := slices.Collect(models.FilterSlice(results, func(e SearchResult) bool {
		return e.Type == TypeChannel
	}))
	// Filter playlist results.
	playlists := slices.Collect(models.FilterSlice(results, func(e SearchResult) bool {
		return e.Type == TypeChannel
	}))

	feeds := make(models.Feeds, 0)
	var mu sync.Mutex

	fetchJobs, jobCtx := errgroup.WithContext(ctx)
	defer jobCtx.Done()

	// Generate feeds for channels.
	fetchJobs.Go(func() error {
		ids := make([]string, 0, len(channels))
		for channel := range slices.Values(channels) {
			ids = append(ids, channel.ID)
		}
		channels, err := getChannelDetails(ids)
		if err != nil {
			return fmt.Errorf("fetch channel details: %w", err)
		}
		for channel := range slices.Values(channels) {
			feed := newChannelFeed(channel)
			if videos, err := FetchVideos(channel.Id, TypeChannel, 3); err == nil {
				feed.Items = CreateItems(feed, videos...)
			}
			mu.Lock()
			feeds = append(feeds, feed)
			mu.Unlock()
		}
		return nil
	})

	// Generate feeds for playlists.
	fetchJobs.Go(func() error {
		ids := make([]string, 0, len(channels))
		for playlist := range slices.Values(playlists) {
			ids = append(ids, playlist.ID)
		}
		playlists, err := getPlaylistDetails(ids)
		if err != nil {
			return fmt.Errorf("fetch channel details: %w", err)
		}
		for playlist := range slices.Values(playlists) {
			feed := newPlaylistFeed(playlist)
			if videos, err := FetchVideos(playlist.Id, TypePlaylist, 3); err == nil {
				feed.Items = CreateItems(feed, videos...)
			}
			mu.Lock()
			feeds = append(feeds, feed)
			mu.Unlock()
		}
		return nil
	})

	if err := fetchJobs.Wait(); err != nil {
		return nil, fmt.Errorf("create feeds: %w", err)
	}

	return feeds, nil
}

func newChannelFeed(channel *youtube.Channel) *models.Feed {
	id := "feed_" + strconv.FormatUint(xxh3.Hash([]byte(channel.Id)), 10)
	feed := &models.Feed{
		FeedID:         id,
		CreatedAt:      time.Now().UTC(),
		LastFetched:    models.UnixEpoch,
		Title:          channel.Snippet.Title,
		Description:    &channel.Snippet.Description,
		SourceType:     models.SourceTypeYoutube,
		SourceURLs:     []string{"https://www.youtube.com/feeds/videos.xml?channel_id=" + channel.Id},
		URL:            "https://youtube.com/channel/" + channel.Id,
		Language:       &channel.Snippet.DefaultLanguage,
		Domain:         "www.youtube.com",
		FetchMethod:    models.FeedFetchMethodDirect,
		UpdateInterval: int64(24 * time.Hour),
	}

	// Add categories.
	if keywords := channel.BrandingSettings.Channel.Keywords; keywords != "" {
		categories := make(models.Categories, 0)
		for category := range slices.Values(strings.Split(keywords, ",")) {
			categories = append(categories, category)
		}
		feed.Categories = categories
	}

	// Add source data.
	feed.SourceData = &models.Feed_SourceData{}
	feed.SourceData.FromYoutubeFeedData(models.YoutubeFeedData{ //nolint:errcheck // should never fail.
		ID:   channel.Id,
		Type: TypeChannel,
	})

	// Set the published date. If no published date in the source, set it to unix epoch.
	if pubDate, err := time.Parse(time.RFC3339, channel.Snippet.PublishedAt); err != nil {
		feed.Published = pubDate.UTC()
	} else {
		feed.Published = models.UnixEpoch
	}

	// Add any image found.
	if channel.BrandingSettings != nil && channel.BrandingSettings.Image != nil &&
		channel.BrandingSettings.Image.WatchIconImageUrl != "" {
		feed.Image = &models.RemoteImage{
			URL:   channel.BrandingSettings.Image.WatchIconImageUrl,
			Title: new(channel.Snippet.Title),
		}
	}

	return feed
}

func newPlaylistFeed(playlist *youtube.Playlist) *models.Feed {
	id := "feed_" + strconv.FormatUint(xxh3.Hash([]byte(playlist.Id)), 10)
	feed := &models.Feed{
		FeedID:         id,
		CreatedAt:      time.Now().UTC(),
		LastFetched:    models.UnixEpoch,
		Title:          playlist.Snippet.Title,
		Description:    &playlist.Snippet.Description,
		SourceType:     models.SourceTypeYoutube,
		SourceURLs:     []string{"https://www.youtube.com/feeds/videos.xml?playlist_id=" + playlist.Id},
		URL:            "https://youtube.com/playlist/" + playlist.Id,
		Language:       &playlist.Snippet.DefaultLanguage,
		Domain:         "www.youtube.com",
		FetchMethod:    models.FeedFetchMethodDirect,
		UpdateInterval: int64(24 * time.Hour),
	}

	// Add source data.
	feed.SourceData = &models.Feed_SourceData{}
	feed.SourceData.FromYoutubeFeedData(models.YoutubeFeedData{ //nolint:errcheck // should never fail.
		ID:   playlist.Id,
		Type: TypePlaylist,
	})

	// Set the published date. If no published date in the source, set it to unix epoch.
	if pubDate, err := time.Parse(time.RFC3339, playlist.Snippet.PublishedAt); err != nil {
		feed.Published = pubDate.UTC()
	} else {
		feed.Published = models.UnixEpoch
	}

	return feed
}

func FetchVideos(id string, objectType string, count int) ([]*youtube.Video, error) {
	client, err := initClient()
	if err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	if objectType == TypeChannel {
		uploadsID, err := getUploadsPlaylistID(id)
		if err != nil {
			return nil, fmt.Errorf("get channel playlist id: %w", err)
		}
		id = uploadsID
	}

	videoIDs := make([]string, 0, count)

	resp, err := client.PlaylistItems.List([]string{"snippet", "contentDetails"}).
		PlaylistId(id).
		MaxResults(int64(count)).Do()
	if err != nil {
		return nil, fmt.Errorf("get video details: %w", err)
	}

	for item := range slices.Values(resp.Items) {
		videoIDs = append(videoIDs, item.ContentDetails.VideoId)
	}

	return getVideoDetails(videoIDs)
}

func FetchVideosSince(channelID string, since time.Time) ([]*youtube.Video, error) {
	// 1 unit
	uploadsID, err := getUploadsPlaylistID(channelID)
	if err != nil {
		return nil, err
	}

	// 1 unit per 50 videos
	videoIDs, err := getVideosSince(uploadsID, since)
	if err != nil {
		return nil, err
	}

	if len(videoIDs) == 0 {
		return nil, nil
	}

	// 1 unit per 50 videos
	return getVideoDetails(videoIDs)
}

func CreateItems(feed *models.Feed, videos ...*youtube.Video) models.Items {
	items := make(models.Items, 0, len(videos))

	for video := range slices.Values(videos) {
		item := &models.Item{
			ItemID:      "item_" + strconv.FormatUint(xxh3.Hash([]byte(feed.GetID()+video.Id)), 10),
			FeedID:      feed.GetID(),
			Timestamp:   time.Now().UTC(),
			Title:       video.Snippet.Title,
			Description: &video.Snippet.Description,
			SourceType:  models.SourceTypeYoutube,
			URL:         "https://www.youtube.com/watch?v=" + video.Id,
			Authors:     []string{video.Snippet.ChannelTitle},
			Language:    &video.Snippet.DefaultLanguage,
			Categories:  video.Snippet.Tags,
			FeedTitle:   feed.GetTitle(),
		}

		// Set the published date. If no published date in the source, set it to unix epoch.
		if pubDate, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt); err != nil {
			feed.Published = pubDate.UTC()
		} else {
			feed.Published = models.UnixEpoch
		}

		// Make some assumptions on video size, API does not expose these directly.
		var width, height int
		switch video.ContentDetails.Definition {
		case "hd":
			width = 1920
			height = 1080
		default:
			width = 640
			height = 480
		}

		// Add youtube extension data if found.
		item.ExtensionType = new(models.ItemExtensionTypeYoutube)
		item.ExtensionData = &models.Item_ExtensionData{}
		item.ExtensionData.FromItemExtensionYoutube(models.ItemExtensionYoutube{ //nolint:errcheck // should never fail.
			VideoId: video.Id,
			Width:   &width,
			Height:  &height,
		})

		// Set the image.
		if img := getBestVideoThumbnail(video); img != nil {
			item.Image = img
		}

		items = append(items, item)
	}

	return items
}

func getBestVideoThumbnail(video *youtube.Video) *models.RemoteImage {
	switch details := video.Snippet.Thumbnails; {
	case details.Maxres != nil:
		return models.NewRemoteImage(details.Maxres.Url, video.Snippet.Title)
	case details.High != nil:
		return models.NewRemoteImage(details.High.Url, video.Snippet.Title)
	case details.Medium != nil:
		return models.NewRemoteImage(details.Medium.Url, video.Snippet.Title)
	case details.Standard != nil:
		return models.NewRemoteImage(details.Standard.Url, video.Snippet.Title)
	case details.Default != nil:
		return models.NewRemoteImage(details.Default.Url, video.Snippet.Title)
	}
	return nil
}

func getUploadsPlaylistID(channelID string) (string, error) {
	client, err := initClient()
	if err != nil {
		return "", fmt.Errorf("init client: %w", err)
	}

	resp, err := client.Channels.List([]string{"contentDetails"}).
		Id(channelID).
		Do()
	if err != nil {
		return "", fmt.Errorf("list channel: %w", err)
	}
	if len(resp.Items) == 0 {
		return "", errors.New("channel not found")
	}
	return resp.Items[0].ContentDetails.RelatedPlaylists.Uploads, nil
}

func getVideosSince(uploadsPlaylistID string, since time.Time) ([]string, error) {
	client, err := initClient()
	if err != nil {
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
			return nil, fmt.Errorf("list playlist items: %w", err)
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

// getVideoDetails fetches the details of the given videos.
func getVideoDetails(videoIDs []string) ([]*youtube.Video, error) {
	client, err := initClient()
	if err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	var videos []*youtube.Video

	// videos.list accepts up to 50 IDs per call
	for i := 0; i < len(videoIDs); i += 50 {
		end := min(i+50, len(videoIDs))
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

// getChannelDetails fetches the details for the given channels.
func getChannelDetails(channelIDs []string) ([]*youtube.Channel, error) {
	client, err := initClient()
	if err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	var channels []*youtube.Channel

	// videos.list accepts up to 50 IDs per call
	for i := 0; i < len(channelIDs); i += 50 {
		end := min(i+50, len(channelIDs))
		batch := channelIDs[i:end]

		resp, err := client.Channels.List([]string{"snippet", "brandingSettings"}).
			Id(batch...).
			Do()
		if err != nil {
			return nil, fmt.Errorf("list videos: %w", err)
		}
		channels = append(channels, resp.Items...)
	}

	return channels, nil
}

// getPlaylistDetails fetches the details for the given playlists.
func getPlaylistDetails(playlistIDs []string) ([]*youtube.Playlist, error) {
	client, err := initClient()
	if err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	var playlists []*youtube.Playlist

	// videos.list accepts up to 50 IDs per call
	for i := 0; i < len(playlistIDs); i += 50 {
		end := min(i+50, len(playlistIDs))
		batch := playlistIDs[i:end]

		resp, err := client.Playlists.List([]string{"snippet"}).
			Id(batch...).
			Do()
		if err != nil {
			return nil, fmt.Errorf("list videos: %w", err)
		}
		playlists = append(playlists, resp.Items...)
	}

	return playlists, nil
}
