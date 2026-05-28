package main

#Config: {
	service: string
	api_key: string @secret() @go(APIKey,type=Secret)
}
