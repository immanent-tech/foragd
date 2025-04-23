// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package feeds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
	"golang.org/x/net/html"
	htmlatom "golang.org/x/net/html/atom"

	"github.com/joshuar/go-feed-me/models/feeds/atom"
	"github.com/joshuar/go-feed-me/models/feeds/jsonfeed"
	"github.com/joshuar/go-feed-me/models/feeds/rss"
	"github.com/joshuar/go-feed-me/models/feeds/types"
	"github.com/joshuar/go-feed-me/models/feeds/validation"
)

var (
	// ErrParseFeed indicates an error parsing the feed content.
	ErrParseFeed = errors.New("unable to parse")
	// ErrUnmarshal indicates an error unmarshaling the feed from its native format.
	ErrUnmarshal = errors.New("unable to unmarshal")
	// ErrUnsupportedFormat indicates that feed format is not known and cannot be parsed.
	ErrUnsupportedFormat = errors.New("unsupported feed format")
)

// FeedResult is returned when calling NewFeedsFromURLs and contains the results for parsing an individual URL. It
// will contain the original URL and either a new Feed or a non-nil error.
type FeedResult struct {
	URL  string
	Feed *Feed
	Err  error
}

// FeedItemsResult is returned when calling NewItemsFromURLs and contains the results for parsing an individual URL. It
// will contain the original URL, any items parsed and a non-nil error if a problem occurred.
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
		err      error
	)
	if _, ok := any(original).(*jsonfeed.Feed); ok {
		// If the original is JSONFeed, unmarshal as JSON.
		err = json.Unmarshal(data, &original)
	} else {
		// Otherwise, unmarshal as XML.
		original, err = Decode[T]("", data)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParseFeed, err)
	}
	source, ok := any(original).(types.FeedSource)
	if !ok {
		return nil, fmt.Errorf("%w: data is not a valid feed type %T", ErrParseFeed, original)
	}
	feed = &Feed{
		FeedSource: source,
	}
	feed.SourceType = parseSource(original)
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
	feed.SourceType = parseSource(source)
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

// parseFeedURL attempts to parse the given URL as a feed source.
func parseFeedURL(ctx context.Context, client *resty.Client, url string) FeedResult {
	// Get the feed data.
	resp, err := client.R().SetContext(ctx).Get(url)
	if err != nil {
		return FeedResult{URL: url, Err: fmt.Errorf("%w: could not access feed URL: %w", ErrParseFeed, err)}
	}
	// Retrieve the content header so we know what format we are dealing with.
	content := resp.Header().Get("Content-Type")
	if content == "" {
		return FeedResult{URL: url, Err: fmt.Errorf("%w: unable to determine feed type", ErrParseFeed)}
	}
	// Try to parse the response body as a valid feed type.
	var feed *Feed
	switch {
	case isMimeType(content, types.MimeTypesRSS):
		// RSS.
		feed, err = NewFeedFromBytes[*rss.RSS](resp.Body())
	case isMimeType(content, types.MimeTypesAtom):
		// Atom.
		feed, err = NewFeedFromBytes[*atom.Feed](resp.Body())
	case isMimeType(content, types.MimeTypesIndeterminate):
		// Likely a feed but mimetype is ambiguous.
		// Try RSS first...
		feed, err = NewFeedFromBytes[*rss.RSS](resp.Body())
		// Try Atom if that failed...
		if err != nil {
			feed, err = NewFeedFromBytes[*atom.Feed](resp.Body())
		}
	case isMimeType(content, types.MimeTypesJSONFeed):
		// JSONFeed
		feed, err = NewFeedFromBytes[*jsonfeed.Feed](resp.Body())
	case isMimeType(content, types.MimeTypesHTML):
		// URL points to a HTML page, not a feed source.
		// Try to find a feed link on the page and then parse that URL.
		if url, err := discoverFeedURL(resp.Body()); err == nil && url != "" {
			return parseFeedURL(ctx, client, url)
		}
		fallthrough
	default:
		// Cannot determine or unsupported content.
		err = fmt.Errorf("%w: %s", ErrUnsupportedFormat, content)
	}
	// (╯°益°)╯彡┻━┻
	if err != nil {
		return FeedResult{URL: url, Err: err}
	}

	// If the source URL is not set, set it.
	if feed.GetSourceURL() == "" {
		feed.SetSourceURL(url)
	}
	// If the feed source did not define an image, try to find and set an appropriate one.
	if feed.GetImage() == nil {
		image, _ := discoverFeedImage(ctx, client, url)
		if image != nil {
			feed.SetImage(image)
		}
	}

	return FeedResult{URL: url, Feed: feed}
}

// discoverFeedImage attempts to find a suitable image to use for a feed.
func discoverFeedImage(ctx context.Context, client *resty.Client, sourceURL string) (*types.Image, error) {
	// Parse the URL.
	site, err := url.Parse(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse URL: %w", err)
	}
	// Assume that the root website hosting the feed will likely have an appropriate icon. Wipe the path to get the root
	// website.
	site.Path = ""
	site.RawQuery = ""

	// Get the root website contents.
	resp, err := client.R().SetContext(ctx).Get(site.String())
	if err != nil {
		return nil, fmt.Errorf("could not access URL: %w", err)
	}
	// Discover any appropriate images.
	imagePath, err := discoverImages(resp.Body())
	if err != nil {
		return nil, fmt.Errorf("could not find appropriate image: %w", err)
	}
	// Generate image URL.
	imageURL, err := url.Parse(imagePath)
	if err != nil {
		return nil, fmt.Errorf("could not find appropriate image: %w", err)
	}
	if !imageURL.IsAbs() {
		imageURL = site.JoinPath(imagePath)
	}
	return &types.Image{Value: imageURL.String()}, nil
}

// discoverFeedURL attempts to find a feed URL within a HTML page.
func discoverImages(content []byte) (string, error) {
	page := html.NewTokenizer(bytes.NewReader(content))
	for {
		tt := page.Next()
		switch tt {
		case html.ErrorToken:
			return "", fmt.Errorf("unable to determine feed url: %w", page.Err())
		case html.StartTagToken:
			tkn := page.Token()
			if tkn.DataAtom != htmlatom.Link {
				continue
			}
			// "rel" attribute must contain "icon" string.
			if !slices.ContainsFunc(tkn.Attr, func(a html.Attribute) bool { return a.Key == "rel" && a.Val == "icon" }) {
				continue
			}
			// "href" attribute must contain "icon" string.
			if idx := slices.IndexFunc(tkn.Attr, func(a html.Attribute) bool {
				return a.Key == "href" && strings.Contains(a.Val, "icon")
			}); idx != -1 {
				return tkn.Attr[idx].Val, nil
			}
		}
	}
}

// discoverFeedURL attempts to find a feed URL within a HTML page.
func discoverFeedURL(content []byte) (string, error) {
	page := html.NewTokenizer(bytes.NewReader(content))
	for {
		tt := page.Next()
		switch tt {
		case html.ErrorToken:
			return "", fmt.Errorf("unable to determine feed url: %w", page.Err())
		case html.SelfClosingTagToken:
			tkn := page.Token()
			if tkn.DataAtom != htmlatom.Link {
				continue
			}
			// "rel" attribute must have a value of "alternate".
			if !slices.ContainsFunc(tkn.Attr, func(a html.Attribute) bool { return a.Key == "rel" && a.Val == "alternate" }) {
				continue
			}
			// "type" attribute must contain valid feed MIME type.
			if idx := slices.IndexFunc(tkn.Attr, func(a html.Attribute) bool {
				return a.Key == "type" && slices.Contains(types.MimeTypesFeed, a.Val)
			}); idx != -1 {
				return tkn.Attr[idx].Val, nil
			}
		}
	}
}

// isMimeType returns a boolean indicating whether the Content Type header string is included in the given mimeType
// slice.
func isMimeType(content string, mimeTypes []string) bool {
	mediatype, _, err := mime.ParseMediaType(content)
	if err != nil {
		return false
	}
	return slices.Contains(mimeTypes, mediatype)
}

// parseSource will attempt to determine the appropriate SourceType value from the given interface object.
func parseSource[T any](source T) SourceType {
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
	// Set the mimetypes we accept. Who knows if this helps but at least we are honest to the server with what mimetypes
	// we want.
	mimeTypes := types.MimeTypesFeed
	mimeTypes = append(mimeTypes, ";q=0.2,*/*", ";q=0.1")
	// Return a client.
	return resty.New().
		SetHeader("Accept", strings.Join(mimeTypes, ","))
}
