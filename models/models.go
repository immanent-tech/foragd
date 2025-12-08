// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package models contains objects and fields representing common schema within the application.
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"time"

	"github.com/go-shiori/go-readability"

	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/validation"
)

//go:generate go tool oapi-codegen -config models-cfg.yaml models.yaml

// DefaultHTTPRequestTimeout is the maximum time allowed for a background HTTP request to execute.
var DefaultHTTPRequestTimeout = 30 * time.Second

var ErrInvalidDateTimeFormat = errors.New("datetime is invalid")
var UnixEpoch = time.Unix(0, 0)

// SliceToMap generates a map from slice content by mapping key-value pairs from the slice with the given map function.
func SliceToMap[K comparable, V any, S any](s []S, mapFn func(S) (K, V)) map[K]V {
	m := make(map[K]V)
	for _, k := range s {
		key, val := mapFn(k)
		m[key] = val
	}
	return m
}

// FilterSlice will filter a slice returning an iter with elements that return true for the given filter function.
func FilterSlice[E any](s []E, fn func(E) bool) iter.Seq[E] {
	return func(yield func(s E) bool) {
		for _, v := range s {
			if fn(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// FilterMap will filter a map by the given function.
func FilterMap[K comparable, V any](m map[K]V, fn func(K, V) bool) iter.Seq2[K, V] {
	return func(yield func(k K, v V) bool) {
		for k, v := range m {
			if fn(k, v) {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

// FilterMapValues will filter map values by the given function.
func FilterMapValues[K comparable, V any](s map[K]V, fn func(V) bool) iter.Seq[V] {
	return FilterSlice(slices.Collect(maps.Values(s)), fn)
}

// GenerateHXVals generates a JSON-formatted object containing the given key-value pairs suitable for use as a hx-vals attribute.
// See also: https://htmx.org/attributes/hx-vals/
func GenerateHXVals(values map[string]any) string {
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}

// ExtractTextFromURL fetches the text content of the given URL and attempts to extract the main article content from
// it.
func ExtractTextFromURL(url string) (string, error) {
	remote, err := readability.FromURL(url, DefaultHTTPRequestTimeout)
	if err != nil {
		return "", fmt.Errorf("failed to parse content for %s, %w", url, err)
	}
	content := validation.SanitizeString(remote.Content)
	return content, nil
}

func (p *ObjectParams) Valid() error {
	if err := validation.Validate.Struct(p); err != nil {
		return err
	}
	return nil
}

func (v *ObjectParams) Sanitise() error {
	return nil
}

func (m *MarkObjectParams) Valid() error {
	if err := validation.Validate.Struct(m); err != nil {
		return err
	}
	return nil
}

func (m *MarkObjectParams) Sanitise() error {
	return nil
}

func NewObjectIssue(obj *ObjectParams, url string) *ObjectIssueRequest {
	return &ObjectIssueRequest{
		PageUrl:  url,
		ObjectID: obj.ObjectID,
		Object:   obj.Object,
	}
}

func (i *ObjectIssueRequest) Valid() error {
	if err := validation.Validate.Struct(i); err != nil {
		return err
	}
	return nil
}

func (i *ObjectIssueRequest) Sanitise() error {
	i.Details = validation.SanitizeString(i.Details)
	return nil
}

func (i *IssueRequest) Valid() error {
	if err := validation.Validate.Struct(i); err != nil {
		return err
	}
	return nil
}

func (i *IssueRequest) Sanitise() error {
	i.Details = validation.SanitizeString(i.Details)
	return nil
}

type Screenshot struct {
	*forms.FileUpload
}

func (s *Screenshot) Valid() error {
	mimeType, err := s.ParseMimetype()
	if err != nil {
		return fmt.Errorf("screenshot invalid: %w", err)
	}
	if !slices.Contains([]string{"image/jpeg", "image/png"}, mimeType) {
		return fmt.Errorf("screenshot invalid: %w", mimeType)
	}
	return nil
}

// validateDatetime will check whether a time.Time is not either the zero value or equal to the Unix epoch.
func validateDatetime(dt time.Time) (bool, error) {
	switch {
	case dt.IsZero():
		return false, fmt.Errorf("%w: is zero time value", ErrInvalidDateTimeFormat)
	case dt.Equal(UnixEpoch):
		return false, fmt.Errorf("%w: is unix epoch", ErrInvalidDateTimeFormat)
	default:
		return true, nil
	}
}
