// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/alexedwards/scs/gormstore"
	"github.com/alexedwards/scs/v2"
	"github.com/knadh/koanf/v2"
	sloggorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
)

const (
	LevelDebug = slog.Level(-4)
)

var (
	ErrConnectFailed = errors.New("postgres connection failed")
	ErrInvalidConfig = errors.New("invalid postgres config")
	ErrSetupFailed   = errors.New("postgres setup failed")

	schemas = [...]any{&models.User{}, &models.Topic{}, &models.Feed{}, &models.ReadItems{}, &models.SavedItems{}, &models.Notification{}, &models.Subscription{}}
)

type Env interface {
	Name() string
	GetStr(key string) string
	Logger() *slog.Logger
}

type Client struct {
	db           *gorm.DB
	sessionStore *gormstore.GORMStore
	logger       *slog.Logger
}

func (c *Client) NewSessionStorage() scs.Store {
	return c.sessionStore
}

func Connect(ctx context.Context, config *koanf.Koanf) (*Client, error) {
	settings := getSettings(config)

	logger := logging.FromContext(ctx).With(slog.String("platform", "postgres"))

	gormLogger := sloggorm.New(
		sloggorm.WithHandler(logger.With(slog.String("component", "gorm")).Handler()),
		sloggorm.SetLogLevel(sloggorm.DefaultLogType, LevelDebug),
	)

	db, err := gorm.Open(postgres.Open(settings.DSN), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}

	logger.Debug("Performing auto-migration...")

	err = db.AutoMigrate(schemas[:]...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSetupFailed, err)
	}

	logger.Debug("Auto-migration complete!")

	store, err := gormstore.New(db)
	if err != nil {
		return nil, fmt.Errorf("unable to create session store in DB: %w", err)
	}

	return &Client{db: db, sessionStore: store, logger: logger}, nil
}
