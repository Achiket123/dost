package app

import (
	"dost/internal/config"
	"dost/internal/repository"
	"dost/internal/service"
	"dost/internal/service/coder"
	"dost/internal/service/orchestrator"
	"dost/internal/service/planner"
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
	if len(args) == 0 {
		fmt.Println("No query provided. Please provide a task or question.")
		return
	}

	query := strings.Join(args, " ")

	if err := orchestrator.InitializeEnhancedOrchestrator(); err != nil {
		fmt.Printf("Failed to initialize orchestrator: %v\n", err)
		return
	}

	result := orchestrator.ProcessUserQuery(query)

	handleOrchestratorResponse(result)
}

func handleOrchestratorResponse(result map[string]any) {
	if result["error"] != nil {
		fmt.Printf("Error: %v\n", result["error"])
		return
	}

	service.PutJSONCache(result)

	status, _ := result["status"].(string)
	fmt.Printf("Status: %s\n", status)

	switch status {
	case "execution_completed":
		handleExecutionCompleted(result)
	case "parallel_execution_completed":
		handleParallelExecution(result)
	case "completed":
		handleSimpleCompletion(result)
	case "subdivided":
		handleSubdivision(result)
	default:
		handleGenericResponse(result)
	}

	processFunctionCalls(result)
}

func handleExecutionCompleted(result map[string]any) {
	fmt.Println("\n🎯 Task Execution Completed")

	if executionID, ok := result["execution_id"].(string); ok {
		fmt.Printf("Execution ID: %s\n", executionID)
	}

	if results, ok := result["results"].([]map[string]any); ok {
		fmt.Printf("Subtasks completed: %d\n", len(results))

		for i, subResult := range results {
			fmt.Printf("\nSubtask %d:\n", i+1)
			if subResult["error"] == nil {
				fmt.Printf("  ✓ Success")
				if output := subResult["output"]; output != nil {
					fmt.Printf(" - Output: %v\n", formatOutput(output))
				}
			} else {
				fmt.Printf("  ✗ Failed: %v\n", subResult["error"])
			}
		}
	}

	if workflow, ok := result["workflow"].(*orchestrator.WorkflowState); ok {
		fmt.Printf("\nWorkflow: %s (Status: %s)\n", workflow.ID, workflow.Status)
	}
}

func handleParallelExecution(result map[string]any) {
	fmt.Println("\n⚡ Parallel Task Execution Completed")

	if executionID, ok := result["execution_id"].(string); ok {
		fmt.Printf("Execution ID: %s\n", executionID)
	}

	if finalResult, ok := result["final_result"].(map[string]any); ok {
		successCount := finalResult["success_count"].(int)
		totalCount := finalResult["total_count"].(int)

		fmt.Printf("Success Rate: %d/%d tasks completed successfully\n", successCount, totalCount)

		if outputs, ok := finalResult["outputs"].([]any); ok && len(outputs) > 0 {
			fmt.Println("\nCombined Results:")
			for i, output := range outputs {
				fmt.Printf("  Result %d: %v\n", i+1, formatOutput(output))
			}
		}

		if errors, ok := finalResult["errors"].([]any); ok && len(errors) > 0 {
			fmt.Println("\nErrors encountered:")
			for i, err := range errors {
				fmt.Printf("  Error %d: %v\n", i+1, err)
			}
		}
	}
}

func handleSimpleCompletion(result map[string]any) {
	fmt.Println("\n✓ Task Completed")

	if agentID, ok := result["agent_id"].(string); ok {
		fmt.Printf("Executed by agent: %s\n", agentID)
	}

	if output := result["output"]; output != nil {
		fmt.Printf("Result: %v\n", formatOutput(output))
	}

	if cacheHit, ok := result["cache_hit"].(bool); ok && cacheHit {
		fmt.Println("(Result retrieved from cache)")
	}
}

func handleSubdivision(result map[string]any) {
	if subdivisionResult, ok := result["sub_tasks"].([]orchestrator.SubTask); ok {
		fmt.Printf("Task subdivided into %d subtasks.\n", len(subdivisionResult))
		fmt.Println("Starting execution of subtasks...")
		orchestrator.ExecuteTaskWithSubdivision(map[string]any{
			"task":               result["original_task"],
			"subdivision_result": result,
		})
	}
}

func handleGenericResponse(result map[string]any) {
	if output := result["output"]; output != nil {
		fmt.Printf("Output: %v\n", formatOutput(output))
	}

	if context := result["context"]; context != nil {
		fmt.Printf("Context used: %v\n", context)
	}

	if cacheInfo := result["cached_response_id"]; cacheInfo != nil {
		fmt.Printf("Response cached as: %v\n", cacheInfo)
	}
}

func processFunctionCalls(result map[string]any) {
	if outputs, ok := result["output"].([]map[string]any); ok {
		availableFunctions := getAvailableAgentFunctions()

		for _, output := range outputs {
			if funcName, hasFunc := output["function_name"].(string); hasFunc {
				if params, hasParams := output["parameters"].(map[string]any); hasParams {
					fmt.Printf("\n🔧 Executing function: %s\n", funcName)

					if fn, exists := availableFunctions[funcName]; exists {
						functionResult := fn.Service(params)

						if functionResult["error"] == nil {
							fmt.Printf("✓ Function executed successfully\n")
							if status := functionResult["status"]; status != nil {
								fmt.Printf("Status: %v\n", status)
							}
							service.PutJSONCache(functionResult)
							displayFunctionResult(funcName, functionResult)
						} else {
							fmt.Printf("✗ Function failed: %v\n", functionResult["error"])
						}
					} else {
						fmt.Printf("✗ Unknown function: %s\n", funcName)
					}
				}
			}
		}
	}
}

// getAvailableAgentFunctions collects all function declarations from all registered live agents.
func getAvailableAgentFunctions() map[string]repository.Function {
	allFunctions := make(map[string]repository.Function)
	orchestratorFunctions := orchestrator.GetEnhancedOrchestratorCapabilitiesMap()
	for name, fn := range orchestratorFunctions {
		allFunctions[name] = fn
	}

	for range orchestrator.RegisteredLiveAgents {

		for name, fn := range planner.GetPlannerCapabilitiesMap() {
			allFunctions[name] = fn
		}

		for name, fn := range coder.GetCoderCapabilitiesMap() {
			allFunctions[name] = fn
		}

	}
	return allFunctions
}

func displayFunctionResult(funcName string, result map[string]any) {
	switch funcName {
	case "register_agents_enhanced":
		if agents, ok := result["registered_agents"].([]string); ok {
			fmt.Printf("Registered agents (%d):\n", len(agents))
			for _, agentID := range agents {
				fmt.Printf("  - %s\n", agentID)
			}
		}

	case "subdivide_task":
		if subTasks, ok := result["sub_tasks"].([]orchestrator.SubTask); ok {
			fmt.Printf("Task subdivided into %d subtasks:\n", len(subTasks))
			for i, subTask := range subTasks {
				fmt.Printf("  %d. %s (Type: %s, Agent: %s)\n",
					i+1, subTask.Description, subTask.Type, subTask.AgentType)
			}
		}

	case "execute_task_with_subdivision":
		handleTaskExecutionResult(result)

	case "get_cache_statistics":
		displayCacheStatistics(result)

	case "get_task_execution_status":
		displayExecutionStatus(result)

	case "parallel_execute_tasks":
		handleParallelExecutionResult(result)

	case "create_workflow":
		handleWorkflowCreation(result)

	case "execute_workflow":
		handleWorkflowExecution(result)

	case "decompose_task":
		displayPlannerBreakdown(result)

	default:
		if status := result["status"]; status != nil {
			fmt.Printf("Result: %v\n", status)
		}
		if message := result["message"]; message != nil {
			fmt.Printf("Message: %v\n", message)
		}
	}
}

func displayPlannerBreakdown(result map[string]any) {
	fmt.Println("\n📋 Planner's Breakdown:")
	if breakdownID, ok := result["breakdown_id"].(string); ok {
		fmt.Printf("Breakdown ID: %s\n", breakdownID)
	}
	if strategy, ok := result["strategy"].(string); ok {
		fmt.Printf("Strategy: %s\n", strategy)
	}
	if complexity, ok := result["complexity"].(string); ok {
		fmt.Printf("Complexity: %s\n", complexity)
	}
	fmt.Println("\nSubtasks:")
	if subtasks, ok := result["subtasks"].([]planner.SubTask); ok {
		for _, subtask := range subtasks {
			fmt.Printf("  - [ID: %s] %s\n", subtask.ID, subtask.Description)
			fmt.Printf("    Agent: %s, Status: %s\n", subtask.RequiredAgent, subtask.Status)
			if len(subtask.Dependencies) > 0 {
				fmt.Printf("    Dependencies: %v\n", subtask.Dependencies)
			}
			fmt.Println()
		}
	}
}

func handleTaskExecutionResult(result map[string]any) {
	if executionID, ok := result["execution_id"].(string); ok {
		fmt.Printf("Task execution started: %s\n", executionID)
	}

	if subTasks, ok := result["sub_tasks"].([]orchestrator.SubTask); ok {
		fmt.Printf("Subdivided into %d subtasks\n", len(subTasks))
	}

	if executionResults, ok := result["execution_results"].([]map[string]any); ok {
		fmt.Printf("Execution results: %d completed\n", len(executionResults))

		successCount := 0
		for _, execResult := range executionResults {
			if execResult["error"] == nil {
				successCount++
			}
		}
		fmt.Printf("Success rate: %d/%d\n", successCount, len(executionResults))
	}
}

func handleParallelExecutionResult(result map[string]any) {
	fmt.Println("\n🚀 Parallel Execution Started")

	if executionID, ok := result["execution_id"].(string); ok {
		fmt.Printf("Execution ID: %s\n", executionID)
	}

	if taskCount, ok := result["task_count"].(int); ok {
		fmt.Printf("Tasks queued for parallel execution: %d\n", taskCount)
	}

	if message, ok := result["message"].(string); ok {
		fmt.Printf("Status: %s\n", message)
	}
}

func handleWorkflowCreation(result map[string]any) {
	fmt.Println("\n📋 Workflow Created")

	if workflowID, ok := result["workflow_id"].(string); ok {
		fmt.Printf("Workflow ID: %s\n", workflowID)
	}

	if stepCount, ok := result["step_count"].(int); ok {
		fmt.Printf("Workflow steps: %d\n", stepCount)
	}

	if status, ok := result["status"].(string); ok {
		fmt.Printf("Status: %s\n", status)
	}
}

func handleWorkflowExecution(result map[string]any) {
	fmt.Println("\n⚙️ Workflow Execution")

	if workflowID, ok := result["workflow_id"].(string); ok {
		fmt.Printf("Workflow ID: %s\n", workflowID)
	}

	if executionID, ok := result["execution_id"].(string); ok {
		fmt.Printf("Execution ID: %s\n", executionID)
	}

	if progress, ok := result["progress"].(map[string]any); ok {
		if completed, ok := progress["completed"].(int); ok {
			if total, ok := progress["total"].(int); ok {
				fmt.Printf("Progress: %d/%d steps\n", completed, total)
			}
		}
	}
}

func displayCacheStatistics(stats map[string]any) {
	fmt.Println("\n📊 Cache Statistics:")

	if totalResponses, ok := stats["total_responses"].(int); ok {
		fmt.Printf("Total cached responses: %d\n", totalResponses)
	}

	if sizeMB, ok := stats["total_size_mb"].(float64); ok {
		fmt.Printf("Cache size: %.2f MB\n", sizeMB)
	}

	if successRate, ok := stats["success_rate"].(float64); ok {
		fmt.Printf("Success rate: %.1f%%\n", successRate*100)
	}

	if agentDist, ok := stats["agent_distribution"].(map[string]int); ok {
		fmt.Println("Agent distribution:")
		for agentID, count := range agentDist {
			fmt.Printf("  %s: %d responses\n", agentID, count)
		}
	}

	if config, ok := stats["config"].(map[string]any); ok {
		fmt.Println("Cache configuration:")
		if maxResponses, ok := config["max_responses"].(int); ok {
			fmt.Printf("  Max responses: %d\n", maxResponses)
		}
		if maxAge, ok := config["max_age"].(string); ok {
			fmt.Printf("  Max age: %s\n", maxAge)
		}
	}
}

func displayExecutionStatus(status map[string]any) {
	fmt.Println("\n📋 Execution Status:")

	if executionID, ok := status["execution_id"].(string); ok {
		fmt.Printf("Execution ID: %s\n", executionID)
	}

	if progress, ok := status["progress"].(float64); ok {
		fmt.Printf("Progress: %.1f%%\n", progress*100)
	}

	if completed, ok := status["completed"].(int); ok {
		if total, ok := status["total_tasks"].(int); ok {
			fmt.Printf("Completed: %d/%d tasks\n", completed, total)
		}
	}

	if failed, ok := status["failed"].(int); ok && failed > 0 {
		fmt.Printf("Failed tasks: %d\n", failed)
	}

	if pending, ok := status["pending"].(int); ok && pending > 0 {
		fmt.Printf("Pending tasks: %d\n", pending)
	}

	if errors, ok := status["errors"].([]any); ok && len(errors) > 0 {
		fmt.Println("Recent errors:")
		maxErrors := len(errors)
		if maxErrors > 3 {
			maxErrors = 3
		}
		for i := 0; i < maxErrors; i++ {
			fmt.Printf("  - %v\n", errors[i])
		}
		if len(errors) > 3 {
			fmt.Printf("  ... and %d more errors\n", len(errors)-3)
		}
	}

	if startTime, ok := status["start_time"].(string); ok {
		fmt.Printf("Started: %s\n", startTime)
	}

	if estimatedCompletion, ok := status["estimated_completion"].(string); ok {
		fmt.Printf("Estimated completion: %s\n", estimatedCompletion)
	}
}

func formatOutput(output any) string {
	switch v := output.(type) {
	case string:
		if len(v) > 500 {
			return v[:500] + "...[truncated]"
		}
		return v

	case []map[string]any:
		if len(v) == 0 {
			return "[]"
		}

		if len(v) == 1 {
			if text, exists := v[0]["text"].(string); exists {
				if len(text) > 500 {
					return text[:500] + "...[truncated]"
				}
				return text
			}
		}

		return fmt.Sprintf("[%d items]", len(v))

	case map[string]any:
		if len(v) == 0 {
			return "{}"
		}

		if text, exists := v["text"].(string); exists {
			if len(text) > 500 {
				return text[:500] + "...[truncated]"
			}
			return text
		}

		if content, exists := v["content"].(string); exists {
			if len(content) > 500 {
				return content[:500] + "...[truncated]"
			}
			return content
		}

		if status, exists := v["status"].(string); exists {
			if message, exists := v["message"].(string); exists {
				return fmt.Sprintf("Status: %s - %s", status, message)
			}
			return fmt.Sprintf("Status: %s", status)
		}

		if message, exists := v["message"].(string); exists {
			if len(message) > 500 {
				return message[:500] + "...[truncated]"
			}
			return message
		}

		return fmt.Sprintf("{%d fields}", len(v))

	case []any:
		if len(v) == 0 {
			return "[]"
		}

		if len(v) > 0 {
			if str, ok := v[0].(string); ok {
				if len(v) == 1 {
					if len(str) > 500 {
						return str[:500] + "...[truncated]"
					}
					return str
				}
				return fmt.Sprintf("[%d strings]", len(v))
			}
		}

		return fmt.Sprintf("[%d items]", len(v))

	default:
		str := fmt.Sprintf("%v", v)
		if len(str) > 500 {
			return str[:500] + "...[truncated]"
		}
		return str
	}
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

	if err := orchestrator.InitializeOrchestratorContext(); err != nil {
		fmt.Printf("Warning: Failed to initialize orchestrator context: %v\n", err)
	}

	if err := orchestrator.InitializeResponseCache(nil); err != nil {
		fmt.Printf("Warning: Failed to initialize response cache: %v\n", err)
	}

	orchestrator.StartCacheMaintenanceScheduler()

	fmt.Println("DOST initialized successfully")
}
