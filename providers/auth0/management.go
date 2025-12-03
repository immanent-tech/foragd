// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"fmt"
	"sync"

	"github.com/auth0/go-auth0/management"
)

// ManagementAPI represents the Auth0 management API backend connection.
type ManagementAPI struct {
	*management.Management
}

var mgmt *management.Management

// LoadManagementAPI loads a connection to the Auth0 management API.
var LoadManagementAPI = sync.OnceValue(func() error {
	var err error
	mgmt, err = management.New(
		cfg.Domain,
		management.WithClientCredentials(
			context.Background(),
			cfg.ClientID,
			cfg.ClientSecret,
		),
	)
	if err != nil {
		return fmt.Errorf("auth0: management api backend: %w", err)
	}
	return nil
})
