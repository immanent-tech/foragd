// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import "github.com/resend/resend-go/v3"

type Attachment struct {
	*resend.Attachment
}

func NewRemoteFileAttachment(path, name string) *Attachment {
	return &Attachment{Attachment: &resend.Attachment{
		Path:     path,
		Filename: name,
	}}
}
