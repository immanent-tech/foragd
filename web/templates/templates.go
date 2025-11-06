// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"fmt"
	"time"

	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/models"
)

var (
	// ContentID points to the element containing the main content of the page.
	ContentID = ID("content")
	// ErrorID points to an element that can be used to display error messages to the user.
	ErrorID = ID("error")
	// ModalContainerID points to an element that holds a modal.
	ModalContainerID = ID("modals")
	// ModalID points to an element that can be used to render a modal.
	ModalID = ID("modal")
	// NotificationsID points to an element that can be used for displaying notifications to the user.
	NotificationsID = ID("notifications")
)

type partialsCtxKey string

// ID represents an id attribute in a HTML element.
type ID string

// Target returns the id attribute as a target (i.e., for htmx requests). This
// is the base id string with a "#" prefix.
func (a ID) Target() string {
	return fmt.Sprintf("#%s", a)
}

// String returns the id attribute as a string.
func (a ID) String() string {
	return string(a)
}

type objectDetails interface {
	GetTitle() string
	GetDescription() string
	GetLink() string
	GetCategories(maxCount int) models.Categories
	GetAuthors() []string
	GetUpdatedDate() time.Time
	GetImage() *types.ImageInfo
	IsFavorite() bool
}

type object[T ~string] interface {
	objectDetails

	GetID() T
	GetObjectType() models.ObjectType
}
