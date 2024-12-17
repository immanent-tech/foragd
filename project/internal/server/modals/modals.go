// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package modals

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	components "github.com/joshuar/go-templ-daisyui"
)

func CommandModal(req *http.Request, res http.ResponseWriter, content templ.Component) error {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, components.ModalTempl("command-modal", content)); err != nil {
		return fmt.Errorf("failed to render command modal: %w", err)
	}

	return nil
}
