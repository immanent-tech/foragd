/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/server/handlers"
)

type fakeVerifier struct{ err error }

func (f fakeVerifier) Verify(*http.Request, []byte) error { return f.err }

type fakeProcessor struct {
	got    resend.EmailRecieved
	err    error
	called bool
}

func (f *fakeProcessor) Process(_ context.Context, d resend.EmailRecieved) error {
	f.called, f.got = true, d
	return f.err
}

func TestHandleResendWebhook(t *testing.T) {
	tests := []struct {
		name       string
		verifyErr  error
		procErr    error
		body       string
		wantStatus int
		wantCalled bool
	}{
		{"bad signature", errors.New("bad"), nil, `{"type":"email.received"}`, 400, false},
		{"malformed json", nil, nil, `{`, 400, false},
		{"missing type doesn't panic", nil, nil, `{}`, 200, false},
		{"unhandled type", nil, nil, `{"type":"email.sent","data":{}}`, 200, false},
		{"received ok", nil, nil, `{"type":"email.received","data":{"from":"a@b.com","to":["u@x.com"]}}`, 200, true},
		{"processor error still acks", nil, errors.New("boom"), `{"type":"email.received","data":{}}`, 200, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := &fakeProcessor{err: tt.procErr}
			h := handlers.HandleResendWebhook(fakeVerifier{tt.verifyErr}, proc)

			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body)))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if proc.called != tt.wantCalled {
				t.Errorf("processor called = %v, want %v", proc.called, tt.wantCalled)
			}
		})
	}
}
