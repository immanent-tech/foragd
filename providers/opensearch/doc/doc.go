/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package doc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

type Client interface {
	DeleteByQuery(ctx context.Context, req *opensearchapi.DeleteByQueryReq) (*opensearchapi.DeleteByQueryResp, error)
}

type Doc[T ~string] interface {
	GetID() T
}

// Create will create the given document, with given id, in the given index.
func Create[T ~string, O Doc[T]](ctx context.Context, client opensearchapi.DocumentClient, index string, doc O) error {
	bodyBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return models.NewAPIError(http.StatusInternalServerError, errors.New("get buffer failed"))
	}
	bodyBuf.Reset()
	defer bufPool.Put(bodyBuf)

	if err := json.NewEncoder(bodyBuf).Encode(doc); err != nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("encode doc: %w", err))
	}

	req := opensearchapi.CreateReq{
		Index:  index,
		ID:     string(doc.GetID()),
		Body:   bodyBuf,
		Header: opensearch.NewRequestHeaders(ctx),
	}

	resp, err := client.Create(ctx, req)
	if err != nil {
		return opensearch.HandleError("create doc", resp, err)
	}
	return nil
}

// Get will retrieve the document with the given ID from the given index.
func Get[T ~string, O Doc[T]](ctx context.Context, client opensearchapi.DocumentClient, index string, id T) (O, error) {
	var doc O
	req := opensearchapi.GetReq{
		Index:  index,
		ID:     string(id),
		Header: opensearch.NewRequestHeaders(ctx),
	}

	resp, err := client.Get(ctx, req)
	if !resp.Found {
		return doc, errors.Join(models.ErrNotFound, err)
	}
	if err != nil {
		return doc, opensearch.HandleError("get doc", resp, err)
	}

	doc, err = opensearch.ExtractSource[O](resp.Source)
	if err != nil {
		return doc, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("extract source: %w", err))
	}

	return doc, nil
}

// Update will apply the updates to the doc with the given id.
func Update[T ~string](
	ctx context.Context,
	client opensearchapi.DocumentClient,
	index string,
	id T,
	doc any,
	options ...UpdateOption,
) error {
	// Set any specified options for the request.
	body := &opensearchapi.UpdateBody{}
	for option := range slices.Values(options) {
		option(body)
	}

	// Encode the updates.
	bodyBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return models.NewAPIError(http.StatusInternalServerError, errors.New("get buffer failed"))
	}
	bodyBuf.Reset()
	defer bufPool.Put(bodyBuf)

	if err := json.NewEncoder(bodyBuf).Encode(doc); err != nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("encode updates: %w", err))
	}
	body.Doc = bodyBuf.Bytes()

	// Create the request.
	req := opensearchapi.UpdateReq{
		Index:  index,
		Header: opensearch.NewRequestHeaders(ctx),
		Body:   body,
		ID:     string(id),
	}

	// Perform the update.
	_, err := client.Update(ctx, req)
	if err != nil {
		return fmt.Errorf("update doc: %w", err)
	}

	return nil
}

// UpdateOption is a functional option to apply to an update request.
type UpdateOption func(*opensearchapi.UpdateBody)

// WithDocAsUpsert will add the doc as a new document if not found. Otherwise, the doc will be updated.
func WithDocAsUpsert(value bool) UpdateOption {
	return func(b *opensearchapi.UpdateBody) {
		b.DocAsUpsert = &value
	}
}

// Delete performs a document deletion request.
func Delete[T ~string](
	ctx context.Context,
	client opensearchapi.DocumentClient,
	index string,
	id T,
	options ...DeleteOption,
) error {
	req := opensearchapi.DeleteReq{
		Index:  index,
		ID:     string(id),
		Header: opensearch.NewRequestHeaders(ctx),
	}

	// Set any additional optional parameters.
	for option := range slices.Values(options) {
		option(req.Params)
	}

	resp, err := client.Delete(ctx, req)
	if err != nil {
		return opensearch.GenerateAPIError("delete", resp)
	}

	return nil
}

// DeleteOption is a functional option applied to a delete request.
type DeleteOption func(*opensearchapi.DeleteParams)

// WithDeleteSeqNo option ensures the deletion will only happen if the document has the given sequence number.
func WithDeleteSeqNo(num int) DeleteOption {
	return func(dp *opensearchapi.DeleteParams) {
		dp.IfSeqNo = &num
	}
}

// WithDeletePrimaryTerm option ensures the deletion will only happen if the document has the given primary term.
func WithDeletePrimaryTerm(term int) DeleteOption {
	return func(dp *opensearchapi.DeleteParams) {
		dp.IfPrimaryTerm = &term
	}
}

// DeleteByQuery performs a delete by query request on the given index with the given options.
func DeleteByQuery(ctx context.Context, client Client, index string, options ...DeleteByQueryOption) error {
	// Set any specified options for the request.
	body := &opensearchapi.DeleteByQueryBody{}
	for option := range slices.Values(options) {
		option(body)
	}

	req := &opensearchapi.DeleteByQueryReq{
		Indices: []string{index},
		Header:  opensearch.NewRequestHeaders(ctx),
		Body:    body,
	}

	resp, err := client.DeleteByQuery(ctx, req)
	if err != nil {
		return opensearch.GenerateAPIError("delete by query", resp)
	}

	return nil
}

type DeleteByQueryOption func(*opensearchapi.DeleteByQueryBody)

// WithDeleteByQuery option specifies the query used to select documents to delete.
func WithDeleteByQuery(options ...search.QueryOption) DeleteByQueryOption {
	return func(req *opensearchapi.DeleteByQueryBody) {
		query := &opensearchapi.CommonQueryDSLQueryContainer{}
		for option := range slices.Values(options) {
			option(query)
		}
		req.Query = query
	}
}

// WithDeleteMaxDocs options specifies the maximum number of documents to delete.
func WithDeleteMaxDocs(max int) DeleteByQueryOption {
	return func(req *opensearchapi.DeleteByQueryBody) {
		req.MaxDocs = &max
	}
}
