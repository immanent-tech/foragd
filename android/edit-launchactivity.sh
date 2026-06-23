# Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
# SPDX-License-Identifier: 	AGPL-3.0-or-later

#!/usr/bin/env bash

FILE="app/src/main/java/app/foragd/twa/LauncherActivity.java"

grep -q "utm_source" "$FILE" || sed -i 's|return uri;|\
        if (uri.getQueryParameter("utm_source") != null) {\
            return uri;\
        }\
        return uri.buildUpon().appendQueryParameter("utm_source", "twa").build();|' "$FILE"
