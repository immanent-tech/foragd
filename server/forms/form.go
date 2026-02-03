// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package forms contains methods for handling form decoding and encoding.
package forms

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/go-playground/form/v4"

	"github.com/immanent-tech/foragd/models"
)

var (
	// ErrDecode indicates an error occurred during decoding.
	ErrDecode = errors.New("error in decoding")
	// ErrEncode indicates an error occurred during encoding.
	ErrEncode = errors.New("error in encoding")
	// ErrValidation indicates an error occurred during validation.
	ErrValidation = errors.New("validation failed")
	// ErrNoFormData indicates that no form data was parsed.
	ErrNoFormData = errors.New("no form data")
	// ErrSanitise indicates an error occurred during sanitisation.
	ErrSanitise = errors.New("sanitisation failed")
)

var (
	decoder = form.NewDecoder()
)

// defaultMaxSize for a multipart for submission is 32 MB.
const defaultMaxSize = 32 << 20

// FormInput represents form input data. It has methods to test if the data is valid and to sanitise the input data.
type FormInput interface {
	Valid() error
	Sanitise() error
}

// DecodeForm will decode submitted form contents into the passed in type. It
// will perform validation of the type and will return the type and a boolean
// true if it is valid. If decoding the form submission fails, a non-nill error
// is returned.
func DecodeForm[T FormInput](req *http.Request) (T, bool, error) {
	var (
		obj T
		err error
	)
	if err := req.ParseForm(); err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	obj, err = decodeObject(req, obj)
	if err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	return obj, true, nil
}

func DecodeMultiPartForm[T FormInput](req *http.Request) (T, bool, error) {
	var (
		obj T
		err error
	)
	if err := req.ParseMultipartForm(defaultMaxSize); err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	obj, err = decodeObject(req, obj)
	if err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	return obj, true, nil
}

func decodeObject[T FormInput](req *http.Request, obj T) (T, error) {
	// Decode the form values.
	if err := decoder.Decode(&obj, req.Form); err != nil {

	}
	// Sanitise the object.
	if err := obj.Sanitise(); err != nil {
		return obj, fmt.Errorf("%w: %w", ErrSanitise, err)
	}
	// Validate the object.
	if err := obj.Valid(); err != nil {
		return obj, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	return obj, nil
}

// FileUpload represents file data uploaded through a mutlipart form.
type FileUpload interface {
	Set(hdr *multipart.FileHeader, data multipart.File)
}

// DecodeMultipartFile will the file represented by the given field in a multipart form
// submission. It will perform validation of the file and will return the file
// object and a boolean true if it is valid. If decoding fails, a non-nill error
// is returned.
func DecodeMultipartFile(req *http.Request, field string) (*models.FileUpload, error) {
	// Parse form values in request.
	if err := req.ParseMultipartForm(defaultMaxSize); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	// Decode the form values.
	data, hdr, err := req.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	// Create a models.FileUpload object.
	upload := &models.FileUpload{
		Data:   data,
		Header: hdr,
	}
	// Validate file upload.
	if err := upload.Valid(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	return upload, nil
}

// DecodeMultipartValue will the file represented by the given field in a multipart form
// submission. It will perform validation of the file and will return the file
// object and a boolean true if it is valid. If decoding fails, a non-nill error
// is returned.
func DecodeMultipartValue(req *http.Request, field string) (string, error) {
	// Parse form values in request.
	if err := req.ParseMultipartForm(defaultMaxSize); err != nil {
		return "", errors.Join(ErrDecode, err)
	}
	// Decode the form values.
	return req.FormValue(field), nil
}
