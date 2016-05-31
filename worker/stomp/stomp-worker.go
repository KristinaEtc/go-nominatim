package main

import (
	"Nominatim/lib"
	"database/sql"
	"encoding/json"
	"flag"
	l4g "github.com/alecthomas/log4go"
	"github.com/go-stomp/stomp"
	_ "github.com/lib/pq"
	"os"
)

var log l4g.Logger

var (
	serverAddr = flag.String("server", "localhost:61613", "STOMP server endpoint")
	configFile = flag.String("config", "../config.json", "config file for Nominatim DB")
	logfile    = flag.String("logfile", "worker.log", "filename for logs")
	queueName  = flag.String("queue", "/queue/nominatimRequest", "Destination queue")
	debugMode  = flag.Bool("debug", false, "Debug mode")
	stop       = make(chan bool)
)

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
			log.Error("error: ", err)
		}
		p.config = configuration
	}
	if *debugMode == true {
		log.Debug("db configurate done")
	}

}

func (p *Params) locationSearch(rawMsg []byte, geocode *Nominatim.ReverseGeocode) ([]byte, *string, error) {

	if *debugMode == true {
		log.Debug("Got request: %s", rawMsg)
	}

	err := p.addCoordinatesToStruct(rawMsg)
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}
	if *debugMode == true {
		log.Debug("addCoordinatesToStruct done")
	}

	place, err := p.getLocationFromNominatim(geocode)
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}

	log.Debug("getLocationFromNominatim done")
	placeJSON, err := getLocationJSON(*place)
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}

	if *debugMode == true {
		log.Debug("getLocationJSON done")
	}

	whoToSent := p.clientReq.ClientID
	if *debugMode == true {
		log.Info("%s %d", p.clientReq.ClientID, p.clientReq.ID)
	}

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
		log.Error(err)
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

	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		log.Critical(err)
		os.Exit(1)
	}
	defer reverseGeocode.Close()

	connSubsc, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		println("cannot connect to server", err.Error())
		return
	}

	connSend, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		println("cannot connect to server", err.Error())
		return
	}

	sub, err := connSubsc.Subscribe(*queueName, stomp.AckAuto)
	if err != nil {
		println("cannot subscribe to", *queueName, err.Error())
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

		if *debugMode == true {
			log.Debug("whoToSent %s", *whoToSent)
		}

		err = connSend.Send("/queue/"+*whoToSent, "text/plain",
			[]byte(reqJSON), nil...)
		if err != nil {
			println("failed to send to server", err)
			return
		}

		if *debugMode == true {
			log.Debug("Sending finished")
		}
	}
}

func main() {

	log = l4g.NewLogger()

	log.AddFilter("stdout", l4g.INFO, l4g.NewConsoleLogWriter())
	log.AddFilter("file", l4g.DEBUG, l4g.NewFileLogWriter(*logfile, false))

	flag.Parse()
	flag.Parsed()

	params := Params{}
	params.configurateDB()
	subscribed := make(chan bool)
	go requestLoop(subscribed, &params)

	<-stop
}
