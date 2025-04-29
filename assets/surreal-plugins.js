// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

import { parseISO, intlFormatDistance } from "date-fns";
// pluginTimestamps contains methods for manipulating timestamps client-side, for showing relative timestamps for e.g.
export function pluginTimestamps(e) {
  function formatTimestamp(e) {
    let dateStr = e.attribute("value");
    let dateValue = parseISO(dateStr, new Date());
    const timeString = intlFormatDistance(dateValue, new Date());
    return timeString;
  }
  e.formatTimestamp = () => {
    return formatTimestamp(e);
  };
}
