// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// InternalServerError handles errors related to non-specific internal server functionality failures.
func InternalServerError(res http.ResponseWriter, req *http.Request, err error) {
	msg := models.NewMessage("Internal server error", models.MessageStatusError, models.WithError(err))
	slogctx.FromCtx(req.Context()).Error("Cannot display content.",
		slog.Any("error", msg))
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, partials.ShowNotification(msg)); err != nil {
		http.Error(res, msg.String(), http.StatusInternalServerError)
	}
}

type HTTPError struct {
	error
	Code int
}

func NewError(code int, message string) *HTTPError {
	return &HTTPError{
		error: errors.New(message),
		Code:  code,
	}
}

func NotFound(message string) *HTTPError {
	return NewError(http.StatusNotFound, message)
}

// InternalServerError handles errors related to non-specific internal server functionality failures.
func HandleError(res http.ResponseWriter, req *http.Request, err *HTTPError) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		http.Error(res, err.Error(), httpErr.Code)
		slogctx.FromCtx(req.Context()).Error("Error occurred.",
			slog.Any("error", err))
	} else {
		// Default to 500
		http.Error(res, err.Error(), http.StatusInternalServerError)
		slogctx.FromCtx(req.Context()).Error("Internal server error.",
			slog.Any("error", err))
	}
}
