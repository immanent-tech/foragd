/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package models

import (
	"encoding/gob"
	"errors"
	"fmt"
	"iter"
	"net/mail"
	"time"
)

func init() {
	gob.Register(ListFilters{})
	gob.Register(SearchRequest{})
	gob.Register(UserProfileResponse{})
	gob.Register(time.Time{})
	gob.Register(map[string]string{})
}

var ErrInvalidDateTimeFormat = errors.New("datetime is invalid")

// UnixEpoch is the time.Time value of Unix epoch.
var UnixEpoch = time.Unix(0, 0)

func Filter[T any](seq iter.Seq[T], pred func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if pred(v) && !yield(v) {
				return
			}
		}
	}
}

func Find[T any](seq iter.Seq[T], pred func(T) bool) (T, bool) {
	for v := range seq {
		if pred(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// ValidateDatetime will check whether a time.Time is not either the zero value or equal to the Unix epoch.
func ValidateDatetime(dt time.Time) (bool, error) {
	switch {
	case dt.IsZero():
		return false, fmt.Errorf("%w: is zero time value", ErrInvalidDateTimeFormat)
	case dt.Equal(UnixEpoch):
		return false, fmt.Errorf("%w: is unix epoch", ErrInvalidDateTimeFormat)
	default:
		return true, nil
	}
}

type Email interface {
	GetID() string
	Timestamp() time.Time
	GetSubject() string
	GetBody() string
	GetFrom() *mail.Address
}

type ItemSource interface {
	GetFeedID() FeedID
	GetMarkedReadAt() time.Time
	GetReadItems() []ItemID
	GetUnreadItems() []ItemID
	GetArticleFilters() *ArticleFilters
}
