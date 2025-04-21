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

package elastic

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"

	"github.com/joshuar/go-feed-me/components/config"
)

const (
	maxIdleConnsPerHost = 10
	connTimeout         = time.Second
)

var defaultTransportConfig = &http.Transport{
	MaxIdleConnsPerHost:   maxIdleConnsPerHost,
	ResponseHeaderTimeout: connTimeout,
	DialContext:           (&net.Dialer{Timeout: connTimeout}).DialContext,
	TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
	},
}

func Connect(ctx context.Context) (*API, error) {
	clientConfig, err := loadConfigOnce(ctx, config.Environment())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}

	esclient, err := elasticsearch.NewTypedClient(*clientConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}

	return &API{API: esclient.API}, nil
}
