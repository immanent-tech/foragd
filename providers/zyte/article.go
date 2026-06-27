// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package zyte

// GetHTML gets the article content as HTML.
func (a *Article) GetHTML() string {
	if a.ArticleBodyHtml != nil {
		return *a.ArticleBodyHtml
	}
	return ""
}

// GetText gets the article content as text.
func (a *Article) GetText() string {
	if a.ArticleBody != nil {
		return *a.ArticleBody
	}
	return ""
}
