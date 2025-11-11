// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package forms contains methods for handling form decoding and encoding.
package forms

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"

	"github.com/go-playground/form/v4"
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
	encoder = form.NewEncoder()
)

// defaultMaxSize for a multipart for submission is 32 MB.
const defaultMaxSize = 32 << 20

// FormInput represents form input data. It has methods to test if the data is valid and to sanitise the input data.
type FormInput interface {
	Valid() (bool, error)
	Sanitise() error
}

// FileUpload represents a file upload by a user.
type FileUpload struct {
	// Data is the file data/content.
	Data multipart.File `json:"data" validate:"required"`

	// Header is the mime header information of the file.
	Header textproto.MIMEHeader `json:"header" validate:"required"`

	// Name is the file name.
	Name string `json:"name" validate:"required"`

	// Size is the size of the file.
	Size int64 `json:"size" validate:"required,gte=0"`
}

// ParseMimetype attempts to parse and return the mimetype of the file from its mime header.
func (f *FileUpload) ParseMimetype() (string, error) {
	mediaType, _, err := mime.ParseMediaType(f.Header.Get("Content-Type"))
	if err != nil {
		return "unknown", fmt.Errorf("cannot parse mime type of file %s: %w", f.Name, err)
	}
	return mediaType, nil
}

// File is an object to store an uploaded file.
type File interface {
	Valid() (bool, error)
	Load(data multipart.File, hdr *multipart.FileHeader) error
}

// DecodeForm will decode submitted form contents into the passed in type. It
// will perform validation of the type and will return the type and a boolean
// true if it is valid. If decoding the form submission fails, a non-nill error
// is returned.
func DecodeForm[T FormInput](req *http.Request) (T, bool, error) {
	var obj T
	// Parse form values in request.
	err := req.ParseForm()
	if err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	// Decode the form values.
	err = decoder.Decode(&obj, req.Form)
	if err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	// Sanitise the object.
	err = obj.Sanitise()
	if err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrSanitise, err)
	}
	// Validate the object.
	if ok, err := obj.Valid(); !ok {
		return obj, false, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	return obj, true, nil
}

// DecodeCustom will decode submitted form contents into the passed in type,
// using the passed function to decode url.Values into the defined type. It will also
// will perform validation of the type and will return the type and a boolean
// true if it is valid. If decoding the form submission fails, a non-nill error
// is returned.
func DecodeCustom[T FormInput](req *http.Request, decoderFunc func(params url.Values) (T, error)) (T, bool, error) {
	var obj T
	// Parse form values in request.
	err := req.ParseForm()
	if err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	// Call the custom decoder function.
	obj, err = decoderFunc(req.Form)
	if err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	// Sanitise the object.
	err = obj.Sanitise()
	if err != nil {
		return obj, false, fmt.Errorf("%w: %w", ErrSanitise, err)
	}
	// Validate the object.
	if ok, err := obj.Valid(); !ok {
		return obj, false, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	return obj, true, nil
}

// DecodeMultipartFile will the file represented by the given field in a multipart form
// submission. It will perform validation of the file and will return the file
// object and a boolean true if it is valid. If decoding fails, a non-nill error
// is returned.
func DecodeMultipartFile(req *http.Request, field string) (*FileUpload, error) {
	// Parse form values in request.
	err := req.ParseMultipartForm(defaultMaxSize)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	// Decode the form values.
	data, hdr, err := req.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	return &FileUpload{
		Data:   data,
		Name:   hdr.Filename,
		Header: hdr.Header,
		Size:   hdr.Size,
	}, nil
}

// DecodeMultipartValue will the file represented by the given field in a multipart form
// submission. It will perform validation of the file and will return the file
// object and a boolean true if it is valid. If decoding fails, a non-nill error
// is returned.
func DecodeMultipartValue(req *http.Request, field string) (string, error) {
	// Parse form values in request.
	err := req.ParseMultipartForm(defaultMaxSize)
	if err != nil {
		return "", errors.Join(ErrDecode, err)
	}
	// Decode the form values.
	return req.FormValue(field), nil
}

// DecodeRequest will decode a request body into the given type.
func DecodeRequest[T any](req *http.Request) (T, error) {
	var obj T
	err := json.NewDecoder(req.Body).Decode(&obj)
	if err != nil {
		return obj, errors.Join(ErrDecode, err)
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
func EncodeForm[T FormInput](obj T) (url.Values, error) {
	// Sanitise the object.
	err := obj.Sanitise()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSanitise, err)
	}
	// Validate the object.
	if ok, err := obj.Valid(); !ok {
		return nil, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	values, err := encoder.Encode(&obj)
	if err != nil {
		return nil, errors.Join(ErrEncode, err)
	}
	return values, nil
}
