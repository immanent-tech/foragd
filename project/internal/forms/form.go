// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package forms

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/go-playground/form/v4"
)

var decoder = form.NewDecoder()
var encoder = form.NewEncoder()

// DefaultMaxSize for a multipart for submission is 32 MB.
const DefaultMaxSize = 32 << 20

// Validator is an object that can be validated.
type Validator interface {
	Valid() bool
}

// File is an object to store an uploaded file.
type File interface {
	Valid() bool
	Load(data multipart.File, hdr *multipart.FileHeader) error
}

// DecodeForm will decode submitted form contents into the passed in type. It
// will perform validation of the type and will return the type and a boolean
// true if it is valid. If decoding the form submission fails, a non-nill error
// is returned.
func DecodeForm[T Validator](req *http.Request) (T, bool, error) {
	var obj T
	// Parse form values in request.
	if err := req.ParseForm(); err != nil {
		return obj, false, fmt.Errorf("parse form: %w", err)
	}
	// Decode the form values.
	err := decoder.Decode(&obj, req.Form)
	if err != nil {
		return obj, false, fmt.Errorf("decode form: %w", err)
	}
	// Validate the object.
	if ok := obj.Valid(); !ok {
		return obj, false, fmt.Errorf("invalid %T", obj)
	}
	return obj, true, nil
}

// DecodeCustom will decode submitted form contents into the passed in type,
// using the passed function to decode url.Values into the defined type. It will also
// will perform validation of the type and will return the type and a boolean
// true if it is valid. If decoding the form submission fails, a non-nill error
// is returned.
func DecodeCustom[T Validator](req *http.Request, decoderFunc func(params url.Values) (T, error)) (T, bool, error) {
	var obj T
	// Parse form values in request.
	if err := req.ParseForm(); err != nil {
		return obj, false, fmt.Errorf("parse form: %w", err)
	}
	// Call the custom decoder function.
	obj, err := decoderFunc(req.Form)
	if err != nil {
		return obj, false, fmt.Errorf("decode form: %w", err)
	}
	// Validate the object.
	if ok := obj.Valid(); !ok {
		return obj, false, fmt.Errorf("invalid %T", obj)
	}

	return obj, true, nil
}

// DecodeFile will the file represented by the given field in a multipart form
// submission. It will perform validation of the file and will return the file
// object and a boolean true if it is valid. If decoding fails, a non-nill error
// is returned.
func DecodeFile[T File](req *http.Request, field string, file T) (T, bool, error) {
	// Parse form values in request.
	if err := req.ParseMultipartForm(DefaultMaxSize); err != nil {
		return file, false, fmt.Errorf("parse form: %w", err)
	}
	// Decode the form values.
	data, hdr, err := req.FormFile(field)
	if err != nil {
		return file, false, fmt.Errorf("decode file: %w", err)
	}
	// Load file data.
	err = file.Load(data, hdr)
	if err != nil {
		return file, false, fmt.Errorf("load file: %w", err)
	}
	// Validate the file data.
	if ok := file.Valid(); !ok {
		return file, false, fmt.Errorf("invalid file %T", file)
	}

	return file, true, nil
}

func DecodeRequest[T any](req *http.Request) (T, error) {
	var obj T
	if err := json.NewDecoder(req.Body).Decode(&obj); err != nil {
		return obj, fmt.Errorf("decode request: %w", err)
	}

	// if ok, problems := obj.Valid(); !ok {
	// 	return obj, problems, fmt.Errorf("invalid %T: %d problems", obj, len(problems))
	// }

	return obj, nil
}

// EncodeForm will encode the given object as url.Values, using the struct tags
// where possible. It will perform validation of the object before attempting
// encoding. If the object cannot be encoded or validation fails, a non-nil
// error is returned.
func EncodeForm[T Validator](obj T) (url.Values, error) {
	// Validate the object.
	if ok := obj.Valid(); !ok {
		return nil, fmt.Errorf("invalid %T", obj)
	}

	values, err := encoder.Encode(&obj)
	if err != nil {
		return nil, fmt.Errorf("encode values: %w", err)
	}
	return values, nil
}
