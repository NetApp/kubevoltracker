#!/bin/bash

cd "$(dirname "$0")"
export MYSQL_IP=`kubectl describe pod kubevoltracker | grep ^IP | awk -F' '  '{print $NF}'`

mysql -t -h${MYSQL_IP} -uroot -proot -D kubevoltracker -e \
	'SELECT name AS "Name", create_time AS "Create Time", delete_time AS "Delete Time" FROM pv ORDER BY delete_time ASC' \
	| less -SFX
