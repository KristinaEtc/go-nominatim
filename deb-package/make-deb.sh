#!/bin/bash
dpkg-deb --build go-stomp-server
mv go-stomp-server.deb go-stomp-server_1.0-1_all.deb
scp go-stomp-server_1.0-1_all.deb k@192.168.240.25:~/
