// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package feeds

import (
	"errors"
	"time"
)

var ErrInvalidFormat = errors.New("invalid data format")

type Metadata interface{}

type Item interface{}

type Feed interface {
	GetTitle() string
	GetGenerator() string
	GetHomepage() string
	GetSourceURL() string
	GetCopyright() string
	GetLastUpdated() time.Time
	// Items() []Item
}

type Object struct {
	Feed
}

// // FromAtomFeed overwrites any union data inside the Feed as the provided AtomFeed.
// func (o *Object) FromRSSFeed(v *rss.RSS) error {
// 	o.Feed = v

// }

// // FromAtomFeed overwrites any union data inside the Feed as the provided AtomFeed.
// func (s *Feed) FromAtomFeed(v *AtomFeed) error {
// 	b, err := types.Encode(v)
// 	s.data = b
// 	s.format = Atom
// 	return err
// }

// // AsRSSFeed returns the union data inside the Feed as a RSSFeed.
// func (s *Feed) AsRSSFeed() (*RSSFeed, error) {
// 	return types.Decode[*RSSFeed](s.data)
// }

// // AsRSSFeed returns the union data inside the Feed as a RSSFeed.
// func (s *Feed) AsAtomFeed() (*AtomFeed, error) {
// 	if s.format != RSS {
// 		return nil, fmt.Errorf("cannot decode as Atom feed: %w", ErrInvalidFormat)
// 	}
// 	return types.Decode[*AtomFeed](s.data)
// }
