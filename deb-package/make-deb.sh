#!/bin/bash
dpkg-deb --build go-stomp-nominatim
mv go-stomp-nominatim.deb go-stomp-nominatim_1.0-1_all.deb
scp go-stomp-nominatim_1.0-1_all.deb k@192.168.240.25:~/
