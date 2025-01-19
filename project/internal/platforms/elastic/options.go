// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

// Option is a generic type for request options.
type Option[T any] func(T) T

type hasIndexPatternOption[T any] interface {
	Index(value string) T
}

// WithValue allows setting an value on a component.
func WithIndexPattern[T any](value string) Option[T] {
	return func(req T) T {
		if settable, ok := any(req).(hasIndexPatternOption[T]); ok {
			req = settable.Index(value)
		}

		return req
	}
}

type docIDOption struct {
	id string
}

func (o *docIDOption) SetDocID(id string) {
	o.id = id
}

func (o *docIDOption) GetDocID() string {
	return o.id
}

type hasDocIDOption[T any] interface {
	SetDocID(id string)
}

// WithDocID allows setting the document ID as an option.
func WithDocID[T any](id string) Option[T] {
	return func(req T) T {
		request := &req

		if settable, ok := any(request).(hasDocIDOption[T]); ok {
			settable.SetDocID(id)
		}

		return *request
	}
}
