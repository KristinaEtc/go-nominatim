package main

import (
	_ "github.com/KristinaEtc/slflog"
)

func processErrorTimeOut(errMessage string, data *ResponseStatistic) {
	data.ErrorCount++
	data.ErrTimeOut++
	data.CurrErrTimeOut++
	data.LastError = errMessage
	data.CurrLastError = errMessage
}

func processCommonError(errMessage string, data *ResponseStatistic) {
	data.ErrorCount++
	data.CurrErrorCount++
	data.LastError = errMessage
	data.CurrLastError = errMessage
}

func processSuccess(data *ResponseStatistic) {
	data.CurrSuccResponses++
	data.SuccResp++
}

func cleanErrorStat(data *ResponseStatistic) {

	data.CurrErrorCount = 0
	data.CurrErrResponses = 0
	data.CurrSuccResponses = 0
	data.CurrErrTimeOut = 0
	data.CurrLastError = ""
	data.CurrRequests = 0
}

func processNewRequest(data *ResponseStatistic) {
	data.CurrRequests++
	data.Reqs++
}
