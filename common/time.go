package common

import (
	"strings"
	"time"
)

const DateFormat = "2006-01-02"

func NowWithLocation(tz string) (*time.Time, error) {
	loc, err := time.LoadLocation(strings.TrimSpace(tz))
	if err != nil {
		return nil, err
	}

	n := time.Now().In(loc)

	return &n, nil
}

func ToDay(tz string) (string, error) {
	n, err := NowWithLocation(tz)
	if err != nil {
		return "", err
	}

	return n.Format(DateFormat), nil
}
