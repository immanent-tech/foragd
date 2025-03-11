// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package watcher

import (
	"github.com/joshuar/go-feed-me/internal/validation"
)

// CronTrigger sets the watch trigger to the specified cron pattern. If the
// pattern is invalid, it will default to a schedule of "every minute".
func CronTrigger(cron string) *string {
	valid, err := validation.ValidateVariable(cron, "cron")
	if !valid || err != nil {
		// Default to run every minute if given schedule is not valid or
		// cannot be parsed.
		cron = "* * * * * *"
	}

	return &cron
}
