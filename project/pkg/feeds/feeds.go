// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package feeds

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"slices"
	"strings"

	"github.com/go-resty/resty/v2"

	"github.com/joshuar/go-feed-me/pkg/feeds/atom"
	"github.com/joshuar/go-feed-me/pkg/feeds/rss"
	"github.com/joshuar/go-feed-me/pkg/feeds/types"
	"github.com/joshuar/go-feed-me/pkg/feeds/validation"
)

var ErrParseFeed = errors.New("unable to parse feed")

// NewFeedFromBytes will create a new Feed of the given type from the given byte array.
func NewFeedFromBytes[T any](data []byte) (*types.Feed, error) {
	var (
		original T
		feed     *types.Feed
	)
	original, err := types.Decode[T]("", data)
	// err := xml.Unmarshal(data, &original)
	if err != nil {
		return nil, errors.Join(ErrParseFeed, err)
	}
	source, ok := any(original).(types.FeedSource)
	if !ok {
		return nil, fmt.Errorf("%w: data is not a valid feed type %T", ErrParseFeed, original)
	}
	feed = &types.Feed{
		FeedSource: source,
	}
	if err := validation.ValidateStruct(feed); err != nil {
		return nil, fmt.Errorf("%w: feed is not valid: %w", ErrParseFeed, err)
	}

	return feed, nil
}

// NewFeedFromSource will create a new Feed from the given source that satisfies the FeedSource interface. This can be
// used to create a Feed from an existing rss.RSS or atom.Feed object.
func NewFeedFromSource[T types.FeedSource](source T) *types.Feed {
	return &types.Feed{
		FeedSource: source,
	}
}

func NewFeedFromURL(ctx context.Context, url string) (*types.Feed, error) {
	client := resty.New()
	// Set the mimetypes we accept. Who knows if this helps but at least we are honest to the server with what mimetypes
	// we want.
	client.SetHeader("Accept", strings.Join(slices.Concat(types.MimeTypesAtom, types.MimeTypesRSS), ","))
	// Get the feed data.
	resp, err := client.R().SetContext(ctx).Get(url)
	if err != nil {
		return nil, errors.Join(ErrParseFeed, err)
	}
	// Retrieve the contentType header so we know what format we are dealing with.
	contentType := resp.Header().Get("Content-Type")
	if contentType == "" {
		return nil, fmt.Errorf("%w: unable to determine feed type", ErrParseFeed)
	}

	var feed *types.Feed
	switch {
	case isRSS(contentType):
		feed, err = NewFeedFromBytes[*rss.RSS](resp.Body())
	case isAtom(contentType):
		feed, err = NewFeedFromBytes[*atom.Feed](resp.Body())
	case isIndeterminate(contentType):
		// Try RSS first...
		feed, err = NewFeedFromBytes[*rss.RSS](resp.Body())
		// Try Atom if that failed...
		if err != nil {
			feed, err = NewFeedFromBytes[*atom.Feed](resp.Body())
		}
	default:
		return nil, fmt.Errorf("%w: unsupported format %s", ErrParseFeed, contentType)
	}
	// (╯°益°)╯彡┻━┻
	if err != nil {
		return nil, err
	}

	// If the source URL is not set, set it.
	if feed.GetSourceURL() == "" {
		feed.SetSourceURL(url)
	}

	return feed, nil
}

func isRSS(contentType string) bool {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return slices.Contains(types.MimeTypesRSS, mediatype)
}

func isAtom(contentType string) bool {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return slices.Contains(types.MimeTypesAtom, mediatype)
}

func isIndeterminate(contentType string) bool {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return slices.Contains(types.MimeTypesIndeterminate, mediatype)
}
