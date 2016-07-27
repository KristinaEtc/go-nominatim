package main

import _ "github.com/KristinaEtc/slflog"

import (
	"Nominatim/lib"
	"database/sql"
	"encoding/json"
	"flag"
	"os"

	"github.com/go-stomp/stomp"
	_ "github.com/lib/pq"
	"github.com/ventu-io/slf"
)

var (
	serverAddr  = flag.String("server", "localhost:61614", "STOMP server endpoint")
	queueFormat = flag.String("qFormat", "/queue/", "queue format")
	queueName   = flag.String("queue", "/queue/nominatimRequest", "Destination queue")

	configFile = flag.String("config", "/home/k/work/go/src/Nominatim/worker/config.json", "config file for Nominatim DB")
	//configFile = flag.String("config", "/opt/go-stomp/go-stomp-nominatim/config.json", "config file for Nominatim DB")
	login    = flag.String("login", "client1", "Login for authorization")
	passcode = flag.String("pwd", "111", "Passcode for authorization")

	logPath  = flag.String("logpath", "logs", "path to logfiles")
	logLevel = flag.String("loglevel", "WARN", "IFOO, DEBUG, ERROR, WARN, PANIC, FATAL - loglevel for stderr")
)

var stop = make(chan bool)

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("guest", "guest"),
	stomp.ConnOpt.Host("/"),
}

type ConfigDB struct {
	DBname, Host, User, Password string
}

type Req struct {
	Lat      float64 `json: Lat`
	Lon      float64 `json:Lon`
	Zoom     int     `json:Zoom`
	ClientID string  `json:ClientID`
	ID       int     `json:ID`
}

type Params struct {
	clientReq      Req
	format         string
	addressDetails bool
	sqlOpenStr     string
	config         ConfigDB
	db             *sql.DB
}

var log slf.StructuredLogger

func (p *Params) locationSearch(rawMsg []byte, geocode *Nominatim.ReverseGeocode) ([]byte, *string, error) {

	log.Debugf("Got request: %s", rawMsg)

	err := p.addCoordinatesToStruct(rawMsg)
	if err != nil {
		log.Error(err.Error())
		return nil, nil, err
	}
	log.Debug("addCoordinatesToStruct done")

	place, err := p.getLocationFromNominatim(geocode)
	if err != nil {
		log.Error(err.Error())
		return nil, nil, err
	}

	log.Debug("getLocationFromNominatim done")
	placeJSON, err := getLocationJSON(*place)
	if err != nil {
		log.Error(err.Error())
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
		log.Error(err.Error())
		return nil, err
	}
	return dataJSON, nil
}

func requestLoop(subscribed chan bool, p *Params) {
	defer func() {
		stop <- true
	}()

	sqlOpenStr := "dbname=" + p.config.DBname +
		" host=" + p.config.Host
	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		log.WithCaller(slf.CallerShort).Panic(err.Error())
	}
	defer reverseGeocode.Close()

	connSubsc, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.WithCaller(slf.CallerShort).Errorf("cannot connect to server (connSubsc): %s", err.Error())
		return
	}

	connSend, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.WithCaller(slf.CallerShort).Errorf("cannot connect to server (connSend): %s", err.Error())
		return
	}

	sub, err := connSubsc.Subscribe(*queueName, stomp.AckAuto)
	if err != nil {
		log.Errorf("cannot subscribe to %s: %s", *queueName, err.Error())
		return
	}
	close(subscribed)

	for {
		msg := <-sub.C
		if msg == nil {
			log.Error("Got nil message")
			return
		}

		reqJSON := msg.Body

		reqJSON, whoToSent, err := p.locationSearch(msg.Body, reverseGeocode)
		if err != nil {
			log.Error("Error: converting request to json")
			return
		}

		log.Debugf("whoToSent %s", *whoToSent)

		err = connSend.Send(*queueFormat+*whoToSent, "text/plain",
			[]byte(reqJSON), nil...)
		if err != nil {
			log.Errorf("Failed to send to server %s", err)
			return
		}

		log.Debug("Sending finished")
	}
}

func (p *Params) configurateDB() {

	file, err := os.Open(*configFile)
	if err != nil {
		log.Error("No configurate file")
	} else {

		defer file.Close()
		decoder := json.NewDecoder(file)
		configuration := ConfigDB{}

		err := decoder.Decode(&configuration)
		if err != nil {
			log.Errorf("error: %v", err.Error())
		}
		p.config = configuration
	}

	log.Debug("db configurate done")
}

func main() {

	flag.Parse()
	log = slf.WithContext("go-stompd-worker.go")

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(*login, *passcode),
		stomp.ConnOpt.Host("/"),
	}

	params := Params{}
	params.configurateDB()
	log.Debug(params.config.User)
	subscribed := make(chan bool)
	log.Info("starting working...")
	go requestLoop(subscribed, &params)

	<-stop
}
