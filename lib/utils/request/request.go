package request

import (
	"encoding/json"
	//l4g "github.com/alecthomas/log4go"
	"strconv"
	"strings"
)

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
	r := Req{}
	r.Lat, err = strconv.ParseFloat(locSlice[0], 32)

	if err != nil {
		//	log.Error(err)
		return nil, err
	}
	r.Lon, err = strconv.ParseFloat(locSlice[1], 32)
	if err != nil {
		//log.Error(err)
		return nil, err
	}
	r.Zoom, err = strconv.Atoi(locSlice[2])
	if err != nil {
		//log.Error(err)
		return nil, err
	}
	r.ClientID = clientID

	r.ID = ID

	jsonReq, err := r.getLocationJSON()
	if err != nil {
		//	log.Error(err)
		return nil, err
	}
	return &jsonReq, nil
}
