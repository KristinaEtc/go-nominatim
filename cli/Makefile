CMDARGS=-f=json -a=53.9009279 -b=27.5592055 -z=18 -d=false
LIBPATH=$(GOPATH)/src/Nominatim/lib

all: main
#	./run-psql.sh
#	go test main.go
	./main $(CMDARGS) -t=true -n="test.csv"

one: main
	./main $(CMDARGS)

main: main.go $(LIBPATH)/PlaceLookup.go $(LIBPATH)/ReverseGeocode.go
	go build main.go





