package main

import (
	"encoding/json"
	"fmt"
	"time"
)

//processAddrResponse process when address response was got
func processAddrResponse(timeRequestsByID *map[int]int64, responseDelaysByID *map[string][]int64, dataDelays *ResponseDelays, dataStatistic *ResponseStatistic, msg []byte) {
	//message := string(msg)
	//log.Debugf("Res=[%s]", message)

	requestID, requestTime, workerID, err := parseResponse(msg)
	if err != nil {
		log.Error(err.Error())
		processCommonError(err.Error(), dataStatistic)
		return
	}

	timeRequest, ok := (*timeRequestsByID)[requestID]
	if !ok {
		log.Debugf("timeRequestsByID: [%v]", timeRequestsByID)
		errMessage := fmt.Sprintf("No requests was sended with such id: [%d,%d]", requestID, requestTime)
		log.Warnf(errMessage)
		processCommonError(errMessage, dataStatistic)
		return
	}

	t := time.Now().UTC().UnixNano()
	(*responseDelaysByID)[workerID] = append((*responseDelaysByID)[workerID], (t-timeRequest)/1000000)
	//log.Debugf("[%d] t.UTC().Unix() - timeRequestsByID[requestID]=[%d]", requestID, (t-timeRequestsByID[requestID])/1000000)

	delete(*timeRequestsByID, requestID)
	processSuccess(dataStatistic)
}

// processAddrRequest process when address request was got
func processAddrRequest(id string, dataStatistic *ResponseStatistic, timeRequestsByID *map[int]int64) {
	//id is a string with  requestID and requestTime: "id,time"
	requestID, requestTime, err := parseID(id)
	if err != nil {
		log.Error(err.Error())
		processCommonError(err.Error(), dataStatistic)
		return
	}
	(*timeRequestsByID)[requestID] = requestTime
	processNewRequest(dataStatistic)
}

//checkRequestTimeOut checking time outs in saved address requests
func checkRequestTimeOut(t time.Time, timeRequestsByID *map[int]int64, responseDelaysByID *map[string][]int64, dataStatistic *ResponseStatistic) {

	for requestID, requestTime := range *timeRequestsByID {

		delay := (t.UTC().UnixNano() - requestTime) / 1000000
		if delay >= requestTimeOut*1000 {
			log.Warnf("timeout for: [%d,%d]", requestID, requestTime)

			errMessage := fmt.Sprintf("TimeOut for: [%d,%d]", requestID, requestTime)
			processErrorTimeOut(errMessage, dataStatistic)
			delete(*timeRequestsByID, requestID)
		}
	}
}

func sendMessageDelays(dataDelays *ResponseDelays, dataStatistic *ResponseStatistic, pr Process, topic string) error {
	reqInJSON, err := json.Marshal(dataDelays)
	if err != nil {
		processCommonError(err.Error(), dataStatistic)
		return err
	}

	err = pr.connSend.Send(topic, "application/json", []byte(reqInJSON), nil...)
	if err != nil {
		processCommonError(err.Error(), dataStatistic)
		return err
	}

	log.Debugf("Delay Message=%s", reqInJSON)

	return nil
}

func sendMessageStatistic(dataDelays *ResponseStatistic, dataStatistic *ResponseStatistic, pr Process, topic string) ([]byte, error) {
	//dataStatistic.ResponseDelaysByID = convertFieldNames(responseDelaysByID)
	dataStatistic.CurrentTime = time.Now().UTC().Format(time.RFC3339)
	dataStatistic.Subtype = "watcher-statistic"

	reqInJSON, err := json.Marshal(dataStatistic)
	if err != nil {
		processCommonError(err.Error(), dataStatistic)
		return nil, err
	}

	log.Debugf("Status Message=%s", reqInJSON)

	err = pr.connSend.Send(topic, "application/json", []byte(reqInJSON), nil...)
	if err != nil {
		processCommonError(err.Error(), dataStatistic)
		return nil, err
	}
	return reqInJSON, nil
}
