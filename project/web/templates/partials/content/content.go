// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package content

import (
	"time"

	"github.com/dustin/go-humanize"
)

const (
	ContentTarget = "content-main"
)

func RelativeTime(timestamp time.Time) string {
	return humanize.Time(timestamp)
}
