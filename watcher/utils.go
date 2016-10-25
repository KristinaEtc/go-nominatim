package main

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

//--------
// utilits, u know
func containsInSlice(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func parseUnixTime(s string) (*time.Time, error) {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}
	tm := time.Unix(i, 0)
	log.Debugf("parseUnixTime=%v\n", tm)
	return &tm, nil

}

func getTimeFromID(id string) (*time.Time, error) {
	wasSended := strings.SplitAfter(id, ",")
	if len(wasSended) < 2 {
		return nil, errors.New("Wrong id: no time parametr")
	}
	t, err := parseUnixTime(wasSended[1])
	if err != nil {
		return nil, err

	}
	return t, nil
}

func createReqBody(ids []string, numOdReq int) (*string, error) {
	return nil, nil
}
