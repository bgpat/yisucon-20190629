#!/bin/sh
cat /var/log/nginx/access.log | /var/www/kataribe -f /var/www/kataribe.toml > /var/www/kataribe.log
curl -X POST --data-urlencode "`cat /var/www/kataribe.log | ruby kataribe.rb`" https://hooks.slack.com/services/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

