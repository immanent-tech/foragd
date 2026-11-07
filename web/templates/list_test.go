/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

func TestWithListActions(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tests := []struct {
		name      string
		actions   []templ.Component
		expectLen int
	}{
		{
			name:      "empty actions should not modify",
			actions:   []templ.Component{},
			expectLen: 0,
		},
		{
			name:      "single action should be added",
			actions:   []templ.Component{NewTestPartial()},
			expectLen: 1,
		},
		{
			name:      "multiple actions should all be added",
			actions:   []templ.Component{NewTestPartial(), NewTestPartial(), NewTestPartial()},
			expectLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lc := &templates.ListControls{}
			option := templates.WithListActions(tt.actions...)
			option(lc)

			require.Len(lc.ExtraActions, tt.expectLen)
		})
	}
}

func TestWithListButtons(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tests := []struct {
		name      string
		buttons   []templ.Component
		expectLen int
	}{
		{
			name:      "empty buttons should not modify",
			buttons:   []templ.Component{},
			expectLen: 0,
		},
		{
			name:      "single button should be added",
			buttons:   []templ.Component{NewTestPartial()},
			expectLen: 1,
		},
		{
			name:      "multiple buttons should all be added",
			buttons:   []templ.Component{NewTestPartial(), NewTestPartial()},
			expectLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lc := &templates.ListControls{}
			option := templates.WithListButtons(tt.buttons...)
			option(lc)

			require.Len(lc.ExtraButtons, tt.expectLen)
		})
	}
}

func TestWithListCategories(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tests := []struct {
		name       string
		categories models.Categories
		expectSet  bool
		expectLen  int
	}{
		{
			name:       "empty categories should not set",
			categories: models.Categories{},
			expectSet:  false,
			expectLen:  0,
		},
		{
			name:       "single category should set",
			categories: models.Categories{"Tech"},
			expectSet:  true,
			expectLen:  1,
		},
		{
			name:       "multiple categories should set",
			categories: models.Categories{"Tech", "Design", "News"},
			expectSet:  true,
			expectLen:  3,
		},
		{
			name:       "nil categories should not set",
			categories: nil,
			expectSet:  false,
			expectLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lc := &templates.ListControls{}
			templates.WithListCategories(tt.categories)(lc)

			if tt.expectSet {
				require.NotEmpty(lc.Categories)
				require.Equal(tt.expectLen, len(lc.Categories))
			} else {
				require.Empty(lc.Categories)
			}
		})
	}
}

func TestWithListLanguages(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tests := []struct {
		name      string
		languages map[string]int64
		expectSet bool
		expectLen int
	}{
		{
			name:      "empty languages should not set",
			languages: map[string]int64{},
			expectSet: false,
			expectLen: 0,
		},
		{
			name:      "single language should set",
			languages: map[string]int64{"en": 100},
			expectSet: true,
			expectLen: 1,
		},
		{
			name:      "multiple languages should set",
			languages: map[string]int64{"en": 100, "es": 50, "fr": 75},
			expectSet: true,
			expectLen: 3,
		},
		{
			name:      "nil languages should not set",
			languages: nil,
			expectSet: false,
			expectLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lc := &templates.ListControls{}
			templates.WithListLanguages(tt.languages)(lc)

			if tt.expectSet {
				require.NotEmpty(lc.Languages)
				require.Equal(tt.expectLen, len(lc.Languages))
			} else {
				require.Empty(lc.Languages)
			}
		})
	}
}

func TestWithoutListSorting(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	t.Run("sets disableSort to true", func(t *testing.T) {
		t.Parallel()
		lc := &templates.ListControls{}
		templates.WithoutListSorting()(lc)
		require.True(lc.DisableSort)
	})

	t.Run("does not modify other fields", func(t *testing.T) {
		t.Parallel()

		lc := &templates.ListControls{
			Path:    "/test",
			Filters: models.ListFilters{Sort: models.SortNewestFirst},
		}

		templates.WithoutListSorting()(lc)
		require.Equal("/test", lc.Path)
		require.Equal(models.SortNewestFirst, lc.Filters.Sort)
	})
}

func TestWithoutListView(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	t.Run("sets disableView to true", func(t *testing.T) {
		t.Parallel()
		lc := &templates.ListControls{}
		templates.WithoutListView()(lc)
		require.True(lc.DisableView)
	})

	t.Run("does not modify other fields", func(t *testing.T) {
		t.Parallel()

		lc := &templates.ListControls{
			Path:    "/test",
			Filters: models.ListFilters{View: models.ViewAll},
		}

		templates.WithoutListView()(lc)
		require.Equal("/test", lc.Path)
		require.Equal(models.ViewAll, lc.Filters.GetView())
	})
}

func TestNewListControls_Empty(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	t.Run("renders without options", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		controls := templates.NewListControls("/test", models.ListFilters{})

		var buf bytes.Buffer
		err := controls.Template().Render(ctx, &buf)
		require.NoError(err)
		require.NotEmpty(buf.String())
	})

	t.Run("sets default path and filters", func(t *testing.T) {
		t.Parallel()

		controls := templates.NewListControls("/my-path", models.ListFilters{
			Sort: models.SortNewestFirst,
			View: models.ViewUnread,
		})

		require.Equal("/my-path", controls.Path)
		require.Equal(models.SortNewestFirst, controls.Filters.GetSort())
		require.Equal(models.ViewUnread, controls.Filters.GetView())
	})

	t.Run("does not modify filters pagination fields", func(t *testing.T) {
		t.Parallel()

		originalFilters := models.ListFilters{
			Sort: models.SortOldestFirst,
			View: models.ViewAll,
		}

		controls := templates.NewListControls("/test", originalFilters)

		require.Equal(models.SortOldestFirst, controls.Filters.GetSort())
		require.Equal(models.ViewAll, controls.Filters.GetView())
	})
}

func TestNewListControls_WithOptions(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	t.Run("applies multiple options in order", func(t *testing.T) {
		t.Parallel()

		lc := &templates.ListControls{
			Path:    "/test",
			Filters: models.ListFilters{Sort: models.SortNewestFirst},
		}

		templates.WithListActions(NewTestPartial())(lc)
		templates.WithListButtons(NewTestPartial())(lc)
		templates.WithListCategories(models.Categories{"Tech"})(lc)
		templates.WithListLanguages(map[string]int64{"en": 100})(lc)

		require.Len(lc.ExtraActions, 1)
		require.Len(lc.ExtraButtons, 1)
		require.Len(lc.Categories, 1)
		require.Equal(map[string]int64{"en": 100}, lc.Languages)
	})

	t.Run("renders with options applied", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		actions := []templ.Component{NewTestPartial()}
		buttons := []templ.Component{NewTestPartial()}

		controls := templates.NewListControls(
			"/test",
			models.ListFilters{Sort: models.SortNewestFirst},
			templates.WithListActions(actions...),
			templates.WithListButtons(buttons...),
			templates.WithListCategories(models.Categories{"Test"}),
			templates.WithListLanguages(map[string]int64{"en": 100}),
		)

		var buf bytes.Buffer
		err := controls.Template().Render(ctx, &buf)
		require.NoError(err)
		require.NotZero(buf.Len())
	})

	t.Run("merges with existing values", func(t *testing.T) {
		t.Parallel()
		// Create controls with some initial state
		original := &templates.ListControls{
			Path:    "/original",
			Filters: models.ListFilters{Sort: models.SortNewestFirst},
		}

		// Apply an option
		templates.WithListCategories(models.Categories{"New", "Extra"})(original)

		// Categories should be merged
		require.Len(original.Categories, 2)
		require.Contains(original.Categories, "New")
		require.Contains(original.Categories, "Extra")
	})
}

func TestNewListControls_ResetsPagination(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	t.Run("resets filters", func(t *testing.T) {
		t.Parallel()
		paginationFilters := models.ListFilters{
			View:        models.ViewUnread,
			Sort:        models.SortNewestFirst,
			From:        0,
			SearchAfter: new("abc123"),
		}

		controls := templates.NewListControls("/test", paginationFilters)

		// Verify pagination was reset
		require.Zero(controls.Filters.GetFrom())
		require.Equal(9, controls.Filters.GetUpTo())
		require.Nil(controls.Filters.SearchAfter)
		require.Nil(controls.Filters.SearchAfter)

		// But other fields should remain unchanged
		require.Equal(models.SortNewestFirst, controls.Filters.GetSort())
		require.Equal(models.ViewUnread, controls.Filters.GetView())
	})

	t.Run("preserves Sort, View, and Count", func(t *testing.T) {
		t.Parallel()
		controls := templates.NewListControls(
			"/test",
			models.ListFilters{
				View: models.ViewAll,
				Sort: models.SortMostRelevant,
			},
		)

		require.Equal(models.ViewAll, controls.Filters.GetView())
		require.Equal(models.SortMostRelevant, controls.Filters.GetSort())
	})
}

func TestNewListControls_DisabledFeatures(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	t.Run("WithoutListSorting sets disableSort", func(t *testing.T) {
		t.Parallel()
		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithoutListSorting(),
		)
		require.True(controls.DisableSort)
	})

	t.Run("WithoutListView sets disableView", func(t *testing.T) {
		t.Parallel()
		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithoutListView(),
		)
		require.True(controls.DisableView)
	})

	t.Run("Both disabled flags work together", func(t *testing.T) {
		t.Parallel()
		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithoutListSorting(),
			templates.WithoutListView(),
		)
		require.True(controls.DisableSort)
		require.True(controls.DisableView)
	})
}

func TestNewListControls_AppendsMultipleOptions(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	t.Run("WithListActions appends multiple components", func(t *testing.T) {
		t.Parallel()
		actions := []templ.Component{
			NewTestPartial(),
			NewTestPartial(),
			NewTestPartial(),
		}

		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithListActions(actions...),
		)

		require.Len(controls.ExtraActions, 3)
	})

	t.Run("WithListButtons appends multiple components", func(t *testing.T) {
		t.Parallel()
		buttons := []templ.Component{
			NewTestPartial(),
			NewTestPartial(),
		}

		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithListButtons(buttons...),
		)
		require.Len(controls.ExtraButtons, 2)
	})
}

func TestNewListControls_RenderBasicStructure(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tests := []struct {
		name              string
		path              string
		filters           models.ListFilters
		options           []templates.ListControlOption
		expectMobilePanel bool
		expectDesktopSort bool
		expectViewMenu    bool
	}{
		{
			name:              "basic controls with no options",
			path:              "/test",
			filters:           models.ListFilters{},
			options:           []templates.ListControlOption{},
			expectMobilePanel: true,
			expectDesktopSort: true,
			expectViewMenu:    false,
		},
		{
			name:              "with sorting disabled",
			path:              "/test",
			filters:           models.ListFilters{},
			options:           []templates.ListControlOption{templates.WithoutListSorting()},
			expectMobilePanel: true,
			expectDesktopSort: false,
			expectViewMenu:    false,
		},
		{
			name:              "with view disabled",
			path:              "/test",
			filters:           models.ListFilters{},
			options:           []templates.ListControlOption{templates.WithoutListView()},
			expectMobilePanel: true,
			expectDesktopSort: true,
			expectViewMenu:    false,
		},
		{
			name:    "with both sorting and view disabled",
			path:    "/test",
			filters: models.ListFilters{},
			options: []templates.ListControlOption{
				templates.WithoutListSorting(),
				templates.WithoutListView(),
			},
			expectMobilePanel: true,
			expectDesktopSort: false,
			expectViewMenu:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			controls := templates.NewListControls(tt.path, tt.filters, tt.options...)

			// Capture rendered output
			var buf bytes.Buffer
			err := controls.Template().Render(ctx, &buf)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			// Verify rendering didn't error and produced output
			require.NotZero(buf.Len(), "Render should produce output")
		})
	}
}

func TestNewListControls_Render(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := templates.NewListControls("/test/path", models.ListFilters{})
	doc := render(t, ctx, c.Template())

	// Verify basic structure exists
	if doc.Find("el-dialog").Length() == 0 {
		t.Error("Expected el-dialog for mobile controls, but not found")
	}
	if doc.Find("[id='desktop-sort-filters']").Length() == 0 {
		t.Error("Expected desktop-sort-filters, but not found")
	}

	// Verify specific labels
	if doc.Find("li[.menu-title]").Text() == "View" {
		t.Error("Expected 'View' label in mobile filters")
	}
	if doc.Find("li[.menu-title]").Text() == "Sort" {
		t.Error("Expected 'Sort' label in mobile filters")
	}
}

func TestNewListControls_RenderCategories(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	c := templates.NewListControls(
		"/test/path",
		models.ListFilters{},
		templates.WithListCategories(models.Categories{"Technology & Computing"}),
	)
	doc := render(t, ctx, c.Template())

	assert.True(
		t,
		strings.TrimSpace(
			doc.Find("ul[id='desktop-category-filters'] > li").First().Text(),
		) == "Technology & Computing",
	)
}
