package main

//important: must execute first; do not move
import (
	"encoding/json"
	"time"

	"github.com/KristinaEtc/config"
	"github.com/KristinaEtc/go-nominatim/lib"
	_ "github.com/KristinaEtc/slflog"
	"github.com/go-stomp/stomp"

	_ "github.com/lib/pq"
	"github.com/ventu-io/slf"
)

var log = slf.WithContext("go-stomp-nominatim.go")

var (
	// These fields are populated by govvv
	BuildDate  string
	GitCommit  string
	GitBranch  string
	GitState   string
	GitSummary string
	Version    string
)

/*-------------------------
	Config option structures
-------------------------*/

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
	//sqlOpenStr     string
	//config         NominatimConf
	//db             *sql.DB
}

type ErrorResponse struct {
	Type    string
	Message string
}

func createErrResponse(err error) []byte {
	respJSON := ErrorResponse{Type: "error", Message: err.Error()}

	bytes, err := json.Marshal(respJSON)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil
	}
	return bytes
}

func (p *Params) locationSearch(rawMsg []byte, geocode *Nominatim.ReverseGeocode) ([]byte, *string, error) {

	log.Debugf("Got request: %s", rawMsg)

	err := p.addCoordinatesToStruct(rawMsg)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil, nil, err
	}
	log.Debug("addCoordinatesToStruct done")

	whoToSent := p.clientReq.ClientID

	place, err := p.getLocationFromNominatim(geocode)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return createErrResponse(err), &whoToSent, nil
	}

	log.Debug("getLocationFromNominatim done")
	placeJSON, err := getLocationJSON(*place)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return createErrResponse(err), &whoToSent, nil
	}

	log.Debug("getLocationJSON done")

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

func requestLoop(subscribed chan bool) {
	defer func() {
		stop <- true
	}()

	sqlOpenStr := "dbname=" + globalOpt.NominatimDB.DBname +
		" host=" + globalOpt.NominatimDB.Host +
		" user=" + globalOpt.NominatimDB.User +
		" password=" + globalOpt.NominatimDB.Password

	log.WithCaller(slf.CallerShort).Debugf("sqlOpenStr=%s", sqlOpenStr)

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
		msg, err := sub.Read()
		if err != nil {
			log.Errorf("error get from server %s", err.Error())
			time.Sleep(time.Second * 2)
			continue
		}

		reqJSON := msg.Body
		var p Params
		replyJSON, whoToSent, err := p.locationSearch(reqJSON, reverseGeocode)
		if err != nil {
			log.WithCaller(slf.CallerShort).Errorf("Error: locationSearch %s", err.Error())
			continue
		}
		if replyJSON == nil {
			continue
		}

		log.Debugf("whoToSent %s", *whoToSent)

		err = connSend.Send(globalOpt.Global.QueueFormat+*whoToSent, "text/plain",
			[]byte(replyJSON), nil...)
		if err != nil {
			log.WithCaller(slf.CallerShort).Errorf("Failed to send to server %s", err)
			time.Sleep(time.Second)
			continue
		}

		log.Debug("Sending finished")
	}
}

func main() {

	log = slf.WithContext("go-stomp-nominatim.go")

	//params := Params{}
	config.ReadGlobalConfig(&globalOpt, "go-stomp-nominatim options")

	subscribed := make(chan bool)
	log.Error("----------------------------------------------")

	log.Infof("BuildDate=%s\n", BuildDate)
	log.Infof("GitCommit=%s\n", GitCommit)
	log.Infof("GitBranch=%s\n", GitBranch)
	log.Infof("GitState=%s\n", GitState)
	log.Infof("GitSummary=%s\n", GitSummary)
	log.Infof("VERSION=%s\n", Version)

	log.Info("Starting working...")
	go requestLoop(subscribed)

	<-stop
}
