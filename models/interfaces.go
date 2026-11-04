/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package models

// Document is any object that has an ID.
type Document interface {
	GetID() string
}

type Content interface {
	Document
	GetTitle() string
	GetDescription() string
	GetAuthors() []Author
	GetImage() *RemoteImage
	GetContent() string
	GetCategories(limit int) Categories
}

type HasState interface {
	IsUnread() bool
	IsFavorite() bool
}
