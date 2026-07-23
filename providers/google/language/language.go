// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package language

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	language "cloud.google.com/go/language/apiv1"
	"cloud.google.com/go/language/apiv1/languagepb"
	"github.com/immanent-tech/go-base/pkg/htmlx"
)

var client *language.Client

//nolint:sloglint // no context passed.
var initClient = sync.OnceValue(func() error {
	var err error
	client, err = language.NewClient(context.Background())
	if err != nil {
		return fmt.Errorf("load language client: %w", err)
	}
	slog.Debug("Language client created.")
	return nil
})

// Classify performs text classification (categorization) of the given text.
func Classify(ctx context.Context, text string) ([]Category, error) {
	if err := initClient(); err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}
	req := &languagepb.ClassifyTextRequest{
		Document: newDocument(text),
		ClassificationModelOptions: &languagepb.ClassificationModelOptions{
			ModelType: &languagepb.ClassificationModelOptions_V2Model_{
				V2Model: &languagepb.ClassificationModelOptions_V2Model{
					ContentCategoriesVersion: languagepb.ClassificationModelOptions_V2Model_V2,
				},
			},
		},
	}
	resp, err := client.ClassifyText(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("classify text: %w", err)
	}

	categories := make([]Category, 0, len(resp.GetCategories()))
	for category := range slices.Values(resp.GetCategories()) {
		categories = append(categories, category)
	}

	return categories, nil
}

// Category is a category that has been derived from analyzed text.
type Category interface {
	GetName() string
	GetConfidence() float32
}

// newDocument creates a document that can be used as input to the language API requests.
func newDocument(text string) *languagepb.Document {
	doc := &languagepb.Document{
		Source: &languagepb.Document_Content{
			Content: text,
		},
	}

	// Determine if the document contains HTML and set the type appropriately.
	switch htmlx.IsHTML(text) {
	case true:
		doc.Type = languagepb.Document_HTML
	case false:
		doc.Type = languagepb.Document_PLAIN_TEXT
	}

	return doc
}
