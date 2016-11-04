package main

import (
	"github.com/KristinaEtc/go-nominatim/lib/monitoring"
	"github.com/go-stomp/stomp"
)

//calculateRatePerSec(data, &prevNumOfReq, &prevNumOfErr, &prevNumOfErrResp)
func calculateRatePerSec(dataP *monitoring.MonitoringData, prevNumOfReq, prevNumOfErr, prevNumOfErrResp, prevSuccR *int64) {

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
