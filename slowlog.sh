#!/bin/sh
mysqldumpslow -s t /var/log/mariadb/slow.log >  /var/www/slow.log
curl -X POST --data-urlencode "`cat /var/www/slow.log | ruby kataribe.rb`" https://hooks.slack.com/services/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 


