package request

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/KristinaEtc/go-nominatim/lib"
	"github.com/ventu-io/slf"
)

const pwdCurr string = "Nominatim/lib/utils/request"

var log = slf.WithContext("watcher.go")

type Req struct {
	Nominatim.ReverseGeocodeRequest
	ClientID string
	ID       interface{}
}

func (r *Req) getLocationJSON() (string, error) {

	dataJSON, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(dataJSON), nil
}

//func MakeReq(parameters, clientID string, ID int, log l4g.Logger) (reqInJSON *string, err error) {
func MakeReq(parameters, clientID string, ID string) (reqInJSON *string, err error) {

	locSlice := strings.Split(parameters, ",")
	if len(locSlice) < 3 {
		return nil, fmt.Errorf("invalid string %s", parameters)
	}
	r := Req{}
	r.Lat, err = strconv.ParseFloat(locSlice[0], 32)

	if err != nil {
		return nil, err
	}
	r.Lon, err = strconv.ParseFloat(locSlice[1], 32)
	if err != nil {
		return nil, err
	}
	r.Zoom, err = strconv.Atoi(locSlice[2])
	if err != nil {
		return nil, err
	}
	r.ClientID = clientID

	r.ID = ID

	jsonReq, err := r.getLocationJSON()
	if err != nil {
		return nil, err
	}
	return &jsonReq, nil
}

// GenerateAddress generates an address across the Belarus
func GenerateAddress() string {

	//return "53.8225923,27.277391,18"

	var s1 = rand.NewSource(time.Now().UnixNano())
	var r1 = rand.New(s1)

	lat := r1.Float64()*5 + 52
	long := r1.Float64()*9 + 24

	log.Debugf("%f-%f", lat, long)

	latStr := strconv.FormatFloat(lat, 'f', -1, 32)
	longStr := strconv.FormatFloat(long, 'f', -1, 32)

	address := latStr + "," + longStr + "," + "18"

	return address
}
