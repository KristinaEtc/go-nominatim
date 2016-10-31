package main

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	//"github.com/ventu-io/slf"
)

// NecessaryFields storesrows of json request, that we want to get
// and that should necessary be
type NecessaryFields struct {
	ID string `json:"id"`
}

//--------
// utilits, u know
func validateID(s []string, e string) (bool, int) {
	for key, a := range s {
		if a == e {
			return true, key
		}
	}
	return false, 0
}

func parseUnixTime(s string) (*time.Time, error) {
	log.Debugf("parseUnixTimeBefore=[%s]", s)
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}

	tm := time.Unix(i, 0)
	log.Debugf("parseUnixTime=[%v]", tm)
	return &tm, nil

}

func parseID(id string) (int, int64, error) {
	wasSended := strings.SplitAfter(id, ",")
	if len(wasSended) < 2 {
		return 0, 0, errors.New("Wrong id: no time parametr")
	}
	/*t, err := parseUnixTime(wasSended[1])
	if err != nil {
		return 0,0, err
	}*/
	t, err := strconv.ParseInt(wasSended[1], 10, 64)
	if err != nil {
		return 0, 0, err
	}

	numStr := strings.TrimSuffix(wasSended[0], ",")
	//log.Debugf("numStr=%s", numStr)
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, t, err
	}
	//num, err := strconv.Atoi(strings.TrimPrefix(wasSended[0], ","))
	//if err != nil {
	//	return 0, t, err
	//}
	return num, t, nil
}

func getID(msg []byte) (int, int64, error) {

	var data NecessaryFields

	if err := json.Unmarshal(msg, &data); err != nil {
		log.Errorf("Could not parse response: %s", err.Error())
		return 0, 0, err
	}
	if data.ID == "" {
		log.Warnf("Messsage with empty ID: %s", string(msg))
		return 0, 0, errors.New("No utc value in request")
	}
	return parseID(data.ID)
}
