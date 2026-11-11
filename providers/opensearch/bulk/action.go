/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package bulk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

const (
	VersionTypeInternal    VersionType = "internal"
	VersionTypeExternal    VersionType = "external"
	VersionTypeExternalGte VersionType = "external_gte"
)

type VersionType string

type ActionMetadata struct {
	// Index is the name of the index this action will target. Required if not specified in the request path.
	Index string `json:"_index,omitzero"`
	// ID is the document ID. Optional. If not provided, OpenSearch generates an ID automatically.
	ID string `json:"_id,omitzero"`
	// RequireAlias when true, requires the destination to be an index alias. Default is false.
	RequireAlias *bool `json:"_require_alias,omitempty"`
	// Routing is the custom routing value for the document operation.
	Routing string `json:"_routing,omitzero"`
	// Version is the explicit version number for the document. Used for optimistic concurrency control.
	Version *uint64 `json:"version,omitempty"`
	// VersionType is the version type: internal, external, external_gte. Default is internal.
	VersionType VersionType `json:"version_type,omitzero"`
	// IfSeqNo performs the operation only if the document has this sequence number. Used for optimistic concurrency
	// control.
	IfSeqNo *uint64 `json:"if_seq_no,omitempty"`
	// IfPrimaryTerm performs the operation only if the document has this primary term. Used for optimistic concurrency control.
	IfPrimaryTerm *uint64 `json:"if_primary_term,omitempty"`
}

const (
	ActionCreate ActionType = "create"
)

type ActionType string

type Action struct {
	Type          ActionType
	Metadata      ActionMetadata
	Doc           any
	meta          bytes.Buffer  // encoded metadata
	body          io.ReadSeeker // encoded document body (if any)
	payloadLength int           // Item payload total length metadata+newline+body length

	// OnSuccess func(context.Context, BulkIndexerItem, BulkIndexerResponseItem)        // Per item
	// OnFailure func(context.Context, BulkIndexerItem, BulkIndexerResponseItem, error) // Per item

	flushDone chan struct{} // non-nil marks a flush sentinel; worker closes it when flush completes
}

func NewAction(action ActionType, options ...ActionOption) *Action {
	a := &Action{
		Type:     action,
		Metadata: ActionMetadata{},
	}
	for option := range slices.Values(options) {
		option(a)
	}
	return a
}

// encode encodes the action metadata and document.
func (a Action) encode() {
	metaEnc := json.NewEncoder(&a.meta)
	switch a.Type {
	case ActionCreate:
		metaEnc.Encode(map[string]any{"create": a.Metadata})
	}
	if a.Doc != nil {
		var data bytes.Buffer
		bodyEnc := json.NewEncoder(&data)
		bodyEnc.Encode(a.Doc)
		data.WriteByte('\n')
		a.body = bytes.NewReader(data.Bytes())
	}
}

func (a Action) WriteTo(w io.Writer) (int64, error) {
	if a.meta.Len() == 0 {
		a.encode()
	}
	var count int64
	n, err := w.Write(a.meta.Bytes())
	if err != nil {
		return count, fmt.Errorf("write meta: %w", err)
	}
	count += int64(n)
	if a.body != nil {
		n, err := io.Copy(w, a.body)
		if err != nil {
			return count, fmt.Errorf("write body: %w", err)
		}
		count += n
	}
	return count, nil
}

func (a Action) computeLength() error {
	if a.meta.Len() == 0 {
		a.encode()
	}
	if a.body != nil {
		n, err := a.body.Seek(0, io.SeekEnd)
		if err != nil {
			return err
		}
		a.payloadLength += int(n)
		_, err = a.body.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
	}
	a.payloadLength += len(a.meta.Bytes())
	// Add one byte to account for newline at the end of payload.
	a.payloadLength++

	return nil
}

type ActionOption func(*Action)

func WithDoc(doc any) ActionOption {
	return func(ba *Action) {
		ba.Doc = doc
	}
}

func ToIndex(index string) ActionOption {
	return func(ba *Action) {
		ba.Metadata.Index = index
	}
}
