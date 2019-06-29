require 'mysql2'

client = Mysql2::Client.new(:socket => '/var/lib/mysql/mysql.sock', :username => 'root', :encoding => 'utf8', :database => 'isuwitter')

results = client.query('select * from friends')
client.query('CREATE TABLE IF NOT EXISTS friends2 (
	me varchar(20) not null,
	friend varchar(20) not null,
	PRIMARY KEY(me, friend)
);')
results.each do |row|
	me = row['me']
	friends = row['friends'].split(',')
	stmt = client.prepare('insert into friends2 values (?, ?)')
	friends.each do |friend|
		stmt.execute(me, friend)
	end
end

stmt.close


