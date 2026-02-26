// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gcp

import (
	"fmt"
	"io"

	"github.com/immanent-tech/go-syndication/client"
)

func queryMetadataServer(path string) (string, error) {
	metadataServerURL := "http://metadata.google.internal"
	httpClient := client.LoadHTTPClient()
	resp, err := httpClient.R().
		SetHeader("Metadata-Flavor", "Google").
		SetDoNotParseResponse(true).
		Get(metadataServerURL + path)
	if err != nil {
		return "", fmt.Errorf("query metadata server: %w", err)
	}
	if resp.IsError() {
		return "", fmt.Errorf("query metdata server: %s", resp.Error())
	}
	defer resp.RawBody().Close()

	body, err := io.ReadAll(resp.RawBody())
	if err != nil {
		return "", fmt.Errorf("read metadata server response: %w", err)
	}
	return string(body), nil
}
