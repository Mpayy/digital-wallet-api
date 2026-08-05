package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

func NewViper() *viper.Viper {
	config := viper.New()

	config.SetConfigName(".env")
	config.SetConfigType("env")

	config.AddConfigPath(".")
	config.AddConfigPath("../../")

	config.AutomaticEnv()

	if err := config.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			panic(fmt.Errorf("fatal error config file: %w", err))
		}
	}

	return config
}
