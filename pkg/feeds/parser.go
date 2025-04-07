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
	"sync"

	"github.com/go-resty/resty/v2"

	"github.com/joshuar/go-feed-me/pkg/feeds/atom"
	"github.com/joshuar/go-feed-me/pkg/feeds/rss"
	"github.com/joshuar/go-feed-me/pkg/feeds/types"
	"github.com/joshuar/go-feed-me/pkg/feeds/validation"
)

var (
	ErrParseFeed = errors.New("unable to parse")
	ErrUnmarshal = errors.New("unable to unmarshal")
)

// FeedResult is returned when calling NewFeedsFromURLs and contains the results for parsing an individual URL. It
// will contain the original URL and either a new Feed or a non-nil error.
type FeedResult struct {
	URL  string
	Feed *Feed
	Err  error
}

type FeedItemsResult struct {
	URL   string
	Items []Item
	Err   error
}

// ItemsResult is returned when calling NewItemsFromURLs and contains the results for parsing an individual Feed URL. It
// will contain the original URL and either a slice of Items or a non-nil error.
type ItemsResult []FeedItemsResult

// NewFeedFromBytes will create a new Feed of the given type from the given byte array.
func NewFeedFromBytes[T any](data []byte) (*Feed, error) {
	var (
		original T
		feed     *Feed
	)
	original, err := Decode[T]("", data)
	// err := xml.Unmarshal(data, &original)
	if err != nil {
		return nil, errors.Join(ErrParseFeed, err)
	}
	source, ok := any(original).(types.FeedSource)
	if !ok {
		return nil, fmt.Errorf("%w: data is not a valid feed type %T", ErrParseFeed, original)
	}
	feed = &Feed{
		FeedSource: source,
	}
	feed.SourceType = determineSourceType(original)
	if err := validation.ValidateStruct(feed); err != nil {
		return nil, fmt.Errorf("%w: feed is not valid: %w", ErrParseFeed, err)
	}

	return feed, nil
}

// NewFeedFromSource will create a new Feed from the given source that satisfies the FeedSource interface. This can be
// used to create a Feed from an existing rss.RSS or atom.Feed object.
func NewFeedFromSource[T types.FeedSource](source T) *Feed {
	feed := &Feed{
		FeedSource: source,
	}
	feed.SourceType = determineSourceType(source)
	return feed
}

// NewFeedsFromURLs will attempt to create new Feed objects from the given list of URLs. It returns a slice containing:
// the URL, any Feed object that was created, else, an non-nil error explaining the problem creating the Feed.
func NewFeedsFromURLs(ctx context.Context, urls ...string) []FeedResult {
	client := newWebClient()

	results := make([]FeedResult, 0, len(urls))
	workerCh := make(chan FeedResult)
	var wg sync.WaitGroup

	go func() {
		defer close(workerCh)
		for url := range slices.Values(urls) {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				workerCh <- parseFeedURL(ctx, client, url)
			}(url)
		}
		wg.Wait()
	}()
	// Gather results.
	for result := range workerCh {
		results = append(results, result)
	}

	return results
}

// NewItemsFromURLs will attempt to create new Item objects from the given list of Feed URLs. It returns a slice
// containing: the Feed URL, a slice of Items for that Feed URL, else, an non-nil error explaining the problem fetching
// Items.
func NewItemsFromURLs(ctx context.Context, urls ...string) ItemsResult {
	client := newWebClient()
	results := make(ItemsResult, 0, len(urls))
	workerCh := make(chan FeedItemsResult)
	var wg sync.WaitGroup

	go func() {
		defer close(workerCh)
		for url := range slices.Values(urls) {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				result := parseFeedURL(ctx, client, url)
				if result.Err != nil {
					workerCh <- FeedItemsResult{
						URL: url,
						Err: result.Err,
					}
				} else {
					workerCh <- FeedItemsResult{
						URL:   url,
						Items: result.Feed.GetItems(),
					}
				}
			}(url)
		}
		wg.Wait()
	}()
	// Gather results.
	for result := range workerCh {
		results = append(results, result)
	}

	return results
}

func parseFeedURL(ctx context.Context, client *resty.Client, url string) FeedResult {
	// Get the feed data.
	resp, err := client.R().SetContext(ctx).Get(url)
	if err != nil {
		return FeedResult{URL: url, Err: fmt.Errorf("%w: could not access feed URL: %w", ErrParseFeed, err)}
	}
	// Retrieve the contentType header so we know what format we are dealing with.
	contentType := resp.Header().Get("Content-Type")
	if contentType == "" {
		return FeedResult{URL: url, Err: fmt.Errorf("%w: unable to determine feed type", ErrParseFeed)}
	}

	var feed *Feed
	switch {
	case isRSS(contentType):
		feed, err = NewFeedFromBytes[*rss.RSS](resp.Body())
	case isAtom(contentType):
		feed, err = NewFeedFromBytes[*atom.Feed](resp.Body())
	case isAmbiguous(contentType):
		// Try RSS first...
		feed, err = NewFeedFromBytes[*rss.RSS](resp.Body())
		// Try Atom if that failed...
		if err != nil {
			feed, err = NewFeedFromBytes[*atom.Feed](resp.Body())
		}
	default:
		err = fmt.Errorf("unsupported file format %s", contentType)
	}
	// (╯°益°)╯彡┻━┻
	if err != nil {
		return FeedResult{URL: url, Err: err}
	}

	// If the source URL is not set, set it.
	if feed.GetSourceURL() == "" {
		feed.SetSourceURL(url)
	}

	return FeedResult{URL: url, Feed: feed}
}

// isRSS returns a boolean indicating whether the Content Type header indicates RSS.
func isRSS(contentType string) bool {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return slices.Contains(types.MimeTypesRSS, mediatype)
}

// isAtom returns a boolean indicating whether the Content Type header indicates Atom.
func isAtom(contentType string) bool {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return slices.Contains(types.MimeTypesAtom, mediatype)
}

// isAmbiguous will return true if the Content Type is ambiguous and the exact Feed type cannot be determined
// accurately. This will be the case if the Content Type is set to the generic application/xml type.
func isAmbiguous(contentType string) bool {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return slices.Contains(types.MimeTypesIndeterminate, mediatype)
}

// determineSourceType will attempt to determine the appropriate SourceType value from the given interface object.
func determineSourceType[T any](source T) SourceType {
	switch any(source).(type) {
	case *atom.Feed:
		return TypeAtom
	case *rss.RSS:
		return TypeRSS
	default:
		return ""
	}
}

func newWebClient() *resty.Client {
	client := resty.New()
	// Set the mimetypes we accept. Who knows if this helps but at least we are honest to the server with what mimetypes
	// we want.
	client.SetHeader("Accept", strings.Join(slices.Concat(types.MimeTypesAtom, types.MimeTypesRSS), ","))

	return client
}
