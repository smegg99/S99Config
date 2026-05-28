package main

#Config: {
	app: {
		name: string
	}
	private_values: {
		from_env: string @go(FromEnv,type=Secret)
	} @go(PrivateValues)
	public_values: {
		from_env:       string @go(FromEnv)
		config_path:    string @go(ConfigPath)
		data_path:      string @go(DataPath)
		optional_value: string @go(OptionalValue)
	} @go(PublicValues)
}
