// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/logging"
	"github.com/joshuar/go-feed-me/templates/partials"
)

func Settings(res http.ResponseWriter, req *http.Request) {
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, partials.SettingsModal()); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("Cannot render add item modal.",
			slog.String("handler", "DisplayAddItemModal"),
			slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}
