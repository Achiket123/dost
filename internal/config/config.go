package config

import (
	"github.com/spf13/viper"
)

// Config holds all configuration for our application
type Config struct {
	App    AppConfig    `mapstructure:"app"`
	AI     AIConfig     `mapstructure:"ai"`
	Logger LoggerConfig `mapstructure:"logger"`
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Port    int    `mapstructure:"port"`
	Debug   bool   `mapstructure:"debug"`
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type AIConfig struct {
	API_KEY string `mapstructure:"API_KEY"`
	ORG     string `mapstructure:"ORG"`
	MODEL   string `mapstructure:"MODEL"`
	// TODO: Will Work on implementing this
	TEMPERATURE float32 `mapstructure:"TEMPERATURE"`
	MAX_TOKENS  int     `mapstructure:"MAX_TOKENS"`
	TOP_P       float32 `mapstructure:"TOP_P"`
}

// Load reads configuration from file and environment variables
func Load() (*Config, error) {
	// Set default values
	viper.SetDefault("app.name", "dost")
	viper.SetDefault("app.version", "1.0.0")
	viper.SetDefault("app.port", 8080)
	viper.SetDefault("app.debug", false)

	viper.SetDefault("ai.API_KEY", "YOUR_API_KEY")
	viper.SetDefault("ai.ORG", "YOUR_ORG")
	viper.SetDefault("ai.MODEL", "YOUR_MODEL")

	viper.SetDefault("logger.level", "info")
	viper.SetDefault("logger.format", "json")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
