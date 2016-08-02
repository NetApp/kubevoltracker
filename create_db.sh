mysql -h$1 -u$2 -p$3 -e "CREATE DATABASE IF NOT EXISTS kubevoltracker"
mysql -h$1 -u$2 -p$3 -e "CREATE DATABASE IF NOT EXISTS kubevoltracker_test"
mysql -h$1 -u$2 -p$3 -D kubevoltracker < ./dbmanager/mysql/clear_schema.sql 
mysql -h$1 -u$2 -p$3 -D kubevoltracker_test < ./dbmanager/mysql/clear_schema.sql 
mysql -h$1 -u$2 -p$3 -D kubevoltracker < ./dbmanager/mysql/schema.sql
mysql -h$1 -u$2 -p$3 -D kubevoltracker_test < ./dbmanager/mysql/schema.sql
