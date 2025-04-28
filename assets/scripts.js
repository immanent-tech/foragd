// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import "./setup-htmx.js";
// hyperscript
import "./setup-hyperscript.js";
// themechange
import { themeChange } from "theme-change";
themeChange();
// fontawesome
import "./fontawesome/js/brands.js";
import "./fontawesome/js/solid.js";
import "./fontawesome/js/fontawesome.js";
import { parseISO, intlFormatDistance } from "date-fns";
export function formatTimestamp(dateStr) {
  console.log(dateStr);
  let dateValue = parseISO(dateStr, new Date());
  console.log(dateValue);
  const timeString = intlFormatDistance(dateValue, new Date());
  return timeString;
}
