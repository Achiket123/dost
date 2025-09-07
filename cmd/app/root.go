package app

import (
	"dost/internal/config"
	"dost/internal/service"
	"dost/internal/service/orchestrator"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	Cfg     *config.Config
)

const environment = "dev"

var INITIAL_CONTEXT = 1

var rootCmd = &cobra.Command{
	Use:   "dost",
	Short: "AI-powered development orchestrator",
	Long: `
DOST is an AI CLI tool for Developers that empowers them to create and organize their projects.
It features an intelligent orchestrator that can subdivide complex tasks and route them to specialized agents.

Key Features:
- Intelligent task subdivision and routing
- Multi-agent coordination with caching
- Context-aware task execution
- Workflow management and monitoring

USING GEMINI API: DOST uses the Gemini API to provide AI-powered features.
It can help you with code generation, planning, analysis, and more.

SETUP: Create a configuration file ".dost.yaml" in your project directory with your API key and settings.
`,
	Args: cobra.ArbitraryArgs,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(InitConfig)
	rootCmd.Run = handleUserQuery

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file default is C:/.dost.yaml")
	rootCmd.PersistentFlags().String("ai", "", "AI query for Gemini API")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
func handleUserQuery(cmd *cobra.Command, args []string) {

	var agent orchestrator.AgentOrchestrator
	agent.NewAgent()
	query := strings.Join(args, " ")
	fmt.Printf("QUERY: %s\n", query)
	agent.Interaction(map[string]any{"query": query})

}
func InitConfig() {
	switch environment {
	case "dev":
		cfgFile = "C:\\Users\\Achiket\\Documents\\go\\dost\\configs\\.dost.yaml"
		viper.SetConfigFile(cfgFile)

	case "prod":
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigName(".dost")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}
	Cfg = cfg

	service.InitializeCache()

	fmt.Println("DOST initialized successfully")
}
