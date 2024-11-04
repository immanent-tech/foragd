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

package renderers

import (
	"fmt"
	"net/http"

	"github.com/angelofallars/htmx-go"
	components "github.com/joshuar/go-templ-daisyui"
)

func FormInput(req *http.Request, res http.ResponseWriter, input components.Input) error {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, input.Show()); err != nil {
		return fmt.Errorf("failed to render input: %w", err)
	}

	return nil
}
