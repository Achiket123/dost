package config

import (
	"github.com/spf13/viper"
)

// Config holds all configuration for our application
type Config struct {
	App          AppConfig          `mapstructure:"app"`
	AI           AIConfig           `mapstructure:"ai"`
	CRITIC       CriticConfig       `mapstructure:"critic"`
	EXECUTOR     ExecutorConfig     `mapstructure:"executor"`
	KNOWLEDGE    KnowledgeConfig    `mapstructure:"knowledge"`
	PLANNER      PlannerConfig      `mapstructure:"planner"`
	ORCHESTRATOR OrchestratorConfig `mapstructure:"orchestrator"`
	CODER        CoderConfig        `mapstructure:"coder"`
	Logger       LoggerConfig       `mapstructure:"logger"`
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Port    int    `mapstructure:"port"`
	Debug   bool   `mapstructure:"debug"`
}
type CriticConfig struct {
	API_KEY string `mapstructure:"API_KEY"`
	ORG     string `mapstructure:"ORG"`
	MODEL   string `mapstructure:"MODEL"`
}
type OrchestratorConfig struct {
	API_KEY string `mapstructure:"API_KEY"`
	ORG     string `mapstructure:"ORG"`
	MODEL   string `mapstructure:"MODEL"`
}
type ExecutorConfig struct {
	API_KEY string `mapstructure:"API_KEY"`
	ORG     string `mapstructure:"ORG"`
	MODEL   string `mapstructure:"MODEL"`
}
type KnowledgeConfig struct {
	API_KEY string `mapstructure:"API_KEY"`
	ORG     string `mapstructure:"ORG"`
	MODEL   string `mapstructure:"MODEL"`
}
type PlannerConfig struct {
	API_KEY string `mapstructure:"API_KEY"`
	ORG     string `mapstructure:"ORG"`
	MODEL   string `mapstructure:"MODEL"`
}
type CoderConfig struct {
	API_KEY string `mapstructure:"API_KEY"`
	ORG     string `mapstructure:"ORG"`
	MODEL   string `mapstructure:"MODEL"`
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type AIConfig struct {
	CoderURL string `mapstructure:"CODER_URL"`
	CoderAPI string `mapstructure:"CODER_API"`
	CoderORG string `mapstructure:"CODER_ORG"`

	PlannerURL string `mapstructure:"PLANNER_URL"`
	PlannerAPI string `mapstructure:"PLANNER_API"`
	PlannerORG string `mapstructure:"PLANNER_ORG"`

	OrchestratorURL string `mapstructure:"ORCHESTRATOR_URL"`
	OrchestratorAPI string `mapstructure:"ORCHESTRATOR_API"`
	OrchestratorORG string `mapstructure:"ORCHESTRATOR_ORG"`

	CriticURL string `mapstructure:"CRITIC_URL"`
	CriticAPI string `mapstructure:"CRITIC_API"`
	CriticORG string `mapstructure:"CRITIC_ORG"`

	ExecutorURL string `mapstructure:"EXECUTOR_URL"`
	ExecutorAPI string `mapstructure:"EXECUTOR_API"`
	ExecutorORG string `mapstructure:"EXECUTOR_ORG"`

	KnowledgeURL string `mapstructure:"KNOWLEDGE_URL"`
	KnowledgeAPI string `mapstructure:"KNOWLEDGE_API"`
	KnowledgeORG string `mapstructure:"KNOWLEDGE_ORG"`

	// [DEPRECATED]
	API_KEY string `mapstructure:"API_KEY"`
	ORG     string `mapstructure:"ORG"`
	MODEL   string `mapstructure:"MODEL"`

	// TODO: Will Work on implementing this
	TEMPERATURE float32 `mapstructure:"TEMPERATURE"`
	MAX_TOKENS  int     `mapstructure:"MAX_TOKENS"`
	TOP_P       float32 `mapstructure:"TOP_P"`
}

// Load reads configuration from file and environment variables
// Config file location is determined by the caller (see cmd/app/root.go InitConfig)
// and supports platform-independent paths
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
