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
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/platform/id"
)

type FeedItem gofeed.Item

//revive:disable:unused-receiver
func (i *FeedItem) ID() string {
	feedID, err := id.NewID(id.Item)
	if err != nil {
		return ""
	}

	return feedID
}

func (i *FeedItem) IsNewer(since time.Time) bool {
	if i.UpdatedParsed != nil {
		return i.UpdatedParsed.After(since)
	}

	return i.PublishedParsed.After(since)
}
