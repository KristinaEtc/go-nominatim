package monitoring

import (
	"fmt"
	"os"
	"time"

	"github.com/ventu-io/slf"
)

var pwdCurr = "KristinaEtc/Nominatim/lib/monitoring"
var log = slf.WithContext(pwdCurr)

// MonitoringData is a struct which will be sended to a special topic
// for diagnostics
type MonitoringData struct {
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
	Type           string  `json:"type"`
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Subtype        string  `json:"subtype"`
	Subsystem      string  `json:"subsystem"`
	ComputerName   string  `json:"computer"`
	UserName       string  `json:"user"`
	ProcessName    string  `json:"process"`
	Version        string  `json:"version"`
	Pid            int     `json:"pid"`
	//Tid          int    `json:tid`
	Message         string  `json:"message"`
	RequestRate     float64 `json:"request_rate"`
	ErrorRate       float64 `json:"error_rate"`
	ErrorRespRate   float64 `json:"error_resp_rate"`
	SuccessRespRate float64 `json:"success_resp_rate"`
}

type requestStat struct {
	numOfReq, numOfErr, numOfErrResp, succR int64
}

var prevReqStat requestStat

// InitMonitoringData creates and returns MonitoringData object
func InitMonitoringData(machineAddr, version, name, uuid string) *MonitoringData {

	timeStr := time.Now().Format(time.RFC3339)
	hostname, _ := os.Hostname()
	//pid :=
	data := MonitoringData{
		StartTime:      timeStr,
		LastReconnect:  "",
		ReconnectCount: 0,
		ErrResp:        0,
		SuccResp:       0,
		AverageRate:    0.0,
		ErrorCount:     0,
		LastError:      "",
		MachineAddr:    machineAddr,
		Severity:       0.0,

		Type: "status",
		ID:   uuid,
		Name: name,

		Subtype:      "worker",
		Subsystem:    "",
		ComputerName: hostname,
		UserName:     fmt.Sprintf("%d", os.Getuid()),
		ProcessName:  os.Args[0],
		Version:      version,
		Pid:          os.Getpid(),
		Message:      "",
		RequestRate:  0,
		ErrorRate:    0,
	}
	return &data
}

// CalculateRatePerSec calculates rate of requests
func (d *MonitoringData) CalculateRatePerSec(timeOut int) {

	if timeOut == 0 {
		log.Warn("timeOut = 0; could not calculate requests/per second")
		return
	}

	period := float64(timeOut)

	d.RequestRate = (float64(d.Reqs) - float64(prevReqStat.numOfReq)) / period
	d.ErrorRate = (float64(d.ErrorCount) - float64(prevReqStat.numOfErr)) / period
	d.ErrorRespRate = (float64(d.ErrResp) - float64(prevReqStat.numOfErrResp)) / period

	//calculating success responces
	prevReqStat.succR = prevReqStat.numOfReq - prevReqStat.numOfErr - prevReqStat.numOfErrResp
	/*if prevSuccResp != *prevSuccR {
		//for first time; to check that all right
		log.Warnf("Wrong success responce calculating %d %d", prevSuccResp, *prevSuccR)
	}*/
	d.SuccessRespRate = (float64(d.SuccResp) - float64(prevReqStat.succR)) / period

	prevReqStat.numOfErr = d.ErrorCount
	prevReqStat.numOfErrResp = d.ErrResp
	prevReqStat.numOfReq = d.Reqs
	prevReqStat.succR = d.SuccResp

}

// CalculateSeverity calculates severity of requests
func (d *MonitoringData) CalculateSeverity(coeffSeverity float64) {

	if (*d).Reqs != 0 {
		(*d).Severity = (float64((*d).ErrorCount) * 100.0) / (float64((*d).Reqs)) * coeffSeverity
	}
	//log.Infof("severity=%f", (*data).Severity)
}

/*

//SendLoop - function to send incoming data from resultDataChannel to stomp topic
//stop chan bool - unused shit-code remains
//serverAddr - stomp server address
//topicName - topic name
//options - connection options
//resultDataChannel - channel with data to send
func SendLoop(stop chan bool, serverAddr, topicName string, options []func(*stomp.Conn) error, resultDataChannel chan []byte) {

	defer func() {
		stop <- true
	}()

	connSend, err := stomp.Dial("tcp", serverAddr, options...)
	if err != nil {
		log.Errorf("cannot connect to server %s", err.Error())
		return
	}
	for {
		select {
		case data := <-resultDataChannel:
        if data == nil{
            log.Error("")
            return
        }

			err = connSend.Send(topicName, "application/json", data, nil...)
			if err != nil {
				log.Errorf("Error %s", err.Error())
				continue
			}

		}
	}
}

*/
