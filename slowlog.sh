#!/bin/sh
mysqldumpslow -s t /var/log/mariadb/slow.log >  /var/www/slow.log
cat /var/www/


