/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package bulk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchutil"
)

const (
	ActionCreate ActionType = "create"
	ActionIndex  ActionType = "index"
	ActionUpdate ActionType = "update"
)

// ActionType is the type of document action that will be performed.
type ActionType string

// Action is an individual bulk indexer action.
type Action struct {
	opensearchutil.BulkIndexerItem

	doc any
}

// NewAction creates a new bulk action that can be sent to a bulk indexer.
func NewAction(action ActionType, options ...ActionOption) Action {
	a := Action{
		Action: string(action),
	}
	for option := range slices.Values(options) {
		option(&a)
	}
	return a
}

func (a Action) encode() error {
	if a.doc != nil {
		data, err := json.Marshal(a.doc)
		if err != nil {
			return fmt.Errorf("marshal doc: %w", err)
		}
		a.Body = bytes.NewReader(data)
	}
	return nil
}

// ActionOption is a functional option applied to a bulk action.
type ActionOption func(*Action)

// WithDoc option defines the document for the action.
func WithDoc(doc any) ActionOption {
	return func(a *Action) {
		a.doc = doc
	}
}

// ToIndex option defines the index on which to perform the action.
func ToIndex(index string) ActionOption {
	return func(a *Action) {
		a.Index = index
	}
}
