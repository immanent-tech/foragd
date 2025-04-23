// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/models/feeds"
)

func main() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	results := feeds.NewFeedsFromURLs(ctx, "https://feeds.feedburner.com/9to5Google")
	spew.Dump(results)
	refeed := results[0].Feed

	// for result := range slices.Values(results) {
	// 	if result.Err != nil {
	// 		slog.Error("unable to parse", slog.String("url", result.URL), slog.Any("error", result.Err))
	// 	} else {
	// 		feed := result.Feed
	// enc := json.NewEncoder(os.Stdout)
	// enc.SetIndent("", "    ")
	// if err := enc.Encode(feed); err != nil {
	// 	panic(err)
	// }
	// data, err = json.Marshal(feed)
	// if err != nil {
	// 	panic(err)
	// }
	// refeed := &models.Feed{}
	// err = json.Unmarshal(data, refeed)
	// if err != nil {
	// 	panic(err)
	// }
	// enc := json.NewEncoder(os.Stdout)
	// enc.SetIndent("", "    ")
	// if err := enc.Encode(refeed); err != nil {
	// 	panic(err)
	// }
	fmt.Fprintf(os.Stdout, "%s: %s\n", refeed.GetTitle(), refeed.GetDescription())
	fmt.Fprintf(os.Stdout, "source url: %s link: %s\n", refeed.GetSourceURL(), refeed.GetLink())
	if image := refeed.GetImage(); image != nil {
		fmt.Fprintf(os.Stdout, "image: %s (%s)\n", refeed.GetImage().String(), refeed.GetImage().URL())
	}
	fmt.Fprintf(os.Stdout, "authors: %v | contributors: %v\n", refeed.GetAuthors(), refeed.GetContributors())
	fmt.Fprintf(os.Stdout, "published: %s updated: %s\n", refeed.GetPublishedDate(), refeed.GetUpdatedDate())
	for category := range slices.Values(refeed.GetCategories()) {
		fmt.Fprintf(os.Stdout, "%s\t", category)
	}
	fmt.Fprintf(os.Stdout, "\n\n")

	// items, err := models.GetFeedItems(ctx, refeed.GetID(), refeed.GetSourceURL())
	// if err != nil {
	// 	panic(err)
	// }

	// for item := range slices.Values(items) {
	// 	enc := json.NewEncoder(os.Stdout)
	// 	enc.SetIndent("", "    ")
	// 	if err := enc.Encode(item); err != nil {
	// 		panic(err)
	// 	}
	// 	break
	// }

	os.Exit(0)
	for entry := range slices.Values(refeed.GetItems()) {
		spew.Dump(entry)
		data, err := json.Marshal(entry)
		if err != nil {
			panic(err)
		}
		reEntry := &feeds.Item{}
		err = json.Unmarshal(data, reEntry)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(os.Stdout, "title: %s (%s)\n", reEntry.GetTitle(), reEntry.GetID())
		fmt.Fprintf(os.Stdout, "description: %s\n", reEntry.GetDescription())
		fmt.Fprintf(os.Stdout, "link: %s\n", reEntry.GetLink())
		if image := reEntry.GetImage(); image != nil {
			fmt.Fprintf(os.Stdout, "image: %s (%s)\n", reEntry.GetImage().String(), reEntry.GetImage().URL())
		}
		fmt.Fprintf(os.Stdout, "authors: %v | contributors: %v\n", reEntry.GetAuthors(), reEntry.GetContributors())
		fmt.Fprintf(os.Stdout, "published: %s updated: %s\n", reEntry.GetPublishedDate(), reEntry.GetUpdatedDate())
		for category := range slices.Values(reEntry.GetCategories()) {
			fmt.Fprintf(os.Stdout, "%s\t", category)
		}
		fmt.Fprintf(os.Stdout, "\n")
		fmt.Fprintf(os.Stdout, "content:\n %s\n", reEntry.GetContent())
		fmt.Fprintf(os.Stdout, "%T\n", reEntry)
		fmt.Fprintf(os.Stdout, "\n\n")
	}
	// 	}
	// }

	os.Exit(0)

	// fmt.Fprintf(os.Stdout, "%s: %s\n", feed.GetTitle(), feed.GetDescription())
	// fmt.Fprintf(os.Stdout, "source url: %s link: %s\n", feed.GetSourceURL(), feed.GetLink())
	// if image := feed.GetImage(); image != nil {
	// 	fmt.Fprintf(os.Stdout, "image: %s (%s)\n", feed.GetImage().String(), feed.GetImage().URL())
	// }
	// fmt.Fprintf(os.Stdout, "authors: %v | contributors: %v\n", feed.GetAuthors(), feed.GetContributors())
	// fmt.Fprintf(os.Stdout, "published: %s updated: %s\n", feed.GetPublishedDate(), feed.GetUpdatedDate())
	// for category := range slices.Values(feed.GetCategories()) {
	// 	fmt.Fprintf(os.Stdout, "%s\t", category.String())
	// }
	// fmt.Fprintf(os.Stdout, "\n\n")

	// spew.Dump(feed, err)
	// b, err := xml.MarshalIndent(feed, "", "    ")
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(string(b))

	// data, _ := os.ReadFile("/workspace/project/pkg/feeds/test/rlxa.atom")
	// feed, err := types.Decode[*atom.Feed]("rss", data)
	// spew.Dump(feed, err)

	// data, _ := os.ReadFile("/workspace/project/pkg/feeds/test/export.opml")
	// opml, _ := opml.New(data)
	// // spew.Dump(opml.ExtractRSS(), len(opml.ExtractRSS()))
	// spew.Dump(opml)

	// data, _ = json.Marshal(feed)
	// var prettyJSON bytes.Buffer
	// error := json.Indent(&prettyJSON, data, "", "\t")
	// if error != nil {
	// 	log.Println("JSON parse error: ", error)
	// 	return
	// }

	// log.Println("CSP Violation:", string(prettyJSON.Bytes()))

	// wantXML := `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><check><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>golang.org</domain:name><domain:name>go.dev</domain:name></domain:check></check></command></epp>`
	// wantValue := &EPP{
	// 	Command: &Command{
	// 		Check: &Check{
	// 			DomainCheck: &DomainCheck{
	// 				DomainNames: []string{
	// 					"golang.org",
	// 					"go.dev",
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	// gotXML, err := xml.Marshal(wantValue)
	// if err != nil {
	// 	panic(err)
	// }
	// if string(gotXML) != wantXML {
	// 	fmt.Printf("xml.Marshal(%#v)\n got: %s\nwant: %s\n", wantValue, gotXML, wantXML)
	// }

	// gotValue := &EPP{}
	// err = xml.Unmarshal([]byte(wantXML), gotValue)
	// if err != nil {
	// 	panic(err)
	// }

	// // spew.Dump(gotValue)
	// if !reflect.DeepEqual(gotValue, wantValue) {
	// 	got, _ := json.Marshal(gotValue)
	// 	want, _ := json.Marshal(wantValue)
	// 	fmt.Printf("xml.Marshal(%s)\n got: %s\nwant: %s\n", wantXML, got, want)
	// }

	// spew.Dump(gotValue)
}
