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

package postgres

import (
	"fmt"
	"time"

	"github.com/joshuar/go-feed-me/platform/id"
)

func (c *Client) getFeedByURL(url string) (*Feed, error) {
	var feed Feed
	if err := c.db.First(&feed, "url = ?", url).Error; err != nil {
		return nil, err
	}

	return &feed, nil
}

func (c *Client) GetUpdatedFeedURLs(since time.Time) ([]string, error) {
	var feeds []Feed

	if err := c.db.Where("updated_at > ?", since).Find(&feeds).Error; err != nil {
		return nil, fmt.Errorf("could not retrieve updated feed list: %w", err)
	}

	urls := make([]string, len(feeds))

	for i, feed := range feeds {
		urls[i] = feed.URL
	}

	return urls, nil
}

func newFeedRecord(url string) (*Feed, error) {
	feedID, err := id.NewID(id.Feed)
	if err != nil {
		return nil, fmt.Errorf("cannot create feed: %w", err)
	}

	return &Feed{ID: feedID, URL: url}, nil
}
