package main

//important: must execute first; do not move
import _ "github.com/KristinaEtc/slflog"

import (
	"database/sql"
	"encoding/json"

	"github.com/KristinaEtc/go-nominatim/lib"
	"github.com/KristinaEtc/utils"
	"github.com/go-stomp/stomp"

	_ "github.com/lib/pq"
	"github.com/ventu-io/slf"
)

var log = slf.WithContext("go-stomp-nominatim.go")

/*-------------------------
	Config option structures
-------------------------*/

var configFile string

// GlobalConf is a struct with global options,
// like server address and queue format, etc.
type GlobalConf struct {
	ServerAddr     string
	ServerUser     string
	ServerPassword string
	QueueFormat    string
	QueueName      string
}

// NominatimConf options
type NominatimConf struct {
	User     string
	Password string
	Host     string
	DBname   string
}

// ConfFile is a file with all program options
type ConfFile struct {
	Global      GlobalConf
	NominatimDB NominatimConf
}

var globalOpt = ConfFile{
	Global: GlobalConf{
		ServerAddr:     "localhost:61614",
		QueueFormat:    "/queue/",
		QueueName:      "/queue/nominatimRequest",
		ServerUser:     "",
		ServerPassword: "",
	},
	NominatimDB: NominatimConf{
		DBname:   "nominatim",
		Host:     "localhost",
		User:     "geocode1",
		Password: "_geocode1#",
	},
}

/*-------------------------
	Geolocation and
	request's structures
-------------------------*/

var stop = make(chan bool)

/*var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("guest", "guest"),
	stomp.ConnOpt.Host("/"),
}*/

type Req struct {
	Lat      float64
	Lon      float64
	Zoom     int
	ClientID string
	ID       int
}

type Params struct {
	clientReq      Req
	format         string
	addressDetails bool
	sqlOpenStr     string
	config         NominatimConf
	db             *sql.DB
}

func (p *Params) locationSearch(rawMsg []byte, geocode *Nominatim.ReverseGeocode) ([]byte, *string, error) {

	log.Debugf("Got request: %s", rawMsg)

	err := p.addCoordinatesToStruct(rawMsg)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil, nil, err
	}
	log.Debug("addCoordinatesToStruct done")

	place, err := p.getLocationFromNominatim(geocode)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil, nil, err
	}

	log.Debug("getLocationFromNominatim done")
	placeJSON, err := getLocationJSON(*place)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil, nil, err
	}

	log.Debug("getLocationJSON done")

	whoToSent := p.clientReq.ClientID

	log.Infof("%s %d", p.clientReq.ClientID, p.clientReq.ID)

	return placeJSON, &whoToSent, nil

}

func (p *Params) addCoordinatesToStruct(data []byte) error {

	location := Req{}
	if err := json.Unmarshal(data, &location); err != nil {
		return err
	}
	p.clientReq = location

	return nil
}

func (p *Params) getLocationFromNominatim(reverseGeocode *Nominatim.ReverseGeocode) (*Nominatim.DataWithoutDetails, error) {

	//oReverseGeocode.SetLanguagePreference()
	reverseGeocode.SetIncludeAddressDetails(p.addressDetails)
	reverseGeocode.SetZoom(p.clientReq.Zoom)
	reverseGeocode.SetLocation(p.clientReq.Lat, p.clientReq.Lon)
	place, err := reverseGeocode.Lookup()
	if err != nil {
		return nil, err
	}
	place.ID = p.clientReq.ID

	return place, nil
}

func getLocationJSON(data Nominatim.DataWithoutDetails) ([]byte, error) {

	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil, err
	}
	return dataJSON, nil
}

func requestLoop(subscribed chan bool, p *Params) {
	defer func() {
		stop <- true
	}()

	sqlOpenStr := "dbname=" + p.config.DBname +
		" host=" + p.config.Host +
		" user=" + p.config.User +
		" password=" + p.config.Password

	log.Error(sqlOpenStr)

	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
	}
	defer reverseGeocode.Close()

	var options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(globalOpt.Global.ServerUser, globalOpt.Global.ServerPassword),
		stomp.ConnOpt.Host(globalOpt.Global.ServerAddr),
	}

	connSubsc, err := stomp.Dial("tcp", globalOpt.Global.ServerAddr, options...)
	if err != nil {
		log.WithCaller(slf.CallerShort).Errorf("cannot connect to server (connSubsc): %s", err.Error())
		return
	}

	connSend, err := stomp.Dial("tcp", globalOpt.Global.ServerAddr, options...)
	if err != nil {
		log.WithCaller(slf.CallerShort).Errorf("cannot connect to server (connSend): %s", err.Error())
		return
	}

	sub, err := connSubsc.Subscribe(globalOpt.Global.QueueName, stomp.AckAuto)
	if err != nil {
		log.WithCaller(slf.CallerShort).Errorf("cannot subscribe to %s: %s",
			globalOpt.Global.QueueName, err.Error())
		return
	}
	close(subscribed)

	for {
		msg := <-sub.C
		if msg == nil {
			log.WithCaller(slf.CallerShort).Error("Got nil message")
			return
		}

		reqJSON := msg.Body

		reqJSON, whoToSent, err := p.locationSearch(msg.Body, reverseGeocode)
		if err != nil {
			log.WithCaller(slf.CallerShort).Error("Error: converting request to json")
			return
		}

		log.Debugf("whoToSent %s", *whoToSent)

		err = connSend.Send(globalOpt.Global.QueueFormat+*whoToSent, "text/plain",
			[]byte(reqJSON), nil...)
		if err != nil {
			log.WithCaller(slf.CallerShort).Errorf("Failed to send to server %s", err)
			return
		}

		log.Debug("Sending finished")
	}
}

func (p *Params) configurateFromConfFile() {
	utils.GetFromGlobalConf(&globalOpt, "go-stomp-nominatim options")
	p.config = globalOpt.NominatimDB

	log.Debug("db configurate done")
}

func main() {

	log = slf.WithContext("go-stomp-nominatim.go")

	params := Params{}
	params.configurateFromConfFile()
	log.Debug(params.config.User)

	subscribed := make(chan bool)
	log.Error("----------------------------------------------")
	log.Info("Starting working...")
	go requestLoop(subscribed, &params)

	<-stop
}
