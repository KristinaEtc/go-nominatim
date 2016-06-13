package main

import (
	"Nominatim/lib"
	"Nominatim/lib/utils/basic"
	"database/sql"
	"encoding/json"
	"flag"
	"github.com/go-stomp/stomp"
	_ "github.com/lib/pq"
	"github.com/ventu-io/slf"
	"github.com/ventu-io/slog"
	"os"
	"path/filepath"
)

var (
	serverAddr  = flag.String("server", "localhost:61613", "STOMP server endpoint")
	queueFormat = flag.String("qFormat", "/queue/", "queue format")
	queueName   = flag.String("queue", "/queue/nominatimRequest", "Destination queue")
	debugMode   = flag.Bool("debug", false, "Debug mode")

	configFile = flag.String("config", "../config.json", "config file for Nominatim DB")
	logfile    = flag.String("logfile", "worker.log", "filename for logs")
	login      = flag.String("login", "client1", "Login for authorization")
	passcode   = flag.String("pwd", "111", "Passcode for authorization")
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

const LogDir = "logs/"
const (
	errorFilename = "error.log"
	infoFilename  = "info.log"
	debugFilename = "debug.log"
)

var (
	bhDebug, bhInfo, bhError, bhDebugConsole *basic.Handler
	logfileInfo, logfileDebug, logfileError  *os.File
	lf                                       slog.LogFactory

	log slf.StructuredLogger
)

// Init loggers
func init() {

	bhDebug = basic.New(slf.LevelDebug)
	bhDebugConsole = basic.New(slf.LevelDebug)
	bhInfo = basic.New()
	bhError = basic.New(slf.LevelError)

	// optionally define the format (this here is the default one)
	bhInfo.SetTemplate("{{.Time}} [\033[{{.Color}}m{{.Level}}\033[0m] {{.Context}}{{if .Caller}} ({{.Caller}}){{end}}: {{.Message}}{{if .Error}} (\033[31merror: {{.Error}}\033[0m){{end}} {{.Fields}}")

	// TODO: create directory in /var/log, if in linux:
	// if runtime.GOOS == "linux" {
	os.Mkdir("."+string(filepath.Separator)+LogDir, 0766)

	// interestings with err: if not initialize err before,
	// how can i use global logfileInfo?
	var err error
	logfileInfo, err = os.OpenFile(LogDir+infoFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf("Could not open/create %s logfile", LogDir+infoFilename)
	}

	logfileDebug, err = os.OpenFile(LogDir+debugFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf("Could not open/create logfile", LogDir+debugFilename)
	}

	logfileError, err = os.OpenFile(LogDir+errorFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf("Could not open/create logfile", LogDir+errorFilename)
	}

	if *debugMode == true {
		bhDebugConsole.SetWriter(os.Stdout)
	}

	bhDebug.SetWriter(logfileDebug)
	bhInfo.SetWriter(logfileInfo)
	bhError.SetWriter(logfileError)

	lf = slog.New()
	lf.SetLevel(slf.LevelDebug) //lf.SetLevel(slf.LevelDebug, "app.package1", "app.package2")
	lf.SetEntryHandlers(bhInfo, bhError, bhDebug)

	if *debugMode == true {
		lf.SetEntryHandlers(bhInfo, bhError, bhDebug, bhDebugConsole)
	} else {
		lf.SetEntryHandlers(bhInfo, bhError, bhDebug)
	}

	// make this into the one used by all the libraries
	slf.Set(lf)

	log = slf.WithContext("main-worker.go")
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
			log.Errorf("error: ", err)
		}
		p.config = configuration
	}

	log.Debug("db configurate done")

}

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
		" host=" + p.config.Host +
		" user=" + p.config.User +
		" password=" + p.config.Password

	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		log.Panic(err.Error())
	}
	defer reverseGeocode.Close()

	connSubsc, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server: %s", err.Error())
		return
	}

	connSend, err := stomp.Dial("tcp", *serverAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server: %s", err.Error())
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

func main() {

	defer logfileInfo.Close()
	defer logfileDebug.Close()
	defer logfileError.Close()

	flag.Parse()
	flag.Parsed()

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(*login, *passcode),
		stomp.ConnOpt.Host("/"),
	}

	params := Params{}
	params.configurateDB()
	subscribed := make(chan bool)
	go requestLoop(subscribed, &params)

	<-stop
}
