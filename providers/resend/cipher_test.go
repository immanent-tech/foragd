/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package resend_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/immanent-tech/foragd/providers/resend"
)

func TestEncodeDecode(t *testing.T) {
	SetupEnv(t)
	tests := []struct {
		name  string
		email string
	}{
		{
			name:  "encode/decode works",
			email: "test@example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := resend.EncodeEmail(tt.email)
			require.NoError(t, err, "encode email")
			decoded, err := resend.DecodeEmail(encoded)
			require.NoError(t, err, "decode email")
			if decoded != tt.email {
				t.Errorf("Encode/Decode got %v, want %v", decoded, tt.email)
			}
		})
	}
}
