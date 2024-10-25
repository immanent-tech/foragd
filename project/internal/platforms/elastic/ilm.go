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
	"os"
	"path/filepath"

	"github.com/elastic/go-elasticsearch/v8/typedapi/ilm/putlifecycle"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

const (
	policyFileSuffix = ".ilm.policy.json"
)

var (
	ErrInvalidILMPolicyFile = errors.New("invalid ILM policy file")
	ErrPutPolicy            = errors.New("put ILM policy failed")
)

func GetILMPolicy(policyName string) (*types.IlmPolicy, error) {
	data, err := os.ReadFile(filepath.Join(assetsPath, policyName+policyFileSuffix))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidILMPolicyFile, err)
	}

	policy := types.NewIlmPolicy()

	if err := policy.UnmarshalJSON(data); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidILMPolicyFile, err)
	}

	return policy, nil
}

func (c *Client) PutILMPolicy(ctx context.Context, policyName string, policy *types.IlmPolicy) error {
	req := &putlifecycle.Request{
		Policy: policy,
	}

	resp, err := c.API.Ilm.PutLifecycle(policyName).Request(req).Do(ctx)
	c.logger.Log(ctx, LevelTrace, "ilm response", slog.Any("response", resp))

	if err != nil {
		return fmt.Errorf("%w: %w", ErrPutPolicy, err)
	}

	return nil
}
