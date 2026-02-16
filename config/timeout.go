// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package config

import (
	"fmt"
	"time"
)

type Timeout string

func (t Timeout) Validate() error {
	if _, err := time.ParseDuration(string(t)); err != nil {
		return fmt.Errorf("parse timeout: %w", err)
	}
	return nil
}

func (t Timeout) Duration() time.Duration {
	duration, err := time.ParseDuration(string(t))
	if err != nil {
		return time.Minute
	}
	return duration
}
