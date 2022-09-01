package models

type Config struct {
	LogLevel  string `yaml:"logLevel"`
	KubeConfigPath    string    `yaml:"kubeConfigPath"`
	Namespace struct {
		Prefix    string    `yaml:"prefix"`
		Suffix    string    `yaml:"suffix"`
		Duration  string    `yaml:"duration"`
		Resources Resources `yaml:"resources"`
	} `yaml:"namespace"`
	BasicAuth BasicAuth `yaml:"basicAuth"`
}

type Resources struct {
	Requests struct {
		cpu     string `yaml:"cpu"`
		memory  string `yaml:"memory"`
		storage string `yaml:"storage"`
	} `yaml:"requests"`
	Limits struct {
		cpu    string `yaml:"cpu"`
		memory string `yaml:"memory"`
	} `yaml:"limits"`
}

type BasicAuth []struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
