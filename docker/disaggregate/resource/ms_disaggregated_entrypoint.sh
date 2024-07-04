#!/bin/bash
#

#get from env
FDB_ENDPOINT=${FDB_ENDPOINT}
CONFIGMAP_PATH=${CONFIGMAP_PATH:="/etc/doris"}
DORIS_HOME=${DORIS_HOME:="/opt/apache-doris"}

echo "fdb_cluster=$FDB_ENDPOINT" >> $DORIS_HOME/ms/conf/selectdb_cloud.conf
if [[ -d $CONFIGMAP_PATH ]]; then
    for file in `ls $CONFIGMAP_PATH`
        do
            if [[ "$file" == "selectdb_cloud.conf" ]] ; then
                mv -f $DORIS_HOME/ms/conf/$file $DORIS_HOME/ms/conf/$file.bak
                cp $CONFIGMAP_PATH/$file $DORIS_HOME/ms/conf/$file
                echo "fdb_cluster=$FDB_ENDPOINT" >> $DORIS_HOME/ms/conf/selectdb_cloud.conf
                continue
            fi

            if test -e $DORIS_HOME/ms/conf/$file ; then
                mv -f $DORIS_HOME/ms/conf/$file $DORIS_HOME/ms/conf/$file.bak
            fi
            ln -sfT $CONFIGMAP_PATH/$file $DORIS_HOME/ms/conf/$file
       done
fi

$DORIS_HOME/ms/bin/start.sh --$1
