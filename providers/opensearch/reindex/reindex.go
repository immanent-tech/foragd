// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package reindex

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch"
)

type Response struct {
	*opensearchapi.ReindexResp
}

func Reindex(ctx context.Context, src *Source, dest *Destination, options ...Option) (*Response, error) {
	client, err := opensearch.GetClient()
	if err != nil {
		return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("connect: %w", err))
	}

	req := &opensearchapi.ReindexReq{
		Body: &opensearchapi.ReindexBody{
			Source: *src.ReindexSource,
			Dest:   *dest.ReindexDestination,
		},
		Params: &opensearchapi.ReindexParams{},
	}

	for option := range slices.Values(options) {
		option(req)
	}

	resp, err := client.Reindex(ctx, req)
	if err != nil {
		return nil, models.NewAPIError(resp.Inspect().Response.StatusCode, fmt.Errorf("reindex: %w", err))
	}

	return &Response{ReindexResp: resp}, nil
}

type Option func(*opensearchapi.ReindexReq)

func WithAbortOnConflict() Option {
	return func(r *opensearchapi.ReindexReq) {
		r.Body.Conflicts = new(opensearchapi.ConflictsAbort)
	}
}

func WithProceedOnConflict() Option {
	return func(r *opensearchapi.ReindexReq) {
		r.Body.Conflicts = new(opensearchapi.ConflictsProceed)
	}
}

func WithRequestsPerSecond(rps float64) Option {
	return func(r *opensearchapi.ReindexReq) {
		r.Params.RequestsPerSecond = &rps
	}
}

func WaitForCompletion(value bool) Option {
	return func(r *opensearchapi.ReindexReq) {
		r.Params.WaitForCompletion = &value
	}
}

// Source represents a reindex source.
type Source struct {
	*opensearchapi.ReindexSource
}

// NewSource creates a new source for a reindex operation.
func NewSource(options ...SourceOption) *Source {
	src := &Source{
		ReindexSource: &opensearchapi.ReindexSource{},
	}
	for option := range slices.Values(options) {
		option(src)
	}
	return src
}

// SourceOption is a functional option applied to a reindex source.
type SourceOption func(*Source)

// From defines the indices that the reindex operation will read from.
func From(from ...string) SourceOption {
	return func(s *Source) {
		s.Index = from
	}
}

// Destination represents a reindex destination.
type Destination struct {
	*opensearchapi.ReindexDestination
}

// NewDestination creates a new destination for a reindex operation.
func NewDestination(options ...DestinationOption) *Destination {
	dest := &Destination{
		ReindexDestination: &opensearchapi.ReindexDestination{},
	}
	for option := range slices.Values(options) {
		option(dest)
	}
	return dest
}

// DestinationOption is a functional option applied to a reindex destination.
type DestinationOption func(*Destination)

// To defines the index that the reindex operation will index to.
func To(to string) DestinationOption {
	return func(s *Destination) {
		s.Index = to
	}
}
