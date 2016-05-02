nsqlookupd -verbose=true -version=true &
nsqd --lookupd-tcp-address=127.0.0.1:4160 -verbose=true -version=true &
nsqadmin --lookupd-http-address=127.0.0.1:4161 -version=true &
