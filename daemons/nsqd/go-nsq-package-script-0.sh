#!/bin/bash

USER_T=go-nsq
PATH_PROGS=/opt/go-nsq

if [ ! -w /opt ]; then
        echo "Error: must be root"
        exit 1
fi

id -u ${USER_T}
        if [ "$?" = "1" ]; then
        echo "Adding a new user: go-nsq..."
        useradd ${USER_T} -M
fi

mkdir -p ${PATH_PROGS}
chown ${USER_T}:${USER_T} ${PATH_PROGS}
#chmod -R u+wrx ${PATH_PROGS}
cp nsqd ${PATH_PROGS}/nsqd
cp nsqlookupd ${PATH_PROGS}/nsqlookupd
cp nsqadmin ${PATH_PROGS}/nsqadmin
cp worker ${PATH_PROGS}/worker
cp config.json ${PATH_PROGS}/config.json

#if [ ! -x ${PATH_PROGS}/nsqd ]; then
#        chown -R ${USER_T} ${PATH_PROGS}/nsqd
#fi

chown ${USER_T}:${USER_T} ${PATH_PROGS}/nsqadmin
#chmod a+r ${PATH_PROGS}/nsqadmin
chown ${USER_T}:${USER_T} ${PATH_PROGS}/nsqd
#chmod a+r ${PATH_PROGS}/nsqd
chown ${USER_T}:${USER_T} ${PATH_PROGS}/nsqlookupd
#chmod a+r ${PATH_PROGS}/nsqlookupd
chown ${USER_T}:${USER_T} ${PATH_PROGS}/worker
#chmod a+r ${PATH_PROGS}/worker
chown ${USER_T}:${USER_T} ${PATH_PROGS}/config.json

#--data-path=${DATA_DIR}
mkdir -p /var/lib/${USER_T}/
chown -R ${USER_T} /var/lib/${USER_T}/
chmod a+rx /var/lib/${USER_T}

mkdir -p /var/log/${USER_T}/
chown -R ${USER_T} /var/log/${USER_T}/
chmod a+rx /var/log/${USER_T}
