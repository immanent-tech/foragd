/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package models

import (
	"strconv"

	"github.com/zeebo/xxh3"
)

func (i *ImportStatus) GetID() string {
	return i.JobID
}

func (i *ImportResult) GetID() string {
	return "import_result_" + strconv.FormatUint(xxh3.Hash([]byte(i.JobID+i.URL)), 10)
}
