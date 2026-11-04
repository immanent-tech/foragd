/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates

import (
	"time"

	"github.com/immanent-tech/foragd/models"
)

type Article interface {
	models.Content
	models.HasState

	GetLink() string
	GetUpdatedDate() time.Time
	GetSubscriptionID() models.SubscriptionID
	GetSubscriptionTitle() string
	GetSubscriptionLink() string
	GetSubscription() models.Subscription
}
