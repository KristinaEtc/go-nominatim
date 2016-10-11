package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	//important: must execute first; do not move
	_ "github.com/KristinaEtc/slflog"

	"github.com/KristinaEtc/config"
	"github.com/KristinaEtc/go-nominatim/lib"
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

var uuid string

type QueueOptConf struct {
	QueueName      string
	QueuePriorName string
	ResentFullReq  bool
}

type DiagnosticsConf struct {
	CoeffEMA      float64
	TopicName     string
	TimeOut       int // in seconds
	MachineID     string
	CoeffSeverity float64
}

type ConnectionConf struct {
	ServerAddr     string
	ServerUser     string
	ServerPassword string
	QueueFormat    string
	HeartBeatError int
	HeartBeat      int
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
	Name        string
	DirWithUUID string
	ConnConf    ConnectionConf
	DiagnConf   DiagnosticsConf
	QueueConf   QueueOptConf
	NominatimDB NominatimConf
}

var globalOpt = ConfFile{
	Name:        "name",
	DirWithUUID: ".go-stomp-nominatim/",

	ConnConf: ConnectionConf{
		ServerAddr:     "localhost:61615",
		QueueFormat:    "/queue/",
		ServerUser:     "",
		ServerPassword: "",
		HeartBeat:      30,
		HeartBeatError: 15,
	},
	QueueConf: QueueOptConf{
		ResentFullReq:  true,
		QueueName:      "/queue/nominatimRequest",
		QueuePriorName: "/queue/nominatimPriorRequest",
	},
	DiagnConf: DiagnosticsConf{
		CoeffEMA:      0.1,
		TopicName:     "/topic/worker.status",
		TimeOut:       5,
		MachineID:     "defaultName",
		CoeffSeverity: 2,
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

// monitoringData is a struct which will be sended to a special topic
// for diagnostics
type monitoringData struct {
	StartTime      string
	CurrentTime    string `json:"utc"`
	LastReconnect  string
	AverageRate    float64 // exponential moving average
	ReconnectCount int
	ErrResp        int64
	SuccResp       int64
	Reqs           int64
	ErrorCount     int64
	LastError      string
	MachineAddr    string  `json:"ip"`
	Severity       float64 `json:"severity"`

	Type string `json:"type"`
	Id   string `json:"id"`
	Name string `json:"name"`

	Subtype      string `json:"subtype"`
	Subsystem    string `json:"subsystem"`
	ComputerName string `json:"computer"`
	UserName     string `json:"user"`
	ProcessName  string `json:"process"`
	Version      string `json:"version"`
	Pid          int    `json:"pid"`
	//Tid          int    `json:tid`
	Message string `json:"message"`

	RequestRate     float64 `json:"request_rate`
	ErrorRate       float64 `json:"error_rate"`
	ErrorRespRate   float64 `json:"error_resp_rate"`
	SuccessRespRate float64 `json:"success_resp_rate"`
}

//--------------------------------------------------------------------------

func main() {

	log = slf.WithContext("go-stomp-nominatim.go")

	//params := Params{}
	config.ReadGlobalConfig(&globalOpt, "go-stomp-nominatim options")
	initOptions()

	uuid = config.GetUUID(globalOpt.DirWithUUID)

	subscribed := make(chan bool)
	timeout := make(chan []byte)

	log.Error("----------------------------------------------")

	log.Infof("BuildDate=%s\n", BuildDate)
	log.Infof("GitCommit=%s\n", GitCommit)
	log.Infof("GitBranch=%s\n", GitBranch)
	log.Infof("GitState=%s\n", GitState)
	log.Infof("GitSummary=%s\n", GitSummary)
	log.Infof("VERSION=%s\n", Version)

	log.Info("Starting working...")

	reverseGeocode, err := initReverseGeocode()
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		os.Exit(1)
	}
	defer reverseGeocode.Close()

	go runProcessLoop(reverseGeocode, subscribed, timeout)
	<-subscribed

	go sendStatus(timeout)

	<-stop
	<-stop
}

func initOptions() {

	options = []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(globalOpt.ConnConf.ServerUser, globalOpt.ConnConf.ServerPassword),
		stomp.ConnOpt.Host(globalOpt.ConnConf.ServerAddr),
		stomp.ConnOpt.HeartBeatError(time.Second * time.Duration(globalOpt.ConnConf.HeartBeatError)),
		stomp.ConnOpt.HeartBeat(time.Second*time.Duration(globalOpt.ConnConf.HeartBeat),
			time.Second*time.Duration(globalOpt.ConnConf.HeartBeat)),
	}
}

func initReverseGeocode() (*Nominatim.ReverseGeocode, error) {

	log.Debug("initReverseGeocode")

	sqlOpenStr := "dbname=" + globalOpt.NominatimDB.DBname +
		" host=" + globalOpt.NominatimDB.Host +
		" user=" + globalOpt.NominatimDB.User +
		" password=" + globalOpt.NominatimDB.Password

	log.WithCaller(slf.CallerShort).Debugf("sqlOpenStr=%s", sqlOpenStr)

	reverseGeocode, err := Nominatim.NewReverseGeocode(sqlOpenStr)
	if err != nil {
		return nil, err
	}
	return reverseGeocode, nil
}

func runProcessLoop(reverseGeocode *Nominatim.ReverseGeocode, subscribed chan bool, timeToMonitoring chan []byte) {
	defer func() {
		stop <- true
	}()

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

	//subPrior, err := connSubsc.Subscribe(globalOpt.QueueConf.QueuePriorName, stomp.AckAuto)
	//if err != nil {
	//	log.WithCaller(slf.CallerShort).Errorf("cannot subscribe to %s: %s",
	//		globalOpt.QueueConf.QueueName, err.Error())
	//	return
	//}

	close(subscribed)

	// init a struct with info for monitoring queque
	data := initMonitoringData(connSend.GetConnInfo())

	ticker := time.NewTicker(time.Duration(globalOpt.DiagnConf.TimeOut) * time.Second)
	var ok bool
	var msg *stomp.Message

	// for requests/per sec; errors/per sec
	var prevNumOfReq, prevNumOfErr, prevNumOfErrResp, prevNumOfSuccResp int64
	//var queque string

	for {
		ok = false
		select {
		case msg, ok = <-sub.C:
			//break
		case <-ticker.C:
			data.CurrentTime = time.Now().Format(time.RFC3339)
			calculateSeverity(&data)
			calculateRatePerSec(&data, &prevNumOfReq, &prevNumOfErr, &prevNumOfErrResp, &prevNumOfSuccResp)
			log.Infof("data: %v", data)
			b, err := json.Marshal(data)
			if err != nil {
				log.Error(err.Error())
				continue
			}
			timeToMonitoring <- b
			continue
		}
		start := time.Now()

		if !ok {
			log.Warn("msg, ok = <-sub.C: !ok")
			data.ReconnectCount++
			data.LastError = "Reconnect"
			continue
		}

		data.Reqs++

		reqJSON := msg.Body
		var p Params
		p.machineId = connSend.GetConnInfo()

		replyJSON, whoToSent, errResp, err := p.locationSearch(reqJSON, reverseGeocode)
		if err != nil {
			log.WithCaller(slf.CallerShort).Errorf("Error: locationSearch %s; fullMsg=%v", err.Error(), msg)
			data.ErrorCount++
			data.LastError = err.Error()
			continue
		}
		if errResp == true {
			data.ErrResp++
		} else {
			data.SuccResp++
		}

		err = connSend.Send(globalOpt.ConnConf.QueueFormat+*whoToSent, "application/json;charset=utf-8",
			[]byte(replyJSON), nil...)
		if err != nil {
			data.MachineAddr = connSend.GetConnInfo()
			data.ErrorCount++
			log.WithCaller(slf.CallerShort).Errorf("Failed to send to server %s", err)
			time.Sleep(time.Second)
			data.LastError = err.Error()
			continue
		}

		elapsed := float64(time.Since(start)) / 1000.0 / 1000.0
		//data.EMA = (data.EMA + elapsed) / 2
		data.AverageRate = (1-globalOpt.DiagnConf.CoeffEMA)*data.AverageRate + globalOpt.DiagnConf.CoeffEMA*elapsed
	}
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

func (p *Params) locationSearch(rawMsg []byte, geocode *Nominatim.ReverseGeocode) ([]byte, *string, bool, error) {

	if len(rawMsg) == 0 {
		return nil, nil, false, fmt.Errorf("%s", "Empty body request")
	}

	log.Debugf("Request: %s", rawMsg)

	err := p.addCoordinatesToStruct(rawMsg)
	if err != nil {
		return nil, nil, false, err
	}

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

	placeJSON, err := getLocationJSON(*place)
	if err != nil {
		log.WithCaller(slf.CallerShort).Error(err.Error())
		return createErrResponse(err), &whoToSent, true, nil
	}

	log.Debugf("Client:%s ID:%d placeJSON:%s", p.clientReq.ClientID, p.clientReq.ID, string(placeJSON))

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

//calculateRatePerSec(data, &prevNumOfReq, &prevNumOfErr, &prevNumOfErrResp)
func calculateRatePerSec(dataP *monitoringData, prevNumOfReq, prevNumOfErr, prevNumOfErrResp, prevSuccR *int64) {

	data := *dataP

	if globalOpt.DiagnConf.TimeOut == 0 {
		log.Warn("globalOpt.DiagnConf.TimeOut = 0; could not calculate requests/per second")
		return
	}

	//log.Warnf(" BEFORE: prevNumOfReq=%d, prevNumOfErr=%d, prevNumOfErrResp=%d, prevSuccR=%d", *prevNumOfReq, *prevNumOfErr, *prevNumOfErrResp, *prevSuccR)
	//log.Warnf(" BEFORE: data.Reqs=%d, data.ErrorCount=%d, data.ErrResp=%d, data.SuccResp=%d", data.Reqs, data.ErrorCount, data.ErrResp, data.SuccResp)

	period := float64(globalOpt.DiagnConf.TimeOut)
	//log.Infof("period=%d", period)

	data.RequestRate = (float64(data.Reqs) - float64(*(prevNumOfReq))) / period
	data.ErrorRate = (float64(data.ErrorCount) - float64(*(prevNumOfErr))) / period
	data.ErrorRespRate = (float64(data.ErrResp) - float64(*(prevNumOfErrResp))) / period

	//calculating success responces
	prevSuccResp := *prevNumOfReq - *prevNumOfErr - *prevNumOfErrResp
	if prevSuccResp != *prevSuccR {
		//for first time; to check that all right
		log.Warnf("Wrong success responce calculating %d %d", prevSuccResp, *prevSuccR)
	}
	data.SuccessRespRate = (float64(data.SuccResp) - float64(prevSuccResp)) / period
	log.Infof("dataP=%v", data)
	*dataP = data

	//log.Warnf("data.RequestRate =%d, data.ErrorRate=%d, data.ErrorRespRate=%d, data.SuccessRespRate=%d", data.RequestRate, data.ErrorRate, data.ErrorRespRate, data.SuccessRespRate)

	*prevNumOfErr = data.ErrorCount
	*prevNumOfErrResp = data.ErrResp
	*prevNumOfReq = data.Reqs
	*prevSuccR = data.SuccResp

	//log.Warnf("AFTER: prevNumOfReq=%d, prevNumOfErr=%d, prevNumOfErrResp=%d, prevSuccR=%d", *prevNumOfReq, *prevNumOfErr, *prevNumOfErrResp, *prevSuccR)
}

func calculateSeverity(data *monitoringData) {

	if (*data).Reqs != 0 {
		(*data).Severity = (float64((*data).ErrorCount) * 100.0) / (float64((*data).Reqs)) * globalOpt.DiagnConf.CoeffSeverity
	}
	log.Infof("sevetiry=%f", (*data).Severity)
}

func initMonitoringData(machineAddr string) monitoringData {

	timeStr := fmt.Sprintf("%s", time.Now().Format(time.RFC3339))
	hostname, _ := os.Hostname()
	//pid :=
	data := monitoringData{
		StartTime:      string(time.Now().Format(time.RFC3339)),
		LastReconnect:  timeStr,
		ReconnectCount: 0,
		ErrResp:        0,
		SuccResp:       0,
		AverageRate:    0.0,
		ErrorCount:     0,
		LastError:      "",
		MachineAddr:    machineAddr,
		Severity:       0.0,

		Type: "status",
		Id:   uuid,
		Name: globalOpt.Name,

		Subtype:      "worker",
		Subsystem:    "",
		ComputerName: hostname,
		UserName:     fmt.Sprintf("%d", os.Getuid()),
		ProcessName:  os.Args[0],
		Version:      Version,
		Pid:          os.Getpid(),
		Message:      "",
		RequestRate:  0,
		ErrorRate:    0,
	}
	return data
}

func sendStatus(timeToMonitoring chan []byte) {

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

			err = connSend.Send(globalOpt.DiagnConf.TopicName, "application/json", data, nil...)
			if err != nil {
				log.Errorf("Error %s", err.Error())
				continue
			}

		}
	}
}
