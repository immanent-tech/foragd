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
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v5/plugins/ism"

	"github.com/immanent-tech/foragd/providers/opensearch"
)

// NewClient creates a new ISM client.
func NewClient(osc *opensearch.Client) *ism.Client {
	return ism.NewClient(osc.Client.Client)
}

func ConfigurePolicy(ctx context.Context, osc *opensearch.Client, policy opensearchapi.ISMPolicy) error {
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
	return nil
}

func GetPolicy(ctx context.Context, osc *opensearch.Client, id string) (*ism.GetPolicyResp, error) {
	client := NewClient(osc)

	resp, err := client.Policy.GetPolicy(ctx, ism.GetPolicyReq{
		PolicyID: id,
		Header:   opensearch.NewRequestHeaders(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("get ism policy %s: %w", id, err)
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
		return opensearch.GenerateAPIError("put ism policy", resp)
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

type PutPolicyOption func(*ism.PutPolicyReq)

func WithSeqNo(seqno int) PutPolicyOption {
	return func(req *ism.PutPolicyReq) {
		if req.Params == nil {
			req.Params = &ism.PutPolicyParams{}
		}
		req.Params.IfSeqNo = &seqno
	}
}

func WithPrimaryTerm(term float64) PutPolicyOption {
	return func(req *ism.PutPolicyReq) {
		if req.Params == nil {
			req.Params = &ism.PutPolicyParams{}
		}
		req.Params.IfPrimaryTerm = &term
	}
}
