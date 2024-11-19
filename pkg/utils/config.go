package optimizerutil

import "github.com/spf13/viper"

type Config struct {
	InsightsService     string `mapstructure:"INSIGHTS_SRV"`
	HttpAddress         string `mapstructure:"HTTP_ADDR"`
	Namespace           string `mapstructure:"NAMESPACE"`
	PrometheusSRV       string `mapstructure:"PROMETHEUS_SRV"`
	PrometheusPort      string `mapstructure:"PROMETHEUS_PORT"`
	PrometheusNamespace string `mapstructure:"PROMETHEUS_NAMESPACE"`
}

func loadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
