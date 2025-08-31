// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import "github.com/immanent-tech/go-feed-me/models"

func MsgBadInput() *models.UserMessage {
	return &models.UserMessage{
		Status:  models.UserMessageStatusWarning,
		Summary: "There was a problem with the inputs. Please check and try again.",
	}
}

func MsgBackendErr() *models.UserMessage {
	return &models.UserMessage{
		Status:  models.UserMessageStatusError,
		Summary: "There was a problem with on the backend that stopped the request from completing.",
	}
}

func MsgForbidden() *models.UserMessage {
	return &models.UserMessage{
		Status:  models.UserMessageStatusError,
		Summary: "Only accessible to logged in users.",
	}
}
