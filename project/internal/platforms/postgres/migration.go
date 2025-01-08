// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"context"
	"fmt"
)

func (c *Client) Migration(ctx context.Context) error {
	c.logger.Debug("Performing auto-migration...")

	if err := c.db.AutoMigrate(schemas[:]...); err != nil {
		return fmt.Errorf("%w: %w", ErrSetupFailed, err)
	}

	c.logger.Debug("Auto-migration complete!")

	return nil
}
