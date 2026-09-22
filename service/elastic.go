/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/immanent-tech/go-base/config"

	"github.com/immanent-tech/foragd/models"
)

const (
	FeedsIndex         Index = "feeds"
	ItemsIndex         Index = "items"
	FavoritesIndex     Index = "favorites"
	UsersIndex         Index = "users"
	SubscriptionsIndex Index = "subscriptions"
	ScheduleIndex      Index = "scheduler"
	SessionsIndex      Index = "sessions"

	// indexWriteSuffix is the suffix appended to indices that are used for write (indexing) operations.
	indexWriteSuffix = "_rw"
	// indexReadSuffix is the suffix appended to indices that are used for read (search, get) operations.
	indexReadSuffix = "_ro"
)

type Index string

type ElasticService struct {
	environment config.Environment
}

var LoadElasticService = sync.OnceValues(func() (*ElasticService, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}
	return &ElasticService{
		environment: appCfg.GetAppEnvironment(),
	}, nil
})

func (s *ElasticService) GetIndexRO(name Index) string {
	return string(name) + "_" + s.environment.String() + indexReadSuffix
}

func (s *ElasticService) GetIndexRW(name Index) string {
	return string(name) + "_" + s.environment.String() + indexWriteSuffix
}

// ElasticsearchToAPIError will extract and wrap a types.ElasticsearchError from the given error, in an APIError
// containing its pertinent information. If the given error does not contain types.ElasticsearchError, the given error
// is wrapped in a generic APIError is created.
func ElasticsearchToAPIError(err error) error {
	if esErr, ok := errors.AsType[*types.ElasticsearchError](err); ok {
		var str strings.Builder

		str.WriteString(*esErr.ErrorCause.Reason)
		str.WriteString(" (")
		str.WriteString(esErr.ErrorCause.Type)
		str.WriteString(")")
		if esErr.ErrorCause.RootCause != nil {
			str.WriteString(" reason: ")
			str.WriteString(*esErr.ErrorCause.CausedBy.Reason)
		}

		return &models.APIError{
			InternalError: fmt.Errorf("%s", str.String()),
			StatusCode:    esErr.Status,
		}
	}
	return &models.APIError{
		InternalError: fmt.Errorf("%w: %w", models.ErrInvalidAPIResult, err),
		StatusCode:    http.StatusInternalServerError,
	}
}
