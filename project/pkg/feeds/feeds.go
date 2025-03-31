// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package feeds

import (
	"errors"

	"github.com/joshuar/go-feed-me/pkg/feeds/types"
)

var ErrInvalidFormat = errors.New("invalid data format")

type NamespacedElement interface {
	GetNamespace() types.NameSpace
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
