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
	"errors"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/ingest/putpipeline"
)

var ErrPutIngestPipeline = errors.New("put ingest pipeline failed")

func (c *Client) PutIngestPipeline(ctx context.Context, name string, pipeline putpipeline.Request) error {
	resp, err := c.conn.API.Ingest.PutPipeline(name).Request(&pipeline).Do(ctx)
	c.logger.Log(ctx, LevelTrace, "put ingest pipeline response", slog.Any("response", resp))

	if err != nil {
		return fmt.Errorf("%w: %w", ErrPutIngestPipeline, err)
	}

	return nil
}
