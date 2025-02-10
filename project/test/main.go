// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/fogfish/opts"
)

// Configuration type
type Client struct {
	*Inherited
	host string
}

// Configuration option
var WithHost = opts.ForType[Client, string]()

// Factory creates configuration instance
func New(opt ...opts.Option[Client]) (*Client, error) {
	c := Client{}

	// apply configuration options to type
	if err := opts.Apply(&c, opt); err != nil {
		return nil, err
	}

	return &c, nil
}

type Inherited struct{ value string }

var WithValue = opts.ForType[Inherited, string]()

func NewInherited(opt ...opts.Option[Inherited]) (*Inherited, error) {
	c := Inherited{}

	// apply configuration options to type
	if err := opts.Apply(&c, opt); err != nil {
		return nil, err
	}

	return &c, nil
}

var WithInherited = opts.Use[Client](NewInherited)

func main() {
	c, err := New(WithHost("example.com"), WithInherited(WithValue("value")))
	if err != nil {
		panic(err)
	}

	spew.Dump(c)

	fmt.Printf("==> %+v\n", c)
}
