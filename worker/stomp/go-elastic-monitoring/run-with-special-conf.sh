#!/bin/bash

go build go-elastic-monitoring.go
./go-elastic-monitoring --Verbose --ConfigPath=go-elasic-monitoring.config-example 2>r
