// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package rss

import (
	"encoding/json"
)

func (r *RSS) Metadata() (*ChannelMetadata, error) {
	data, err := json.Marshal(r.Channel)
	if err != nil {
		return nil, err
	}
	var metadata ChannelMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}
