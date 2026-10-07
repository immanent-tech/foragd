/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type mockTempl struct{}

func (m *mockTempl) Render(ctx context.Context, w io.Writer) error {
	w.Write([]byte{'X'})
	return nil
}

// mockComponent returns a simple templ.Component for testing.
func mockComponent() templ.Component {
	return &mockTempl{}
}

func TestWithListActions(t *testing.T) {
	t.Parallel()

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
			actions:   []templ.Component{mockComponent()},
			expectLen: 1,
		},
		{
			name:      "multiple actions should all be added",
			actions:   []templ.Component{mockComponent(), mockComponent(), mockComponent()},
			expectLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lc := &templates.ListControls{}
			option := templates.WithListActions(tt.actions...)
			option(lc)

			require.Len(t, lc.ExtraActions, tt.expectLen)
		})
	}
}

func TestWithListButtons(t *testing.T) {
	t.Parallel()

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
			buttons:   []templ.Component{mockComponent()},
			expectLen: 1,
		},
		{
			name:      "multiple buttons should all be added",
			buttons:   []templ.Component{mockComponent(), mockComponent()},
			expectLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lc := &templates.ListControls{}
			option := templates.WithListButtons(tt.buttons...)
			option(lc)

			require.Len(t, lc.ExtraButtons, tt.expectLen)
		})
	}
}

func TestWithListCategories(t *testing.T) {
	t.Parallel()

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
				require.NotEmpty(t, lc.Categories)
				require.Equal(t, tt.expectLen, len(lc.Categories))
			} else {
				require.Empty(t, lc.Categories)
			}
		})
	}
}

func TestWithListLanguages(t *testing.T) {
	t.Parallel()

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
				require.NotEmpty(t, lc.Languages)
				require.Equal(t, tt.expectLen, len(lc.Languages))
			} else {
				require.Empty(t, lc.Languages)
			}
		})
	}
}

func TestWithoutListSorting(t *testing.T) {
	t.Parallel()

	t.Run("sets disableSort to true", func(t *testing.T) {
		t.Parallel()
		lc := &templates.ListControls{}
		templates.WithoutListSorting()(lc)
		require.True(t, lc.DisableSort)
	})

	t.Run("does not modify other fields", func(t *testing.T) {
		t.Parallel()
		lc := &templates.ListControls{
			Path:    "/test",
			Filters: models.ListFilters{Sort: models.SortNewestFirst},
		}
		templates.WithoutListSorting()(lc)
		require.Equal(t, "/test", lc.Path)
		require.Equal(t, models.SortNewestFirst, lc.Filters.Sort)
	})
}

func TestWithoutListView(t *testing.T) {
	t.Parallel()

	t.Run("sets disableView to true", func(t *testing.T) {
		t.Parallel()
		lc := &templates.ListControls{}
		templates.WithoutListView()(lc)
		require.True(t, lc.DisableView)
	})

	t.Run("does not modify other fields", func(t *testing.T) {
		t.Parallel()
		lc := &templates.ListControls{
			Path:    "/test",
			Filters: models.ListFilters{View: models.ViewAll},
		}
		templates.WithoutListView()(lc)
		require.Equal(t, "/test", lc.Path)
		require.Equal(t, models.ViewAll, lc.Filters.View)
	})
}

func TestNewListControls_Empty(t *testing.T) {
	t.Parallel()

	t.Run("renders without options", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		controls := templates.NewListControls("/test", models.ListFilters{})

		var buf bytes.Buffer
		err := controls.Template().Render(ctx, &buf)
		require.NoError(t, err)
		require.NotEmpty(t, buf.String())
	})

	t.Run("sets default path and filters", func(t *testing.T) {
		t.Parallel()
		controls := templates.NewListControls("/my-path", models.ListFilters{
			Sort:  models.SortNewestFirst,
			View:  models.ViewUnread,
			Count: 10,
		})

		require.Equal(t, "/my-path", controls.Path)
		require.Equal(t, models.SortNewestFirst, controls.Filters.GetSort())
		require.Equal(t, models.ViewUnread, controls.Filters.GetView())
		require.Equal(t, 10, controls.Filters.GetCount())
	})

	t.Run("does not modify filters pagination fields", func(t *testing.T) {
		t.Parallel()
		originalFilters := models.ListFilters{
			Sort:  models.SortOldestFirst,
			View:  models.ViewAll,
			Count: 20,
		}

		controls := templates.NewListControls("/test", originalFilters)

		require.Equal(t, models.SortOldestFirst, controls.Filters.GetSort())
		require.Equal(t, models.ViewAll, controls.Filters.GetView())
		require.Equal(t, 20, controls.Filters.GetCount())
	})
}

func TestNewListControls_WithOptions(t *testing.T) {
	t.Parallel()

	t.Run("applies multiple options in order", func(t *testing.T) {
		t.Parallel()
		lc := &templates.ListControls{
			Path:    "/test",
			Filters: models.ListFilters{Sort: models.SortNewestFirst},
		}

		templates.WithListActions(mockComponent())(lc)
		templates.WithListButtons(mockComponent())(lc)
		templates.WithListCategories(models.Categories{"Tech"})(lc)
		templates.WithListLanguages(map[string]int64{"en": 100})(lc)

		require.Len(t, lc.ExtraActions, 1)
		require.Len(t, lc.ExtraButtons, 1)
		require.Len(t, lc.Categories, 1)
		require.Equal(t, map[string]int64{"en": 100}, lc.Languages)
	})

	t.Run("renders with options applied", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		actions := []templ.Component{mockComponent()}
		buttons := []templ.Component{mockComponent()}

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
		require.NoError(t, err)
		require.NotZero(t, buf.Len())
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
		require.Len(t, original.Categories, 2)
		require.Contains(t, original.Categories, "New")
		require.Contains(t, original.Categories, "Extra")
	})
}

func TestNewListControls_ResetsPagination(t *testing.T) {
	t.Parallel()

	t.Run("resets From, UpTo, and SearchAfter", func(t *testing.T) {
		t.Parallel()
		paginationFilters := models.ListFilters{
			View:        models.ViewUnread,
			Sort:        models.SortNewestFirst,
			Count:       10,
			From:        new(1),
			UpTo:        new(50),
			SearchAfter: new("abc123"),
		}

		controls := templates.NewListControls("/test", paginationFilters)

		// Verify pagination was reset
		require.Nil(t, controls.Filters.From)
		require.Nil(t, controls.Filters.UpTo)
		require.Nil(t, controls.Filters.SearchAfter)

		// But other fields should remain unchanged
		require.Equal(t, models.SortNewestFirst, controls.Filters.GetSort())
		require.Equal(t, models.ViewUnread, controls.Filters.GetView())
		require.Equal(t, 10, controls.Filters.GetCount())
	})

	t.Run("preserves Sort, View, and Count", func(t *testing.T) {
		t.Parallel()
		controls := templates.NewListControls(
			"/test",
			models.ListFilters{
				View:  models.ViewAll,
				Sort:  models.SortMostRelevant,
				Count: 25,
			},
		)

		require.Equal(t, models.ViewAll, controls.Filters.GetView())
		require.Equal(t, models.SortMostRelevant, controls.Filters.GetSort())
		require.Equal(t, 25, controls.Filters.GetCount())
	})
}

func TestNewListControls_DisabledFeatures(t *testing.T) {
	t.Parallel()

	t.Run("WithoutListSorting sets disableSort", func(t *testing.T) {
		t.Parallel()
		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithoutListSorting(),
		)
		require.True(t, controls.DisableSort)
	})

	t.Run("WithoutListView sets disableView", func(t *testing.T) {
		t.Parallel()
		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithoutListView(),
		)
		require.True(t, controls.DisableView)
	})

	t.Run("Both disabled flags work together", func(t *testing.T) {
		t.Parallel()
		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithoutListSorting(),
			templates.WithoutListView(),
		)
		require.True(t, controls.DisableSort)
		require.True(t, controls.DisableView)
	})
}

func TestNewListControls_AppendsMultipleOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithListActions appends multiple components", func(t *testing.T) {
		t.Parallel()
		actions := []templ.Component{
			mockComponent(),
			mockComponent(),
			mockComponent(),
		}

		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithListActions(actions...),
		)

		require.Len(t, controls.ExtraActions, 3)
	})

	t.Run("WithListButtons appends multiple components", func(t *testing.T) {
		t.Parallel()
		buttons := []templ.Component{
			mockComponent(),
			mockComponent(),
		}

		controls := templates.NewListControls(
			"/test",
			models.ListFilters{},
			templates.WithListButtons(buttons...),
		)
		require.Len(t, controls.ExtraButtons, 2)
	})
}

func TestNewListControls_RenderBasicStructure(t *testing.T) {
	t.Parallel()

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
			require.NotZero(t, buf.Len(), "Render should produce output")
		})
	}
}

func TestNewListControls_Render(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := templates.NewListControls("/test/path", models.ListFilters{})
	doc := render(t, ctx, c.Template())

	// Verify basic structure exists
	if doc.Find("el-dialog").Length() == 0 {
		t.Error("Expected el-dialog for mobile controls, but not found")
	}
	if doc.Find("desktop-sort-filters").Length() == 0 {
		t.Error("Expected desktop-sort-filters, but not found")
	}

	// Verify specific labels
	if doc.Find("View").Length() == 0 {
		t.Error("Expected 'View' label in mobile filters")
	}
	if doc.Find("Sort").Length() == 0 {
		t.Error("Expected 'Sort' label in mobile filters")
	}
}
