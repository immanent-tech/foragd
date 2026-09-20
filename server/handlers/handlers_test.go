/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	log.Println("running main")
	if err := godotenv.Load("../../.env.development"); err != nil {
		// Not fatal — CI often sets real env vars instead of a .env file
		log.Println("no .env file found, using existing environment")
	}
	os.Exit(m.Run())
}
