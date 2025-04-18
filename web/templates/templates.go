// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
)

type Writer struct {
	resp htmx.Response
	res  http.ResponseWriter
}

func (w *Writer) Write(req *http.Request, component Component) error {
	if err := w.resp.RenderTempl(req.Context(), w.res, component.Render(req)); err != nil {
		return err
	}
	return nil
}

func NewWriter(res http.ResponseWriter) *Writer {
	return &Writer{
		resp: htmx.NewResponse(),
		res:  res,
	}
}

type Component interface {
	Render(req *http.Request) templ.Component
}
