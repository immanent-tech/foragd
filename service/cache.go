// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"time"

	"github.com/maypok86/otter/v2"

	"github.com/immanent-tech/foragd/models"
)

var subscriptionsCache = otter.Must(&otter.Options[string, models.Subscriptions]{
	MaximumSize: 1000,
	ExpiryCalculator: otter.ExpiryAccessing[string, models.Subscriptions](
		60 * time.Second,
	), // Reset timer on reads/writes
	// RefreshCalculator: otter.RefreshWriting[string, models.Subscriptions](
	// 	500 * time.Millisecond,
	// ), // Refresh after writes
	// StatsRecorder: counter, // Attach stats collector
})

var userCache = otter.Must(&otter.Options[string, models.User]{
	MaximumSize: 10_000,
	ExpiryCalculator: otter.ExpiryAccessing[string, models.User](
		60 * time.Second,
	), // Reset timer on reads/writes
	// RefreshCalculator: otter.RefreshWriting[string, models.Subscriptions](
	// 	500 * time.Millisecond,
	// ), // Refresh after writes
	// StatsRecorder: counter, // Attach stats collector
})
