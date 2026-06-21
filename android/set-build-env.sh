# Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
# SPDX-License-Identifier: 	AGPL-3.0-or-later

#!/usr/bin/env bash

sed \
  -e "s|__ANDROID_KEYSTORE_PATH__|${ANDROID_KEYSTORE_PATH}|g" \
  -e "s|__ANDROID_KEYSTORE_ALIAS__|${ANDROID_KEYSTORE_ALIAS}|g" \
  -e "s|__ANDROID_KEYSTORE_FINGERPRINT__|${ANDROID_KEYSTORE_FINGERPRINT}|g" \
  -e "s|__FORAGD_DOMAIN__|${FORAGD_DOMAIN}|g" \
  -e "s|__FORAGD_BASEURL__|${FORAGD_BASEURL}|g" \
  twa-manifest.json > twa-manifest-generated.json
