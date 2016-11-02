package main

import (
	_ "github.com/KristinaEtc/slflog"
)

func processErrorTimeOut(errMessage string, data *WatcherData) {
	data.ErrorCount++
	data.ErrTimeOut++
	data.CurrErrTimeOut++
	data.LastError = errMessage
	data.CurrLastError = errMessage
}

func processCommonError(errMessage string, data *WatcherData) {
	data.ErrorCount++
	data.CurrErrorCount++
	data.LastError = errMessage
	data.CurrLastError = errMessage
}

func processSuccess(data *WatcherData) {
	data.CurrSuccResponses++
	data.SuccResp++
}

func cleanErrorStat(data *WatcherData) {

	data.CurrErrorCount = 0
	data.CurrErrResponses = 0
	data.CurrSuccResponses = 0
	data.CurrErrTimeOut = 0
	data.CurrLastError = ""
	data.CurrRequests = 0

	data.ResponseDelaysByID = make(map[string]int64)
}
