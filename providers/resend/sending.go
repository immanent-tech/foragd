// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"fmt"

	"github.com/resend/resend-go/v3"
)

func SendTemplatedEmail(to string, template string) error {
	client, err := loadClient()
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	params := &resend.SendEmailRequest{
		To: []string{to},
		Template: &resend.EmailTemplate{
			Id: template,
		},
	}

	_, err = client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("send template email: %w", err)
	}

	return nil
}
