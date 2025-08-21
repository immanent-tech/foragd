// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package elastic defines methods and structures for interacting with Elasticsearch.
package elastic

type Object[T ~string] interface {
	GetID() T
}

// func VariantToValue[S any](fv types.FieldValue) (S, error) {
// 	value, ok := fv.(S)
// 	if !ok {

// 	}

// 	err := variant.Store(&value)
// 	if err != nil {
// 		return value, fmt.Errorf("unable to convert D-Bus variant %v to type %T: %w", variant, value, err)
// 	}

// 	return value, nil
// }
