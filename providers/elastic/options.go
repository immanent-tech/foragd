// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

// Option is a generic type for functional options.
type Option[T any] func(T)

// hasIndexOption represents requests that have an option to set the index (or
// index pattern).
type hasIndexOption[T any] interface {
	Index(value string) T
}

// // WithIndex option specifies the index (or index pattern) to use.
// func WithIndex[T hasIndexOption[T]](value string) Option[T] {
// 	return func(req T) {
// 		req = req.Index(value)
// 	}
// }

// hasIDsOption represents requests that have an option to set the doc IDs.
type hasIDsOption[T any] interface {
	Ids(ids ...string) T
}

// // WithIDs option retrieves the documents with the given IDs.
// func WithIDs[T hasIDsOption[T]](ids ...string) Option[T] {
// 	return func(req T) {
// 		req = req.Ids(ids...)
// 	}
// }

// hasSourceOption represents requests that have an option to fetch the source
// (or particular fields from the source).
type hasSourceOption[T any] interface {
	Source_(value string) T
}

// WithSource option specifies that the `_source` field should be retrieved.
func WithSource[T hasSourceOption[T]]() Option[T] {
	return func(req T) {
		req = req.Source_("true")
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

// WithDocID option allows setting the document ID.
func WithDocID[T any](id string) Option[T] {
	return func(req T) {
		request := &req

		if settable, ok := any(request).(hasDocIDOption[T]); ok {
			settable.SetDocID(id)
		}
	}
}
