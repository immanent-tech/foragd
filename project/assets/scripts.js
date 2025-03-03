// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import "htmx.org";
import "./setup-htmx.js";
// htmx server side events extension
import "htmx.org/dist/ext/sse";
// htmx websocket extension
import "htmx.org/dist/ext/ws";
// htmx morphdom-swap extension
import "htmx.org/dist/ext/morphdom-swap";

// hyperscript
import _hyperscript from "hyperscript.org";
_hyperscript.browserInit();

import { themeChange } from "theme-change";
themeChange();
