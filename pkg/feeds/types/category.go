// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import "encoding/json"

func (c *Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Value)
}

func (c *Category) UnmarshalJSON(v []byte) error {
	var value string
	err := json.Unmarshal(v, &value)
	if err != nil {
		return err
	}
	c.Value = value
	return nil
}
