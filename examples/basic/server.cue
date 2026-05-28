package main

#Server: {
	host: string | *"127.0.0.1"
	port: int & >0 & <=65535 | *8080
}
