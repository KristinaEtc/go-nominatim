package main

//important: must execute first; do not move
import (
	"encoding/json"
	"fmt"
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

type QueueOptConf struct {
	QueueName      string
	QueuePriorName string
	ResentFullReq  bool
}

type DiagnosticsConf struct {
	CoeffEMA  float64
	TopicName string
	TimeOut   int // in seconds
	MachineID string
}

type ConnectionConf struct {
	ServerAddr     string
	ServerUser     string
	ServerPassword string
	QueueFormat    string
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
	ConnConf    ConnectionConf
	DiagnConf   DiagnosticsConf
	QueueConf   QueueOptConf
	NominatimDB NominatimConf
}

var globalOpt = ConfFile{

	ConnConf: ConnectionConf{
		ServerAddr:     "localhost:61615",
		QueueFormat:    "/queue/",
		ServerUser:     "",
		ServerPassword: "",
	},
	QueueConf: QueueOptConf{
		ResentFullReq:  true,
		QueueName:      "/queue/nominatimRequest",
		QueuePriorName: "/queue/nominatimPriorRequest",
	},
	DiagnConf: DiagnosticsConf{
		CoeffEMA:  0.5,
		TopicName: "/topic/worker.status",
		TimeOut:   5,
		MachineID: "defaultName",
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

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("", ""),
	stomp.ConnOpt.Host("127.0.0.1"),
}

type Req struct {
	Lat      float64
	Lon      float64
	Zoom     int
	ClientID string
	ID       interface{}
}

type Params struct {
	clientReq      Req
	format         string
	addressDetails bool
	machineId      string
	//sqlOpenStr     string
	//config         NominatimConf
	//db             *sql.DB
}

type ErrorResponse struct {
	Type    string
	Message string
}

// monitoringData is a struct which will be sended to a spetial topic
// for diagnostics
type monitoringData struct {
	LastReconnect string
	EMA           float64 // exponential moving average
	ConnTryings   int
	ErrResp       int
	SuccResp      int
	Reqs          int
	Err           int
	LastErr       string
	MachineID     string
	MachineAddr   string
}

//--------------------------------------------------------------------------

func createErrResponse(err error) []byte {
	respJSON := ErrorResponse{Type: "error", Message: err.Error()}

	bytes, err := json.Marshal(respJSON)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil
	}
	return bytes
}

func (p *Params) locationSearch(rawMsg []byte, geocode *Nominatim.ReverseGeocode) ([]byte, *string, bool, error) {

	log.Debugf("Got request: %s", rawMsg)

	err := p.addCoordinatesToStruct(rawMsg)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return nil, nil, false, err
	}
	log.Debug("addCoordinatesToStruct done")

	whoToSent := p.clientReq.ClientID

	if globalOpt.QueueConf.ResentFullReq == true {

		var msgMapTemplate interface{}
		err := json.Unmarshal(rawMsg, &msgMapTemplate)
		if err != nil {
			log.Panic("err != nil")
		}
		geocode.SetFullReq(msgMapTemplate)
	}

	place, err := p.getLocationFromNominatim(geocode)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return createErrResponse(err), &whoToSent, true, nil
	}

	log.Debug("getLocationFromNominatim done")
	placeJSON, err := getLocationJSON(*place)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return createErrResponse(err), &whoToSent, true, nil
	}

	log.Debug("getLocationJSON done")

	log.Debugf("Client:%s ID:%d", p.clientReq.ClientID, p.clientReq.ID)

	return placeJSON, &whoToSent, false, nil

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
	reverseGeocode.SetMachineID(p.machineId)

	place, err := reverseGeocode.Lookup(globalOpt.QueueConf.ResentFullReq)
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

func requestLoop(subscribed chan bool, timeToMonitoring chan monitoringData) {
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

	connSubsc, err := stomp.Dial("tcp", globalOpt.ConnConf.ServerAddr, options...)
	if err != nil {
		log.WithCaller(slf.CallerShort).Errorf("cannot connect to server (connSubsc): %s", err.Error())
		return
	}

	connSend, err := stomp.Dial("tcp", globalOpt.ConnConf.ServerAddr, options...)
	if err != nil {
		log.WithCaller(slf.CallerShort).Errorf("cannot connect to server (connSend): %s", err.Error())
		return
	}

	sub, err := connSubsc.Subscribe(globalOpt.QueueConf.QueueName, stomp.AckAuto)
	if err != nil {
		log.WithCaller(slf.CallerShort).Errorf("cannot subscribe to %s: %s",
			globalOpt.QueueConf.QueueName, err.Error())
		return
	}

	subPrior, err := connSubsc.Subscribe(globalOpt.QueueConf.QueuePriorName, stomp.AckAuto)
	if err != nil {
		log.WithCaller(slf.CallerShort).Errorf("cannot subscribe to %s: %s",
			globalOpt.QueueConf.QueueName, err.Error())
		return
	}

	close(subscribed)

	timeStr := fmt.Sprintf("%s", time.Now().Format(time.RFC3339))
	var data = monitoringData{
		LastReconnect: timeStr,
		ConnTryings:   0,
		ErrResp:       0,
		SuccResp:      0,
		EMA:           0.0,
		Err:           0,
		LastErr:       "",
		MachineAddr:   connSend.GetConnInfo(),
		MachineID:     globalOpt.DiagnConf.MachineID,
	}

	timer := time.NewTimer(time.Duration(globalOpt.DiagnConf.TimeOut) * time.Second)

	var ok bool
	var msg *stomp.Message
	//	var queque string

	for {

		select {
		case msg, ok = <-subPrior.C:
			//log.Info("got prior")
			//queque = globalOpt.QueueConf.QueuePriorName
			break
		case <-timer.C:
			timeToMonitoring <- data
		default:
			select {
			case msg, ok = <-sub.C:
				//	queque = globalOpt.QueueConf.QueueName
				//log.Info("got usual")
				break
			default:
				continue
			}
		}

		start := time.Now()

		if !ok {
			log.Warn("!ok")
			data.ConnTryings++
			data.LastErr = "msg, ok = <-sub.C; !ok"
			continue
		}

		reqJSON := msg.Body
		var p Params
		p.machineId = connSend.GetConnInfo()
		replyJSON, whoToSent, errResp, err := p.locationSearch(reqJSON, reverseGeocode)
		if err != nil {
			log.WithCaller(slf.CallerShort).Errorf("Error: locationSearch %s", err.Error())
			data.Err++
			data.LastErr = err.Error()
			continue
		}
		if errResp == true {
			data.ErrResp++
		} else {
			data.SuccResp++
		}
		if replyJSON == nil {
			log.Warn("MUST NOT ENTERED")
			data.Err++
			data.LastErr = "replyJSON == nil"
			continue
		}

		//log.Debugf("whoToSent %s", *whoToSent)
		//log.Debugf("i'm sending: %s\n", string(replyJSON[:]))
		log.Debugf("i'm sending to %s\n", globalOpt.ConnConf.QueueFormat+*whoToSent)

		err = connSend.Send(globalOpt.ConnConf.QueueFormat+*whoToSent, "text/plain",
			[]byte(replyJSON), nil...)
		if err != nil {
			data.MachineAddr = connSend.GetConnInfo()
			data.Err++
			log.WithCaller(slf.CallerShort).Errorf("Failed to send to server %s", err)
			time.Sleep(time.Second)
			data.LastErr = err.Error()
			continue
		}

		elapsed := float64(time.Since(start)) / 1000.0 / 1000.0 / 1000.0
		//data.EMA = (data.EMA + elapsed) / 2
		data.EMA = (1-globalOpt.DiagnConf.CoeffEMA)*data.EMA + globalOpt.DiagnConf.CoeffEMA*elapsed
		log.Debug("Sending finished")
	}
}

func sendStatus(timeToMonitoring chan monitoringData) {

	defer func() {
		stop <- true
	}()

	connSend, err := stomp.Dial("tcp", globalOpt.ConnConf.ServerAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %s", err.Error())
		return
	}
	for {
		select {
		case data := <-timeToMonitoring:
			//V TIMERE
			b, err := json.Marshal(data)
			if err != nil {
				log.Error(err.Error())
				continue
			}

			err = connSend.Send(globalOpt.DiagnConf.TopicName, "text/json", b, nil...)
			if err != nil {
				log.Errorf("Error %s", err.Error())
				continue
			}

		}
	}
}

func initOptions() {
	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(globalOpt.ConnConf.ServerUser, globalOpt.ConnConf.ServerPassword),
		stomp.ConnOpt.Host(globalOpt.ConnConf.ServerAddr),
	}
}

func main() {

	log = slf.WithContext("go-stomp-nominatim.go")

	//params := Params{}
	config.ReadGlobalConfig(&globalOpt, "go-stomp-nominatim options")
	initOptions()

	subscribed := make(chan bool)
	timeout := make(chan monitoringData)

	log.Error("----------------------------------------------")

	log.Infof("BuildDate=%s\n", BuildDate)
	log.Infof("GitCommit=%s\n", GitCommit)
	log.Infof("GitBranch=%s\n", GitBranch)
	log.Infof("GitState=%s\n", GitState)
	log.Infof("GitSummary=%s\n", GitSummary)
	log.Infof("VERSION=%s\n", Version)

	log.Info("Starting working...")
	go requestLoop(subscribed, timeout)
	<-subscribed

	go sendStatus(timeout)

	<-stop
	<-stop
}
