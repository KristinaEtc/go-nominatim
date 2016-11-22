FROM ubuntu
MAINTAINER Kristina Etc

RUN apt-get -y upgrade && apt-get -y update && apt-get install -y checkinstall
RUN apt-get install -y curl
RUN curl -O https://storage.googleapis.com/golang/go1.6.linux-amd64.tar.gz && tar -xvf go1.6.linux-amd64.tar.gz && \
mv go /usr/local
RUN echo PATH=$PATH:/usr/local/go/bin >> ~/.profile
RUN echo GOPATH=~/go >>  ~/.profile
RUN mkdir ~/go && cd ~/go && mkdir src && cd src && pwd && go get github.com/ahmetalpbalkan/govvv && \
go get github.com/ahmetalpbalkan/govvv && \
go get github.com/kardianos/govendor && \
go get github.com/KristinaEtc/go-nominatim; exit 0 && \
cd $(GOPATH)/github.com/KristinaEtc/go-nominatim && \
git checkout dev && \
govendor sync

