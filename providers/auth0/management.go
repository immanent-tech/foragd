// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"fmt"
	"net/http"
	"net/url"
)

func GenerateLogoutURL(req *http.Request) (string, error) {
	logoutUrl, err := url.Parse("https://" + auth0Config.Domain + "/v2/logout")
	if err != nil {
		return "", fmt.Errorf("unable to determine logout URL: %w", err)
	}
	returnTo, err := url.Parse("https://" + req.Host)
	if err != nil {
		return "", fmt.Errorf("unable to determine redirect URL: %w", err)
	}
	parameters := url.Values{}
	parameters.Add("returnTo", returnTo.String())
	parameters.Add("client_id", auth0Config.ClientID)
	logoutUrl.RawQuery = parameters.Encode()
	return logoutUrl.String(), nil
}
