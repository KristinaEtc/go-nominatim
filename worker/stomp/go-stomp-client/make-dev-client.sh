#!/bin/bash
set -e

make -f Makefile-client
mv Makefile-client Makefile
sudo checkinstall -D --pkgversion=0.6.1 --pkgname=go-stomp-client \
       --maintainer="Kristina Kovalevskaya isitiriss@gmail.com" --autodoinst=no \
       --spec=ABOUT.md --provides=go-stomp-server --pkgsource=go-stomp-client
make clean
mv Makefile Makefile-client
RETVAL=$?
[ $RETVAL -eq 0 ] && echo Success
[ $RETVAL -ne 0 ] && echo Failure
