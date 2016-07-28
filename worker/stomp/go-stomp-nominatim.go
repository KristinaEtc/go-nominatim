package main

//important: do not move
import _ "github.com/KristinaEtc/slflog"

import (
	"Nominatim/lib"
	"database/sql"
	"encoding/json"
	"flag"
	"os"

	"github.com/go-stomp/stomp"
	"github.com/kardianos/osext"

	_ "github.com/lib/pq"
	"github.com/ventu-io/slf"
)

/*go build -ldflags "-X github.com/KristinaEtc/slflog.configLogFile=/usr/share/go-stomp-nominatim/go-stomp-nominatim.logconfig
-X go-stomp-worker.configFile=/usr/share/go-stomp-nominatim/go-stomp-nominatim.config" go-stomp-nominatim.go */
var configFile string

var log = slf.WithContext("go-stomp-nominatim.go")

var (
	serverAddr  = flag.String("server", "localhost:61614", "STOMP server endpoint")
	queueFormat = flag.String("qFormat", "/queue/", "queue format")
	queueName   = flag.String("queue", "/queue/nominatimRequest", "Destination queue")

	configFileNominatim = flag.String("conf", getPathToConfig(), "config file for Nominatim DB")
	login               = flag.String("login", "client1", "Login for authorization")
	passcode            = flag.String("pwd", "111", "Passcode for authorization")
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
	config         ConfigDB
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
		log.WithCaller(slf.CallerShort).Errorf("cannot subscribe to %s: %s", *queueName, err.Error())
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

		err = connSend.Send(*queueFormat+*whoToSent, "text/plain",
			[]byte(reqJSON), nil...)
		if err != nil {
			log.WithCaller(slf.CallerShort).Errorf("Failed to send to server %s", err)
			return
		}

		log.Debug("Sending finished")
	}
}

func (p *Params) configurateDB() {

	file, err := os.Open(*configFileNominatim)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error("No configurate file")
	} else {

		defer file.Close()
		decoder := json.NewDecoder(file)
		configuration := ConfigDB{}

		err := decoder.Decode(&configuration)
		if err != nil {
			log.WithCaller(slf.CallerShort).Errorf("error: %v", err.Error())
		}
		p.config = configuration
	}

	log.Debug("db configurate done")
}

func main() {

	flag.Parse()
	log = slf.WithContext("go-stomp-nominatim.go")

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(*login, *passcode),
		stomp.ConnOpt.Host("/"),
	}

	params := Params{}
	params.configurateDB()
	log.Debug(params.config.User)
	subscribed := make(chan bool)
	log.Error("--------------------------new connection---------------------")
	log.WithCaller(slf.CallerShort).Info("starting working...")
	go requestLoop(subscribed, &params)

	<-stop
}

func getPathToConfig() string {

	var path = configFile
	log.Debug(path)

	// path to config was setted by a linker value
	if path != "" {
		exist, err := exists(path)
		if err != nil {
			log.WithCaller(slf.CallerShort).Errorf("Error: wrong configure file from linker value %s: %s\n", path, err.Error())
			path = ""
		} else if exist != true {
			log.WithCaller(slf.CallerShort).Errorf("Error: Configure file from linker value %s: does not exist\n", path)
			path = ""
		}
	}

	// no path from a linker value or wrong linker value; searching where a binary is situated
	if path == "" {
		pathTemp, err := osext.Executable()
		if err != nil {
			log.WithCaller(slf.CallerShort).Errorf("Error: could not get a path to binary file for getting configfile: %s\n", err.Error())
		} else {
			path = pathTemp + ".config"
		}
	}
	log.WithCaller(slf.CallerShort).Infof("Configfile that will be used: [%s]", path)
	return path
}

// Exists returns whether the given file or directory exists or not.
func exists(path string) (bool, error) {

	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
