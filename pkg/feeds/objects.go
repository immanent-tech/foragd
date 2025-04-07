// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package feeds

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/joshuar/go-feed-me/pkg/feeds/atom"
	"github.com/joshuar/go-feed-me/pkg/feeds/rss"
	"github.com/joshuar/go-feed-me/pkg/feeds/types"
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
