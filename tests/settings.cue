package tests

#Config: {
	app: {
		name: string
		port: int & >0 & <=65535 | *8080
	}
	token?:  string @go(Token,type=Secret)
	output?: string
}
