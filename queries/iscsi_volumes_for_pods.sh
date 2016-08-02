#!/bin/bash

cd "$(dirname "$0")"

export MYSQL_IP=`kubectl describe pod kubevoltracker | grep ^IP | awk -F' '  '{print $NF}'`
mysql -t -h${MYSQL_IP} -uroot -proot -D kubevoltracker < iscsi_volumes_for_pods.sql \
		| less -SFX
