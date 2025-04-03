// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package feeds

import (
	"context"
	"encoding/json"
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

const (
	TypeRSS  SourceType = "RSS"
	TypeAtom SourceType = "Atom"
)

// SourceType is a string constant that indicates the underlying source data type of a Feed/Item object. This is mainly
// used when unmarshaling from JSON where the JSON structure of the source types can be ambiguous.
type SourceType string

// Item represents a single item or entry (or article) in a feed.
type Item struct {
	types.ItemSource `json:"source"`
	SourceType       SourceType `json:"type"`
}

func (i *Item) UnmarshalJSON(v []byte) error {
	// Unmarshal the FeedSource based on the type field value.
	sourceType, source, err := sourceFromBytes(v)
	if err != nil {
		return err
	}
	switch sourceType {
	case TypeAtom:
		i.SourceType = TypeAtom
		i.ItemSource, err = unMarshalSource[*atom.Entry](source)
		if err != nil {
			return fmt.Errorf("%w: unable to unmarshal Atom data: %w", ErrUnmarshal, err)
		}
		return nil
	case TypeRSS:
		i.SourceType = TypeRSS
		i.ItemSource, err = unMarshalSource[*rss.Item](source)
		if err != nil {
			return fmt.Errorf("%w: unable to unmarshal RSS data: %w", ErrUnmarshal, err)
		}
		return nil
	}
	return fmt.Errorf("%w: unknown data type", ErrUnmarshal)
}

// Feed represents any feed type containing a number of items.
type Feed struct {
	types.FeedSource `json:"source"`
	SourceType       SourceType `json:"type"`
}

func (f *Feed) GetItems() []Item {
	var items []Item
	for item := range slices.Values(f.FeedSource.GetItems()) {
		items = append(items, Item{ItemSource: item, SourceType: f.SourceType})
	}
	return items
}

func (f *Feed) UnmarshalJSON(v []byte) error {
	// Unmarshal the FeedSource based on the type field value.
	sourceType, source, err := sourceFromBytes(v)
	if err != nil {
		return err
	}
	switch sourceType {
	case TypeAtom:
		f.SourceType = TypeAtom
		f.FeedSource, err = unMarshalSource[*atom.Feed](source)
		if err != nil {
			return fmt.Errorf("%w: unable to unmarshal Atom data: %w", ErrUnmarshal, err)
		}
		return nil
	case TypeRSS:
		f.SourceType = TypeRSS
		f.FeedSource, err = unMarshalSource[*rss.RSS](source)
		if err != nil {
			return fmt.Errorf("%w: unable to unmarshal RSS data: %w", ErrUnmarshal, err)
		}
		return nil
	}
	return fmt.Errorf("%w: unknown data type", ErrUnmarshal)
}

func sourceFromBytes(v []byte) (SourceType, json.RawMessage, error) {
	topLevel := make(map[string]json.RawMessage)
	err := json.Unmarshal(v, &topLevel)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrUnmarshal, err)
	}
	// Check for a type field and unmarshal its value if found.
	rawType, found := topLevel["type"]
	if !found {
		return "", nil, fmt.Errorf("%w: unknown data type", ErrUnmarshal)
	}
	var sourceType SourceType
	err = json.Unmarshal(rawType, &sourceType)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrUnmarshal, err)
	}
	return sourceType, topLevel["source"], nil
}

func unMarshalSource[T any](v json.RawMessage) (T, error) {
	var source T
	err := json.Unmarshal(v, &source)
	if err != nil {
		return source, err
	}
	return source, nil
}

// ParseURLResult is returned when calling NewFeedsFromURLs and contains the results for parsing an individual URL. It
// will contain the original URL and either a new Feed or a non-nil error.
type ParseURLResult struct {
	URL  string
	Feed *Feed
	Err  error
}

// NewFeedFromBytes will create a new Feed of the given type from the given byte array.
func NewFeedFromBytes[T any](data []byte) (*Feed, error) {
	var (
		original T
		feed     *Feed
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
func NewFeedsFromURLs(ctx context.Context, urls ...string) []ParseURLResult {
	client := resty.New()
	// Set the mimetypes we accept. Who knows if this helps but at least we are honest to the server with what mimetypes
	// we want.
	client.SetHeader("Accept", strings.Join(slices.Concat(types.MimeTypesAtom, types.MimeTypesRSS), ","))

	results := make([]ParseURLResult, 0, len(urls))
	workerCh := make(chan ParseURLResult)
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

func parseFeedURL(ctx context.Context, client *resty.Client, url string) ParseURLResult {
	// Get the feed data.
	resp, err := client.R().SetContext(ctx).Get(url)
	if err != nil {
		return ParseURLResult{URL: url, Err: fmt.Errorf("%w: could not access feed URL: %w", ErrParseFeed, err)}
	}
	// Retrieve the contentType header so we know what format we are dealing with.
	contentType := resp.Header().Get("Content-Type")
	if contentType == "" {
		return ParseURLResult{URL: url, Err: fmt.Errorf("%w: unable to determine feed type", ErrParseFeed)}
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
		return ParseURLResult{URL: url, Err: err}
	}

	// If the source URL is not set, set it.
	if feed.GetSourceURL() == "" {
		feed.SetSourceURL(url)
	}

	return ParseURLResult{URL: url, Feed: feed}
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
