// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend_test

import "testing"

func SetupEnv(t *testing.T) {
	t.Helper()
	t.Setenv("RESEND_KEY", "key")
	t.Setenv("RESEND_SALT", "salt")
	t.Setenv("RESEND_WEBHOOKSECRET", "secret")
	t.Setenv("RESEND_APIKEY", "apikey")
	t.Setenv("RESEND_ADMINEMAIL", "admin@test.com")
	t.Setenv("RESEND_REPLYTO", "reply-to@test.com")
}
