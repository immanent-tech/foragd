/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package cache

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/immanent-tech/go-base/config"

	"github.com/immanent-tech/foragd/providers/google/gcs"
)

var NewItemsCache = sync.OnceValues(func() (ObjectCache, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}

	switch appCfg.GetAppEnvironment() {
	case config.EnvProduction:
		bucketName := os.Getenv("FORAGD_SERVER_BUCKET")
		var err error
		itemsCache, err := gcs.Connect(context.Background(), bucketName, "articles")
		if err != nil {
			return nil, fmt.Errorf("connect to gcs:%s: %w", "articles", err)
		}
		return itemsCache, nil
	default:
		var err error
		itemsCache, err := newDirCache("articles")
		if err != nil {
			return nil, fmt.Errorf("create items dir cache: %w", err)
		}
		return itemsCache, nil
	}
})
