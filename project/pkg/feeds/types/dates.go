// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import (
	"encoding/json"
	"errors"
	"slices"
	"time"
)

var DateTimeFormats = []string{time.RFC1123Z, time.RFC1123, time.RFC3339}

type DateTime struct {
	time.Time
}

func (d DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Time.Format(DateTimeFormats[0]))
}

func (d *DateTime) UnmarshalJSON(data []byte) error {
	var dateStr string
	err := json.Unmarshal(data, &dateStr)
	if err != nil {
		return err
	}
	parsed, err := tryFormats(dateStr)
	if err != nil {
		return err
	}
	d.Time = parsed
	return nil
}

func (d DateTime) String() string {
	return d.Time.Format(DateTimeFormats[0])
}

func (d *DateTime) UnmarshalText(data []byte) error {
	parsed, err := tryFormats(string(data))
	if err != nil {
		return err
	}
	d.Time = parsed
	return nil
}

func tryFormats(data string) (time.Time, error) {
	var parsed time.Time
	for format := range slices.Values(DateTimeFormats) {
		value, err := time.Parse(format, data)
		if err != nil {
			continue
		}
		parsed = value
	}
	if parsed.IsZero() {
		return parsed, errors.New("unsupported format")
	}
	return parsed, nil
}
