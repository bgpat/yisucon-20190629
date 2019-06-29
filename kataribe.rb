require 'json'
logfile = readlines
logfile = logfile.join("")

name_status = `git log --name-status HEAD^..HEAD`

payload = {
	text: "\`\`\`#{name_status}\`\`\`\n\n" + logfile
}
payload = "payload=#{payload.to_json}"

print %Q(#{payload})


