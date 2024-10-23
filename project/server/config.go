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

package server

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func (s *Server) saveConfig() error {
	data, err := s.Config.Marshal(toml.Parser())
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}

	if err := os.WriteFile(filepath.Join(ConfigPath, ConfigFile), data, fs.FileMode(ConfigPerms)); err != nil {
		return fmt.Errorf("could not write config file: %w", err)
	}

	return nil
}

func (s *Server) loadConfig() error {
	s.Config = koanf.New(".")
	// Load config from file.
	if err := s.Config.Load(file.Provider(filepath.Join(ConfigPath, ConfigFile)), toml.Parser()); err != nil {
		return fmt.Errorf("error loading config file: %w", err)
	}

	// Merge environment variables with config.
	if err := s.Config.Load(env.Provider(envPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, envPrefix)), "_", ".", -1)
	}), nil); err != nil {
		return fmt.Errorf("error merging file and environment: %w", err)
	}

	return nil
}
