package main

import (
	"fmt"
	"os"
	"time"

	"github.com/go-stomp/stomp"
)

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
	//log.Infof("severity=%f", (*data).Severity)
}

func initMonitoringData(machineAddr string) monitoringData {

	timeStr := time.Now().Format(time.RFC3339)
	hostname, _ := os.Hostname()
	//pid :=
	data := monitoringData{
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

func sendStatus(monitoringCh chan []byte) {

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
		case data := <-monitoringCh:

			err = connSend.Send(globalOpt.DiagnConf.TopicName, "application/json", data, nil...)
			if err != nil {
				log.Errorf("Error %s", err.Error())
				continue
			}

		}
	}
}
