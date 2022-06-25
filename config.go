package main

import (
	"github.com/spf13/viper"
	"time"
)

type Config struct {
	HdgEndpoint string        `mapstructure:"HDG_ENDPOINT"`
	Language    string        `mapstructure:"HDG_LANGUAGE"`
	Ids         []int         `mapstructure:"HDG_IDS"`
	Timeout     time.Duration `mapstructure:"HDG_TIMEOUT"`
	Port        int           `mapstructure:"PORT"`
}

func loadConfig() (config Config, err error) {
	viper.AddConfigPath(".")
	viper.SetConfigName("app")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	viper.SetDefault("HDG_LANGUAGE", "deutsch")
	viper.SetDefault("HDG_TIMEOUT", "30s")
	viper.SetDefault("PORT", 8080)

	err = viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error
		} else {
			// Config file was found but another error was produced
			return
		}
	}

	err = viper.Unmarshal(&config)
	return
}
