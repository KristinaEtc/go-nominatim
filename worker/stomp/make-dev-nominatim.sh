#!/bin/bash
set -e

mv Makefile-nominatim Makefile
make
sudo checkinstall -D --pkgversion=0.6.1 --pkgname=go-stomp-nominatim \
       --maintainer="Kristina Kovalevskaya isitiriss@gmail.com" --autodoinst=no \
       --spec=ABOUT.md --provides=go-stomp-server --pkgsource=go-stomp-nominatim

make clean
mv Makefile Makefile-nominatim

RETVAL=$?
[ $RETVAL -eq 0 ] && echo Success
[ $RETVAL -ne 0 ] && echo Failure
