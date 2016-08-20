package request

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const pwdCurr string = "Nominatim/lib/utils/request"

type Req struct {
	Lat      float64 `json:Lat`
	Lon      float64 `json:Lon`
	Zoom     int     `json:Zoom`
	ClientID string  `json:ClientID`
	ID       int     `json:ID`
}

func (r *Req) getLocationJSON() (string, error) {

	dataJSON, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(dataJSON), nil
}

//func MakeReq(parameters, clientID string, ID int, log l4g.Logger) (reqInJSON *string, err error) {
func MakeReq(parameters, clientID string, ID int) (reqInJSON *string, err error) {

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
