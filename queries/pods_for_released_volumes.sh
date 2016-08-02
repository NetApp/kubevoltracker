#!/bin/sh

cd "$(dirname "$0")"
export MYSQL_IP=`kubectl describe pod kubevoltracker | grep ^IP | awk -F' '  '{print $NF}'`

mysql -t -h${MYSQL_IP} -uroot -proot -D kubevoltracker < pods_for_released_volumes.sql
