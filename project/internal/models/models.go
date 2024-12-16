// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"

	"github.com/mmcdole/gofeed"
)

var ErrParseFeed = errors.New("could not parse feed")

var parser *gofeed.Parser

func init() {
	parser = gofeed.NewParser()
	parser.UserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0"
}
