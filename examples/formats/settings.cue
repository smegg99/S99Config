package main

#Config: {
	name: string
	private: {
		env_value:        string @go(EnvValue,type=Secret)
		configured_value: string @secret() @go(ConfiguredValue,type=Secret)
	}
	public: {
		env_value: string @go(EnvValue)
		data_path: string @go(DataPath)
	}
}
