// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package htmx

// ResponseHandling configures how htmx handles different HTTP response codes. When htmx receives a response it will
// iterate in order over the htmx.config.responseHandling array and test if the code property of a given object, when
// treated as a Regular Expression, matches the current response. If an entry does match the current response code, it
// will be used to determine if and how the response will be processed.
//
// https://htmx.org/docs/#response-handling
type ResponseHandling struct {
	// Code is a String representing a regular expression that will be tested against response codes.
	Code string `json:"code"`
	// Swap is true if the response should be swapped into the DOM, false otherwise.
	Swap bool `json:"swap"`
	// Error is true if htmx should treat this response as an error.
	Error bool `json:"error,omitempty,omitzero"`
	// IgnoreTitle is true if htmx should ignore title tags in the response.
	IgnoreTitle bool `json:"ignoreTitle,omitempty,omitzero"`
	// Select is a CSS selector to use to select content from the response.
	Select string `json:"select,omitempty,omitzero"`
	// Target is a CSS selector specifying an alternative target for the response.
	Target string `json:"target,omitempty,omitzero"`
	// SwapOverride is an alternative swap mechanism for the response.
	SwapOverride string `json:"swapOverride,omitempty,omitzero"`
}

// Config defines the htmx config options.
//
// https://htmx.org/docs/#config
type Config struct {
	// AllowNestedOOBSwaps configures whether to process OOB swaps on elements that are nested within the main response
	// element.
	AllowNestedOOBSwaps bool `json:"allowNestedOobSwaps"`
	// InlineStyleNonce configures a none to be added to inline styles created by htmx.
	InlineStyleNonce string `json:"inlineStyleNonce,omitempty"`
	// InlineStyleNonce configures a none to be added to inline scripts created by htmx.
	InlineScriptNonce string `json:"inlineScriptNonce,omitempty"`
	// IncludeIndicatorStyles configures whether htmx will dynamically add indicator styles inline for requests.
	IncludeIndicatorStyles bool `json:"includeIndicatorStyles"`
	// HistoryRestoreAsHxRequest configures whether to treat history cache miss full page reload requests as a
	// “HX-Request” by returning this response header. This should always be disabled when using HX-Request header to
	// optionally return partial responses
	HistoryRestoreAsHxRequest bool `json:"historyRestoreAsHxRequest"`
	// GlobalViewTransitions configures whether htmx will use the View Transition API when swapping in new content.
	GlobalViewTransitions bool `json:"globalViewTransitions"`
	// ResponseHandling configures how to handle various HTTP response codes.
	ResponseHandling []*ResponseHandling `json:"responseHandling,omitzero"`
}

// HXLocationRequest defines the value of the HX-Location header.
//
// https://htmx.org/headers/hx-location/
type HXLocationRequest struct {
	// The URL path.
	Path string `json:"path"`
	//  The source element of the request.
	Source string `json:"source,omitzero"`
	// An event that “triggered” the request.
	Event string `json:"event,omitzero"`
	// A JS callback that will handle the response HTML.
	Handler string `json:"handler,omitzero"`
	// The target to swap the response into.
	Target string `json:"target,omitzero"`
	// How the response will be swapped in relative to the target.
	Swap string `json:"swap,omitzero"`
	// Values to submit with the request.
	Values map[string]any `json:"values,omitzero"`
	// Headers to submit with the request.
	Headers map[string]string `json:"headers,omitzero"`
	// Allows you to select the content you want swapped from a response.
	Select string `json:"select,omitzero"`
	// Set to 'false' or a path string to prevent or override the URL pushed to browser location history
	Push string `json:"push,omitzero"`
	// A path string to replace the URL in the browser location history
	Replace string `json:"replace,omitzero"`
}
