FROM ubuntu
FROM golang:1.6.1
MAINTAINER Kristina Etc

RUN apt-get install checkinstall
RUN go get github.com/ahmetalpbalkan/govvv

