// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"

	"github.com/elastic/go-elasticsearch/v8/typedapi/cluster/putcomponenttemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ilm/putlifecycle"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/putindextemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ingest/putpipeline"
)

var ErrPutILMPolicyFailed = errors.New("create ILM policy failed")

func (c *Client) PutILM(ctx context.Context, name string, policy *putlifecycle.Request) error {
	_, err := c.API.Ilm.PutLifecycle(name).Request(policy).Do(ctx)
	if err != nil {
		return errors.Join(ErrPutILMPolicyFailed, err)
	}

	return nil
}

func (c *Client) PutComponentTemplate(ctx context.Context, name string, template *putcomponenttemplate.Request) error {
	_, err := c.API.Cluster.PutComponentTemplate(name).Request(template).Do(ctx)
	if err != nil {
		return errors.Join(ErrPutILMPolicyFailed, err)
	}

	return nil
}

func (c *Client) PutIndexTemplate(ctx context.Context, name string, template *putindextemplate.Request) error {
	_, err := c.API.Indices.PutIndexTemplate(name).Request(template).Do(ctx)
	if err != nil {
		return errors.Join(ErrPutILMPolicyFailed, err)
	}

	return nil
}

func (c *Client) PutIngestPipeline(ctx context.Context, name string, pipeline *putpipeline.Request) error {
	_, err := c.API.Ingest.PutPipeline(name).Request(pipeline).Do(ctx)
	if err != nil {
		return errors.Join(ErrPutILMPolicyFailed, err)
	}

	return nil
}
