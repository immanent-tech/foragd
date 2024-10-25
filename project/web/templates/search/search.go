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

//go:generate stringer -type=CommandID -linecomment -output search_generated.go
package search

import (
	"strings"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/web/templates/commands"
)

const (
	AddCommand      CommandID = iota // add
	SettingsCommand                  // settings
	ProfileCommand                   // profile
)

type CommandID int

type Command struct {
	Button components.Button
	ID     CommandID
}

type CommandList map[CommandID]Command

func (c CommandList) IsACommand(text string) CommandID {
	// All commands are lowercase.
	text = strings.ToLower(text)
	for cmd := range c {
		if text == cmd.String() || strings.HasPrefix(text, cmd.String()+" ") {
			return cmd
		}
	}

	return -1
}

type SearchAPI any

const (
	CommandResult SearchResultType = iota
	FeedItemResult
	FeedResult
	TopicResult
)

type SearchResultType int

type SearchResults map[SearchResultType]any

func GenerateSearchResults(_ SearchAPI, terms string) SearchResults {
	searchCommands := CommandList{
		AddCommand:      {ID: AddCommand, Button: commands.AddCommand},
		SettingsCommand: {ID: SettingsCommand, Button: commands.SettingsCommand},
		ProfileCommand:  {ID: ProfileCommand, Button: commands.ProfileCommand},
	}

	results := make(SearchResults)

	if cmd := searchCommands.IsACommand(terms); cmd != -1 {
		results[CommandResult] = searchCommands[cmd].Button
	}

	return results
}

func NewSearchInput() components.SearchInput {
	return components.SearchInput{
		Validator: "/home/search",
		Input: components.Input{
			Placeholder: "Search anything...",
			Type:        components.InputTypeSearch,
		},
	}
}
