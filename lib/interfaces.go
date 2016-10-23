package Nominatim

import (
	"io"
)

//ReverseGeocode - reverse geocoding API
type ReverseGeocode interface {
	io.Closer

	Lookup(r *ReverseGeocodeRequest) (*ReverseGeocodeResponse, error)
}

//ReverseGeocodeRequest - request data
type ReverseGeocodeRequest struct {
	Lat            float64
	Lon            float64
	Zoom           int
	IncludeDetails bool
}

//ClientID       string  `json:"ClientID"`
//ID             int     `json:"ID"`

//ReverseGeocodeResponse - response data
type ReverseGeocodeResponse struct {
	PlaceID     string `json:"Place_id"`
	OsmType     string `json:"Osm_type"`
	OsmID       string `json:"Osm_id"`
	Lat         string
	Lon         string
	Langaddress string
	//Details!
}
