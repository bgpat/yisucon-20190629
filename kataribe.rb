require 'json'
logfile = readlines
logfile = logfile.join("")
payload = {
	text: '```' + "\n"  + logfile + "\n" + '```'
}
payload = "payload=#{payload.to_json}"

print %Q(#{payload})


