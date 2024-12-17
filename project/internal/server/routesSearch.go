// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"log/slog"
	"net/http"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
)

func (s Server) Search(res http.ResponseWriter, req *http.Request) {
	logger := s.Logger.With(slog.String("handler", "Search"))

	ctx := logging.ToContext(req.Context(), logger)

	handlers.Search(res, req.WithContext(ctx))
}
