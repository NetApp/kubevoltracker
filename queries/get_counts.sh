cd "$(dirname "$0")"

export MYSQL_IP=`kubectl describe pod kubevoltracker | grep ^IP | awk -F' '  '{print $NF}'`
mysql -h${MYSQL_IP} -uroot -proot -D kubevoltracker -e "SELECT COUNT(*) as 'Pod Count' FROM pod;"
mysql -h${MYSQL_IP} -uroot -proot -D kubevoltracker -e "SELECT COUNT(*) as 'PVC Count' FROM pvc;"
mysql -h${MYSQL_IP} -uroot -proot -D kubevoltracker -e "SELECT COUNT(*) as 'PV Count' FROM pv;"
