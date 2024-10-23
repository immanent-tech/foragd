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

	"github.com/elastic/go-elasticsearch/v8/typedapi/cluster/putcomponenttemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/putindextemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/platform/elastic/schema"
)

var (
	ErrPutComponentTemplate = errors.New("put component template failed")
	ErrPutIndexTemplate     = errors.New("put index template failed")
)

func (c *Client) PutIndexTemplate(ctx context.Context, template schema.IndexTemplate) error {
	components := make([]string, 0, len(template.Components))

	for _, component := range template.Components {
		err := c.PutComponentTemplate(ctx, component)
		if err != nil {
			return errors.Join(ErrPutIndexTemplate, ErrPutComponentTemplate, err)
		}

		components = append(components, component.Name)
	}

	req := &putindextemplate.Request{
		ComposedOf:    components,
		IndexPatterns: template.IndexPatterns,
		DataStream:    types.NewDataStreamVisibility(),
		Priority:      &template.Priority,
	}
	if template.Meta != nil {
		req.Meta_ = *template.Meta
	}

	resp, err := c.API.Indices.PutIndexTemplate(template.Name).Request(req).Do(ctx)
	c.logger.Log(ctx, LevelTrace, "put index template", slog.Any("response", resp))

	if err != nil {
		return errors.Join(ErrPutIndexTemplate, err)
	}

	return nil
}

func (c *Client) PutComponentTemplate(ctx context.Context, template schema.ComponentTemplate) error {
	req := &putcomponenttemplate.Request{
		Template: template.Template,
	}

	resp, err := c.API.Cluster.PutComponentTemplate(template.Name).Request(req).Do(ctx)
	c.logger.Log(ctx, LevelTrace, "put component response", slog.Any("response", resp))

	if err != nil {
		return fmt.Errorf("%w: %w", ErrPutPolicy, err)
	}

	return nil
}
