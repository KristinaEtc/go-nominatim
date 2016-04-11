CMDARGS=-f=json -a=53.913658 -b=27.600286 -z=20 -d=false
LIBPATH=$(GOPATH)/src/Nominatim/lib

all: main
#	./run-psql.sh
	./main $(CMDARGS)

main: main.go $(LIBPATH)/PlaceLookup.go $(LIBPATH)/ReverseGeocode.go
	go build main.go





