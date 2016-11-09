package request

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

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

//func MakeReqFloat
func MakeReqFloat(lat, lon float64, zoom int, clientID, ID string) (reqInJSON *string, err error) {

	r := Req{
		ClientID: clientID,
		ID:       ID,
	}

	r.Lat = lat
	r.Lon = lon

	jsonReq, err := r.getLocationJSON()
	if err != nil {
		return nil, err
	}
	return &jsonReq, nil
}

// GenerateAddress generates an address across the Belarus
func GenerateAddress(r1 *rand.Rand) (float64, float64, int) {

	lat := r1.Float64()*0 + 500
	lon := r1.Float64()*8 + 26

	return lat, lon, 18
}
