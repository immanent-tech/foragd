// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ErrInvalidDateTimeFormat indicates that the value of the datetime is not one of the defined DateTimeFormats. In most
// cases, this indicates the feed not using a valid datetime format according to its specification.
var ErrInvalidDateTimeFormat = errors.New("invalid datetime format")

// DateTimeFormats are the valid datetime formats across different feed specifications. A DateTime object will try to
// parse a given value as one of these formats.
var DateTimeFormats = []string{
	time.RFC1123Z,
	time.RFC1123,
	"Mon, 2 Jan 2006 15:04:05 -0700", // RFC1123Z without leading zero on day
	"Mon, 2 Jan 2006 15:04:05 MST",   // RFC1123 without leading zero on day
	"Mon, 02 Jan 2006 15:04 -0700",   // RFC1123Z without seconds
	"Mon, 02 Jan 2006 15:04 MST",     // RFC1123 without seconds
	"Mon, 2 Jan 2006 15:04 -0700",    // RFC1123Z without leading zero on day or seconds
	"Mon, 2 Jan 2006 15:04 MST",      // RFC1123 without leading zero on day or seconds
	time.RFC3339,
	time.DateTime,
}

// DateTime is a datetime value for a feed (or item) object, such as its published/updated date.
type DateTime struct {
	time.Time
}

func (d DateTime) MarshalJSON() ([]byte, error) {
	date, err := json.Marshal(d.Time.Format(DateTimeFormats[0]))
	if err != nil {
		return nil, errors.Join(ErrInvalidDateTimeFormat, err)
	}
	return date, nil
}

func (d *DateTime) UnmarshalJSON(data []byte) error {
	var dateStr string
	err := json.Unmarshal(data, &dateStr)
	if err != nil {
		return errors.Join(ErrInvalidDateTimeFormat, err)
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
		data = strings.TrimSpace(data)
		value, err := time.Parse(format, data)
		if err != nil {
			continue
		}
		parsed = value
	}
	if parsed.IsZero() {
		return parsed, fmt.Errorf("%w: got zero value", ErrInvalidDateTimeFormat)
	}
	return parsed, nil
}
