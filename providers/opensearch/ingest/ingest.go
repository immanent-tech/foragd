/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package ingest

import (
	"context"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch"
)

// NewIngestPipeline configures an ingest pipeline in the cluster.
func NewIngestPipeline(
	ctx context.Context,
	client *opensearchapi.IngestClient,
	req opensearchapi.IngestPutPipelineReq,
) error {
	if req.Header == nil {
		req.Header = opensearch.NewRequestHeaders(ctx)
	}
	resp, err := client.PutPipeline(ctx, req)
	if err != nil {
		return opensearch.HandleError("put ingest pipeline", resp, err)
	}
	return nil
}
