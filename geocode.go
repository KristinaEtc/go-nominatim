package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ventu-io/slf"

	"github.com/KristinaEtc/go-nominatim/lib"
)

//Request - json geocode request
type Request struct {
	Nominatim.ReverseGeocodeRequest
	ClientID string
	ID       interface{}
}

//Response - json geocode response
type Response struct {
	Nominatim.ReverseGeocodeResponse
	MachineID string
	TimeReq   string
	ID        interface{}
	FullReq   interface{}
}

// ErrorResponse - struct for error answer to geocoding request
type ErrorResponse struct {
	Type    string
	Message string
	ID      interface{}
}

func locationSearch(rawMsg []byte, geocode Nominatim.ReverseGeocode, workerID string) ([]byte, *string, bool, error) {

	if len(rawMsg) == 0 {
		return nil, nil, false, fmt.Errorf("%s", "Empty body request")
	}

	if globalOpt.IsDebug {
		log.Debugf("Request: %s", rawMsg)
	}

	request, err := parseRequest(rawMsg)
	if err != nil {
		return nil, nil, false, err
	}

	response := Response{}
	if globalOpt.QueueConf.ResentFullReq == true {

		var msgMapTemplate interface{}
		err := json.Unmarshal(rawMsg, &msgMapTemplate)
		if err != nil {
			log.Panic("err != nil")
		}
		response.FullReq = msgMapTemplate
	}

	place, err := getLocationFromNominatim(geocode, &request.ReverseGeocodeRequest)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return createErrResponse(err, request.ID), &request.ClientID, true, nil
	}
	response.ReverseGeocodeResponse = *place
	response.ID = request.ID
	response.TimeReq = time.Now().Format(time.RFC3339)
	response.MachineID = workerID

	placeJSON, err := getLocationJSON(&response)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return createErrResponse(err, request.ID), &request.ClientID, true, nil
	}
	if globalOpt.IsDebug {
		log.Debugf("Client:%s ID:%d placeJSON:%s", request.ClientID, request.ID, string(placeJSON))
	}

	return placeJSON, &request.ClientID, false, nil

}

func parseRequest(data []byte) (*Request, error) {

	request := Request{}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, err
	}

	return &request, nil
}

func getLocationFromNominatim(reverseGeocode Nominatim.ReverseGeocode, request *Nominatim.ReverseGeocodeRequest) (*Nominatim.ReverseGeocodeResponse, error) {

	//oReverseGeocode.SetLanguagePreference()
	//reverseGeocode.SetIncludeAddressDetails(p.addressDetails)
	//reverseGeocode.SetZoom(p.clientReq.Zoom)
	//reverseGeocode.SetLocation(p.clientReq.Lat, p.clientReq.Lon)
	//reverseGeocode.SetMachineID(p.machineID)

	place, err := reverseGeocode.Lookup(request)
	if err != nil {

		return nil, err
	}

	//place.ID = p.clientReq.ID

	return place, nil
}

func getLocationJSON(response *Response) ([]byte, error) {

	dataJSON, err := json.Marshal(response)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil, err
	}
	return dataJSON, nil
}

func createErrResponse(err error, id interface{}) []byte {
	respJSON := ErrorResponse{Type: "error", Message: err.Error(), ID: id}

	bytes, err := json.Marshal(respJSON)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil
	}
	return bytes
}
