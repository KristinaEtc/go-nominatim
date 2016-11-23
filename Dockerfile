FROM go-nominatim
MAINTAINER Kristina Etc

RUN apt-get install -yq git

RUN go env && \
whereis checkinstall && \
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin && \
echo '$GOPATH/bin' && \
cd /go && \
go get github.com/ahmetalpbalkan/govvv && \
go get github.com/kardianos/govendor && \
cd /go/src/github.com/kardianos/govendor && go install && \
govendor status

EXPOSE 5432

RUN export PATH=$PATH:$GOROOT/bin:$GOPATH/bin && \
go get github.com/KristinaEtc/go-nominatim || true && \
echo "OK" && \
cd /go/src/github.com/KristinaEtc/go-nominatim && \
git checkout dev && \
govendor init && \
govendor sync && \
govvv build

RUN export PATH=$PATH:$GOROOT/bin:$GOPATH/bin && \
cd go/src/github.com/KristinaEtc/go-nominatim && git clone https://github.com/KristinaEtc/go-deb && \
cd go-deb && ./make-dev-nominatim.sh
