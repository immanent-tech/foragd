/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates

import "github.com/immanent-tech/foragd/models"

type MarkAction struct {
	Path  string
	Label string
	Read  bool // true => "mark read" variant (shows opened-mail icon)
}

func NewMarkAction(path, label string, view models.View) MarkAction {
	if view == models.ViewUnread {
		return MarkAction{Path: path + "/" + string(models.MarkRead), Label: label + " read", Read: true}
	}
	return MarkAction{Path: path + "/" + string(models.MarkUnread), Label: label + " unread"}
}
