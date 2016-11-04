package main

import (
	//important: must execute first; do not move
	_ "github.com/KristinaEtc/slflog"
)

import (
	"encoding/json"
	"os"
	"time"

	"github.com/KristinaEtc/config"
	"github.com/KristinaEtc/go-nominatim/lib"
	"github.com/go-stomp/stomp"
	_ "github.com/lib/pq"
	"github.com/ventu-io/slf"
)

var log = slf.WithContext("go-stomp-nominatim.go")

// These fields are populated by govvv

var (
	// BuildDate - binary build date
	BuildDate string
	// GitCommit - git commit hash
	GitCommit string
	// GitBranch - git branch name
	GitBranch string
	// GitState - working directory state (clean/dirty)
	GitState string
	// GitSummary - commit summary
	GitSummary string
	// Version - file version
	Version string
)

/*-------------------------
  Config option structures
-------------------------*/

var uuid string

//QueueOptConf - request/response queue Config
type QueueOptConf struct {
	QueueName      string
	QueuePriorName string
	ResentFullReq  bool
}

//DiagnosticsConf - worker status and diagnostics message configuration
type DiagnosticsConf struct {
	CoeffEMA      float64
	TopicName     string
	TimeOut       int // in seconds
	MachineID     string
	CoeffSeverity float64
	BufferSize    int //monitoring channel bufferSize
}

//ConnectionConf - mq connection config
type ConnectionConf struct {
	ServerAddr     string
	ServerUser     string
	ServerPassword string
	QueueFormat    string
	HeartBeatError int
	HeartBeat      int
}

// NominatimConf nominatim database connection options
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
	IsDebug     bool
	IsMock      bool
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
		BufferSize:    60, //5 minutes worth of messages (with default 5 seconds TimeOut)
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

var options = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("", ""),
	stomp.ConnOpt.Host("127.0.0.1"),
}

// ErrorResponse - struct for error answer to geocoding request
type ErrorResponse struct {
	Type    string
	Message string
}

//--------------------------------------------------------------------------

func main() {

	log = slf.WithContext("go-stomp-nominatim.go")

	config.ReadGlobalConfig(&globalOpt, "go-stomp-nominatim options")
	uuid = config.GetUUID(globalOpt.DirWithUUID)
	initOptions()

	subscribed := make(chan bool)
	timeout := make(chan []byte, globalOpt.DiagnConf.BufferSize)

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
		stomp.ConnOpt.Header("wormmq.link.peer_name", globalOpt.Name),
		stomp.ConnOpt.Header("wormmq.link.peer", uuid),
		stomp.ConnOpt.HeartBeatError(time.Second * time.Duration(globalOpt.ConnConf.HeartBeatError)),
		stomp.ConnOpt.HeartBeat(time.Second*time.Duration(globalOpt.ConnConf.HeartBeat),
			time.Second*time.Duration(globalOpt.ConnConf.HeartBeat)),
	}
}

func initReverseGeocode() (Nominatim.ReverseGeocode, error) {

	log.Debug("initReverseGeocode")

	if globalOpt.IsMock {
		return Nominatim.NewReverseGeocodeMock(), nil
	}

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

func runProcessLoop(reverseGeocode Nominatim.ReverseGeocode, subscribed chan bool, monitoringCh chan []byte) {
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
	monitoringChannelFull := false

	for {
		ok = false
		select {
		case msg, ok = <-sub.C:
			//break
		case <-ticker.C:
			data.CurrentTime = time.Now().Format(time.RFC3339)
			calculateSeverity(&data)
			log.Infof("status: requests:%d/%d errors:%d/%d ok:%d/%d errResp:%d/%d",
				data.Reqs-prevNumOfReq, data.Reqs,
				data.ErrorCount-prevNumOfErr, data.ErrorCount,
				data.SuccResp-prevNumOfSuccResp, data.SuccResp,
				data.ErrResp-prevNumOfErrResp, data.ErrResp)
			calculateRatePerSec(&data, &prevNumOfReq, &prevNumOfErr, &prevNumOfErrResp, &prevNumOfSuccResp)
			b, err := json.Marshal(data)
			if err != nil {
				log.Error(err.Error())
				continue
			}
			select {
			case monitoringCh <- b:
				monitoringChannelFull = false
			default:
				if !monitoringChannelFull {
					monitoringChannelFull = true
					log.Error("runProcessLog: monitoring channel full")
				}
			}

			continue
		}
		start := time.Now()

		if !ok {
			log.Warn("msg, ok = <-sub.C: !ok")
			data.ReconnectCount++
			data.LastError = "Reconnect"
			//data.LastReconnect = time.Now().In(time.UTC).Format(time.RFC3339)
			data.LastReconnect = time.Now().Format(time.RFC3339)
			continue
		}

		data.Reqs++

		reqJSON := msg.Body
		//var p params
		//p.machineID = connSend.GetConnInfo()

		replyJSON, whoToSent, errResp, err := locationSearch(reqJSON, reverseGeocode, data.ID)
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
