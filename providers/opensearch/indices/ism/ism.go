/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package ism

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v5/plugins/ism"

	"github.com/immanent-tech/foragd/providers/opensearch"
)

// NewClient creates a new ISM client.
func NewClient(osc *opensearch.Client) *ism.Client {
	return ism.NewClient(osc.Client.Client)
}

// ConfigurePolicy performs all steps required to set up and configure an ISM policy. It will add or update a policy,
// then ensure the policy applies to any existing indices.
func ConfigurePolicy(
	ctx context.Context,
	osc *opensearch.Client,
	index opensearch.Index,
	policy opensearchapi.ISMPolicy,
) error {
	currentPolicy, err := GetPolicy(ctx, osc, *policy.PolicyID)
	if err != nil {
		return fmt.Errorf("get current policy: %w", err)
	}
	if currentPolicy == nil {
		if err := PutPolicy(ctx, osc, policy); err != nil {
			return fmt.Errorf("create ism policy: %w", err)
		}
	} else {
		if err := PutPolicy(
			ctx,
			osc,
			policy,
			WithSeqNo(int(*currentPolicy.SeqNo)),
			WithPrimaryTerm(*currentPolicy.PrimaryTerm),
		); err != nil {
			return fmt.Errorf("update ism policy: %w", err)
		}
	}
	if err := AddPolicyToIndices(ctx, osc, index, *policy.PolicyID); err != nil {
		return fmt.Errorf("add policy to existing indices: %w", err)
	}
	return nil
}

// GetPolicy retrieves an existing ISM policy.
func GetPolicy(ctx context.Context, osc *opensearch.Client, id string) (*ism.GetPolicyResp, error) {
	client := NewClient(osc)

	resp, err := client.Policy.GetPolicy(ctx, ism.GetPolicyReq{
		PolicyID: id,
		Header:   opensearch.NewRequestHeaders(ctx),
	})
	if err != nil {
		if resp != nil && resp.Inspect().Response.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, opensearch.HandleError("get ism policy", resp, err)
	}
	return resp, nil
}

// PutPolicy adds the given ISM policy to the cluster.
func PutPolicy(
	ctx context.Context,
	osc *opensearch.Client,
	policy opensearchapi.ISMPolicy,
	options ...PutPolicyOption,
) error {
	client := NewClient(osc)

	req, err := NewPutPolicyReq(policy, options...)
	if err != nil {
		return fmt.Errorf("create ism policy req: %w", err)
	}

	resp, err := client.Policy.PutPolicy(ctx, *req)
	if err != nil {
		return opensearch.HandleError("put ism policy", resp, err)
	}
	return nil
}

func NewPutPolicyReq(policy opensearchapi.ISMPolicy, options ...PutPolicyOption) (*ism.PutPolicyReq, error) {
	data, err := json.Marshal(opensearchapi.ISMPolicyEnvelope{Policy: &policy})
	if err != nil {
		return nil, fmt.Errorf("marshal ISM policy: %w", err)
	}
	req := &ism.PutPolicyReq{
		PolicyID: *policy.PolicyID,
		Body:     bytes.NewReader(data),
	}
	for option := range slices.Values(options) {
		option(req)
	}
	return req, nil
}

// PutPolicyOption is a functional option applied to a [ism.PutPolicyReq].
type PutPolicyOption func(*ism.PutPolicyReq)

// WithSeqNo option defines the seq_no value to reference when updating an ISM policy. If the actual value is different
// to the one specified, the update will fail.
func WithSeqNo(seqno int) PutPolicyOption {
	return func(req *ism.PutPolicyReq) {
		if req.Params == nil {
			req.Params = &ism.PutPolicyParams{}
		}
		req.Params.IfSeqNo = &seqno
	}
}

// WithPrimaryTerm option defines a primary_term value to reference when updating an ISM policy. If the actual value is
// different to the one specified, the update will fail.
func WithPrimaryTerm(term float64) PutPolicyOption {
	return func(req *ism.PutPolicyReq) {
		if req.Params == nil {
			req.Params = &ism.PutPolicyParams{}
		}
		req.Params.IfPrimaryTerm = &term
	}
}

// AddPolicyToIndices applies an ISM policy with the given ID to existing indices.
func AddPolicyToIndices(ctx context.Context, osc *opensearch.Client, index opensearch.Index, id string) error {
	client := NewClient(osc)
	resp, err := client.Policy.AddPolicy(ctx, &ism.AddPolicyReq{
		Indices: []string{index.String() + "*"},
		Body: &opensearchapi.ISMAddPolicyRequest{
			PolicyID: id,
		},
		Header: opensearch.NewRequestHeaders(ctx),
	})
	if err != nil {
		return opensearch.HandleError("add ism policy to indices", resp, err)
	}
	return nil
}
