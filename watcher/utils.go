package main

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/ventu-io/slf"
)

//--------
// utilits, u know
func containsInSlice(s []string, e string) (bool, int) {
	for key, a := range s {
		if a == e {
			return true, key
		}
	}
	return false, 0
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

func parseID(msg []byte) (*NecessaryFields, error) {

	var data NecessaryFields

	if err := json.Unmarshal(msg, &data); err != nil {
		log.Errorf("Could not get parse request: %s", err.Error())
		return nil, err
	}
	if data.ID == "" {
		log.Warnf("Wrong message %s", string(msg))
		return nil, errors.New("No utc value in request")
	}
	return &data, nil
}

func getJSON(data WatcherData) ([]byte, error) {
	data.StartTime = time.Now().Format(time.RFC3339)

	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil, err
	}
	log.Debugf("dataJSON=%v", string(dataJSON))

	//debug mode
	//os.Exit(1)

	return dataJSON, nil
}
