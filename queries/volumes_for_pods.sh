#!/bin/bash

cd "$(dirname "$0")"

export MYSQL_IP=`kubectl describe pod kubevoltracker | grep ^IP | awk -F' '  '{print $NF}'`
if [ "$#" -eq "0" ]
then
	mysql -t -h${MYSQL_IP} -uroot -proot -D kubevoltracker < volumes_for_pods.sql \
		| less -SFX
else
	if [ "$1" = "running" ] || [ "$1" = "r" ]
	then
		mysql -t -h${MYSQL_IP} -uroot -proot -D kubevoltracker < \
			volumes_for_pods_running.sql | less -SFX
	elif [ "$1" = "stopped" ] || [ "$1" = "s" ]
	then
		mysql -t -h${MYSQL_IP} -uroot -proot -D kubevoltracker < \
			volumes_for_pods_stopped.sql | less -SFX
	else
		echo "Usage: $0 [r[unning]|s[topped]]"
	fi
fi
