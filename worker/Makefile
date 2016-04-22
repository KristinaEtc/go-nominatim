all:
	./configure-nsqd.sh
	#go run worker.go
close:
	pkill nsqd
	pkill nsqlookupd
	pkill nsqadmin 
	rm *.dat

