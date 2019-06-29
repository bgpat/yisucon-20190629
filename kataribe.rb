require 'json'
logfile = readlines
logfile = logfile.join("")
payload = {
	text: logfile
}
payload = "payload=#{payload.to_json}"

print %Q(#{payload})


