/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package models_test

import (
	"testing"

	"github.com/immanent-tech/foragd/models"
)

func TestArticle_GetContent(t *testing.T) {
	tests := []struct {
		name    string
		article models.Article
		want    string
	}{
		{
			name: "full content",
			article: models.Article{
				ShowFullContent: true,
				Content:         new(`<div>test</div>`),
			},
			want: `<div>test</div>`,
		},
		{
			name: "full content missing, fallback to description",
			article: models.Article{
				ShowFullContent: true,
				Item: models.Item{
					Description: new("test"),
				},
			},
			want: `<p>test</p>`,
		},
		{
			name: "item content",
			article: models.Article{
				Item: models.Item{
					Content: new("test"),
				},
			},
			want: `<p>test</p>`,
		},
		{
			name: "item description",
			article: models.Article{
				Item: models.Item{
					Description: new("test"),
				},
			},
			want: `<p>test</p>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.article.GetContent()
			if got != tt.want {
				t.Errorf("GetContent() = %v, want %v", got, tt.want)
			}
		})
	}
}
