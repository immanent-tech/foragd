// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package elastic defines methods and structures for interacting with Elasticsearch.
package elastic

type Object[T ~string] interface {
	GetID() T
}
