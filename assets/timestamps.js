// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later
// date-fns
import { parseISO } from "date-fns";
export function formatTimestamp(dateStr) {
  console.log(dateStr);
  let dateValue = parseISO(dateStr, new Date());
  console.log(dateValue);
  // const timeString = intlFormatDistance(dateValue.Date, new Date());
  // return timeString;
}
