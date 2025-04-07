// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import 'htmx.org'
import './setup-htmx.js'

// hyperscript
import _hyperscript from 'hyperscript.org'
_hyperscript.browserInit()

import { themeChange } from 'theme-change'
themeChange()
