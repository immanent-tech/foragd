// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"time"

	"github.com/go-shiori/go-readability"

	"github.com/immanent-tech/foragd/validation"
)

func fetchArticleRemoteContent(url string) (string, error) {
	remote, err := readability.FromURL(url, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to parse content for %s, %w", url, err)
	}
	content := validation.SanitizeString(remote.Content)
	return content, nil
}
