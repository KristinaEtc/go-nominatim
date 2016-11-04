package Nominatim

import "fmt"

type reverseGeocodeMock struct {
}

func (r *reverseGeocodeMock) Close() error {
	return nil
}

//NewReverseGeocodeMock - create mock reverse geocoding
func NewReverseGeocodeMock() ReverseGeocode {
	return &reverseGeocodeMock{}
}

func (r *reverseGeocodeMock) Lookup(request *ReverseGeocodeRequest) (*ReverseGeocodeResponse, error) {
	return &ReverseGeocodeResponse{
		Lat: fmt.Sprintf("%f", request.Lat),
		Lon: fmt.Sprintf("%f", request.Lon),
	}, nil
}
