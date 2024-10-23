// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package feeds

import (
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

type Feed struct {
	*gofeed.Feed
}

func (f *Feed) GetItemsSince(since time.Time) []FeedItem {
	var items []FeedItem

	for _, i := range f.Items {
		item := FeedItem(*i)
		if item.IsNewer(since) {
			items = append(items, item)
		}
	}

	return items
}

func FetchFeed(url string) (*Feed, error) {
	fp := gofeed.NewParser()

	feed, err := fp.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("cannot parse feed %s: %w", url, err)
	}

	return &Feed{
		Feed: feed,
	}, nil
}
