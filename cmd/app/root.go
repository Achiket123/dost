package app

import (
	"dost/internal/config"
	"dost/internal/handler"
	"dost/internal/repository"
	"dost/internal/service"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	Cfg     *config.Config
)

var INITIAL_CONTEXT = 1

const MAXIMUM_CONTEXT_AWARENESS = 5

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "dost",
	Short: "A brief description of your application",
	Long: `
DOST is an AI CLI  tool for Developers that empowers them to create and organize their projects.
It is designed to be simple and easy to use, with a focus on developer productivity.

You can use it to manage your projects, run commands, and automate tasks.
USING GEMINI API: DOST uses the Gemini API to provide AI-powered features.
It can help you with code generation, code completion, and more.

SETUP: To set up DOST, you need to create a configuration file in  directory add the path to DOST.
The configuration file should be named ".dost.yaml" and should contain your API key and other settings.
The full path to the configuration file should be provided in the "config" flag.
`,
	Args: cobra.ArbitraryArgs,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	initConfig()
	cobra.OnInitialize(initConfig)

	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Please provide a prompt as the first argument")
			return
		}

		originalTask := strings.Join(args, " ")

		// Initialize JSON cache system
		if err := service.InitializeCache(); err != nil {
			fmt.Printf("Error initializing cache: %v\n", err)
			return
		}

		// Set system instruction in cache
		service.SetSystemInstruction(repository.Instructions)

		// Initialize enhanced task tracker with time-based controls
		tracker := &repository.TaskTracker{
			OriginalTask:               originalTask,
			CurrentPhase:               "INITIALIZATION",
			CompletedSteps:             []string{},
			PendingSteps:               []string{},
			FilesCreated:               []string{},
			FilesModified:              []string{},
			CommandsRun:                []string{},
			FailedCommands:             []string{},
			Iteration:                  0,
			TaskCompleted:              false,
			ConsecutiveReadCalls:       0,
			ConsecutiveSameCommands:    0,
			LastActionType:             "",
			StuckCounter:               0,
			ErrorsFound:                false,
			BuildAttempted:             false,
			LastFunctionCall:           "",
			LastError:                  "",
			ErrorCount:                 0,
			LastSuccessfulCommand:      "",
			DependencyErrors:           []string{},
			RequiresFixing:             false,
			ProjectStructureRetrieved:  false,
			EmptyResponseCount:         0,
			ConversationResetCount:     0,
			LastFunctionResponse:       nil,
			FunctionCallsThisIteration: 0,
			ConsecutiveFailures:        0,
			StartTime:                  time.Now(),
			LastSuccessTime:            time.Now(),
			MaxExecutionTime:           10 * time.Minute, // Maximum 10 minutes execution
			IdleTime:                   0,
		}

		service.PutJSONCache(map[string]any{"user": originalTask, "tracker": tracker})

		// Add initial user message to conversation history
		service.AddUserMessage(originalTask)

		GoogleAI := handler.NewGoogleAI(Cfg, originalTask)
		functionalActionsCount := 0

		fmt.Printf("🎯 TASK: %s\n", originalTask)
		fmt.Printf("📋 Starting autonomous agent...\n")

		// Event-driven execution loop without fixed iterations
		for {
			tracker.Iteration++
			tracker.FunctionCallsThisIteration = 0
			// iterationStartTime := time.Now()

			// Check termination conditions first
			if repository.ShouldTerminate(tracker) {
				break
			}

			// Handle stuck states
			if isStuckInAdvancedLoop(tracker) {
				fmt.Printf("⚠️ ADVANCED LOOP DETECTED! Taking corrective action...\n")
				if !handler.HandleAdvancedStuckState(tracker, &GoogleAI) {
					fmt.Printf("🛑 Unable to recover from stuck state. Terminating.\n")
					break
				}
			}

			tracker.CurrentPhase = determineEnhancedPhase(tracker, functionalActionsCount)
			fmt.Printf("\n=== ITERATION %d - PHASE: %s ===\n", tracker.Iteration, tracker.CurrentPhase)

			// Enhanced contextual prompt with error analysis
			contextualPrompt := buildEnhancedContextualPrompt(tracker, originalTask)
			GoogleAI.Reset(contextualPrompt)

			returns := service.RequestTool(getArgsWithConversationHistory())

			// Handle API errors with exponential backoff
			if returns["error"] != nil {
				errorStr := fmt.Sprintf("%v", returns["error"])
				fmt.Printf("❌ API Error: %s\n", errorStr)

				if strings.Contains(errorStr, "429") || strings.Contains(errorStr, "rate limit") || strings.Contains(errorStr, "quota") {
					fmt.Println("🚫 RATE LIMITED - Please wait and try again later.")
					return
				}

				tracker.ConsecutiveFailures++
				if tracker.ConsecutiveFailures > 3 {
					fmt.Printf("🛑 Too many consecutive API failures. Terminating.\n")
					break
				}

				sleepTime := time.Duration(tracker.ConsecutiveFailures*5) * time.Second
				fmt.Printf("⏳ Waiting %v seconds before retrying...\n", sleepTime)
				time.Sleep(sleepTime)
				continue
			}

			// Reset consecutive failures on successful API call
			tracker.ConsecutiveFailures = 0

			// Process AI response with new conversation flow rules
			iterationActions := processAIResponseWithProperFlow(returns, tracker, GoogleAI)
			functionalActionsCount += iterationActions

			// Update progress tracking
			if iterationActions > 0 {
				tracker.LastSuccessTime = time.Now()
				tracker.IdleTime = 0
			} else {
				tracker.IdleTime = time.Since(tracker.LastSuccessTime)
			}

			fmt.Printf("📊 Actions this iteration: %d | Total functional actions: %d\n", tracker.FunctionCallsThisIteration, functionalActionsCount)
			fmt.Printf("📈 Progress: %s\n", generateEnhancedProgressSummary(tracker))
			fmt.Printf("⏱️ Execution time: %v | Idle time: %v\n", time.Since(tracker.StartTime), tracker.IdleTime)

			// Enhanced task completion detection
			tracker.TaskCompleted = repository.EvaluateTaskCompletionWithErrorAnalysis(tracker, returns, originalTask)

			if tracker.TaskCompleted {
				fmt.Printf("\n✅ TASK COMPLETED SUCCESSFULLY!\n")
				printTaskSummary(tracker, functionalActionsCount)
				break
			}

			// Check if we should continue or pause for user input
			if repository.ShouldPauseForUserInput(tracker) {
				fmt.Printf("\n🤔 Task seems complex or stuck. Continue? (y/N): ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Printf("🛑 User requested termination.\n")
					break
				}
				tracker.StuckCounter = 0 // Reset after user confirmation
			}
		}

		// Final status report
		executionTime := time.Since(tracker.StartTime)
		if !tracker.TaskCompleted {
			fmt.Printf("\n⚠️ Task terminated after %v\n", executionTime)
			fmt.Printf("📊 Final stats: %d functional actions across %d iterations\n", functionalActionsCount, tracker.Iteration)
			printTaskSummary(tracker, functionalActionsCount)
		}
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file default is C:/.dost.yaml")
	rootCmd.PersistentFlags().String("ai", "", "AI query for Gemini API")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Enhanced termination conditions - the main logic for when to stop

// Print comprehensive task summary
func printTaskSummary(tracker *repository.TaskTracker, functionalActionsCount int) {
	fmt.Printf("📋 Task Summary:\n")
	fmt.Printf("   - Duration: %v\n", time.Since(tracker.StartTime))
	fmt.Printf("   - Iterations: %d\n", tracker.Iteration)
	fmt.Printf("   - Files created: %v\n", tracker.FilesCreated)
	fmt.Printf("   - Files modified: %v\n", tracker.FilesModified)
	fmt.Printf("   - Commands executed: %v\n", tracker.CommandsRun)
	fmt.Printf("   - Failed commands: %v\n", tracker.FailedCommands)
	fmt.Printf("   - Total functional actions: %d\n", functionalActionsCount)
	fmt.Printf("   - Error count: %d\n", tracker.ErrorCount)
	fmt.Printf("   - Final phase: %s\n", tracker.CurrentPhase)

	// Success rate calculation
	totalCommands := len(tracker.CommandsRun) + len(tracker.FailedCommands)
	if totalCommands > 0 {
		successRate := float64(len(tracker.CommandsRun)) / float64(totalCommands) * 100
		fmt.Printf("   - Command success rate: %.1f%%\n", successRate)
	}
}

// New AI response processing with proper conversation flow and FIXED function call/response synchronization
func processAIResponseWithProperFlow(returns map[string]any, tracker *repository.TaskTracker, GoogleAI *handler.GoogleAI) int {
	actionCount := 0

	if returns["output"] == nil {
		fmt.Println("⚠️ No output field in response")
		tracker.EmptyResponseCount++
		return actionCount
	}

	// Handle different response formats
	switch output := returns["output"].(type) {
	case []map[string]any:
		return processOutputArrayWithProperFlow(output, tracker, GoogleAI)
	case []interface{}:
		return processGenericArrayWithProperFlow(output, tracker, GoogleAI)
	case string:
		return processStringOutputWithProperFlow(output, tracker)
	case map[string]any:
		// Handle single map response
		singleOutput := []map[string]any{output}
		return processOutputArrayWithProperFlow(singleOutput, tracker, GoogleAI)
	default:
		fmt.Printf("⚠️ Unexpected output type: %T\n", output)
		tracker.EmptyResponseCount++
		return 0
	}
}

// FIXED: Proper function call and response synchronization
func processOutputArrayWithProperFlow(outputs []map[string]any, tracker *repository.TaskTracker, GoogleAI *handler.GoogleAI) int {
	actionCount := 0
	hasText := false
	hasFunction := false
	lastText := ""

	// Separate function calls and collect them
	var functionCalls []service.FunctionCallData
	var functionResults []map[string]any

	// Check if output is empty array
	if len(outputs) == 0 {
		fmt.Println("⚠️ Empty output array - skipping")
		tracker.EmptyResponseCount++
		return 0
	}

	// Process each output item
	for i, v := range outputs {
		fmt.Printf("Processing output %d: %+v\n", i, v)

		// Process text content first
		if text, exists := v["text"]; exists && text != nil {
			textStr := fmt.Sprintf("%v", text)
			cleanText := cleanAndValidateText(textStr)

			if cleanText != "" && cleanText != "[]" {
				fmt.Printf("AI Response: %s\n", cleanText)
				tracker.LastAIResponse = cleanText
				lastText = cleanText
				hasText = true
			}
		}

		// Process function calls - collect them first, execute later
		if functionName, exists := v["function_name"]; exists && functionName != nil {
			funcNameStr, ok := functionName.(string)
			if !ok || funcNameStr == "" {
				fmt.Printf("⚠️ Invalid function name at index %d\n", i)
				continue
			}

			// Get parameters with better error handling
			parameters := make(map[string]any)
			if params, exists := v["parameters"]; exists {
				if paramMap, ok := params.(map[string]any); ok {
					parameters = paramMap
				} else if paramStr, ok := params.(string); ok && paramStr != "" {
					if err := json.Unmarshal([]byte(paramStr), &parameters); err != nil {
						fmt.Printf("⚠️ Failed to parse parameters: %v\n", err)
					}
				}
			}

			// Add to function calls list
			functionCalls = append(functionCalls, service.FunctionCallData{
				Name: funcNameStr,
				Args: parameters,
			})

			hasFunction = true
		}
	}

	// First, add model response with function calls if we have both text and functions
	if hasText && hasFunction {
		// Add model response with both text and function calls
		service.AddModelResponse(lastText, functionCalls)
	} else if hasFunction && !hasText {
		// Add model response with only function calls (empty text)
		service.AddModelResponse("", functionCalls)
	} else if hasText && !hasFunction {
		// Add model response with only text (no function calls)
		service.AddModelResponse(lastText, []service.FunctionCallData{})
	}

	// Now execute the function calls and collect results
	if hasFunction {
		for _, funcCall := range functionCalls {
			fmt.Printf("🔧 Executing function: %s with params: %+v\n", funcCall.Name, funcCall.Args)

			// Execute the function and get result
			result := executeFunctionCallWithResult(funcCall.Name, map[string]any{
				"function_name": funcCall.Name,
				"parameters":    funcCall.Args,
			}, tracker, GoogleAI)

			actionCount++
			tracker.FunctionCallsThisIteration++

			// Store function result for conversation
			functionResults = append(functionResults, map[string]any{
				"name":       funcCall.Name,
				"parameters": funcCall.Args,
				"result":     result,
			})

			tracker.LastFunctionCall = funcCall.Name
			tracker.EmptyResponseCount = 0
		}

		// Add function responses to conversation
		addFunctionResponsesToConversation(functionResults, tracker)
	} else if hasText {
		// Only text response - add user guidance
		addUserGuidanceAfterTextOnly(tracker)
	} else {
		// Empty response - skip and increment counter
		fmt.Println("⚠️ Empty or invalid response detected - skipping")
		tracker.EmptyResponseCount++
		return 0
	}

	return actionCount
}

func processGenericArrayWithProperFlow(outputs []interface{}, tracker *repository.TaskTracker, GoogleAI *handler.GoogleAI) int {
	actionCount := 0

	for i, item := range outputs {
		if itemMap, ok := item.(map[string]any); ok {
			singleOutput := []map[string]any{itemMap}
			actionCount += processOutputArrayWithProperFlow(singleOutput, tracker, GoogleAI)
		} else {
			fmt.Printf("⚠️ Unexpected item type at index %d: %T\n", i, item)
		}
	}

	return actionCount
}

func processStringOutputWithProperFlow(output string, tracker *repository.TaskTracker) int {
	cleanText := cleanAndValidateText(output)

	// Handle empty responses properly
	if cleanText == "" || cleanText == "[]" {
		fmt.Println("⚠️ Empty string output detected - skipping")
		tracker.EmptyResponseCount++
		return 0
	}

	fmt.Printf("AI Text Response: %s\n", cleanText)
	tracker.LastAIResponse = cleanText
	tracker.EmptyResponseCount = 0

	// Add model response then user guidance
	service.AddModelResponse(cleanText, []service.FunctionCallData{})
	addUserGuidanceAfterTextOnly(tracker)

	return 0
}

// FIXED: Separate function for adding function responses
func addFunctionResponsesToConversation(functionResults []map[string]any, tracker *repository.TaskTracker) {
	parts := []map[string]any{}

	// Add function response parts
	for _, funcResult := range functionResults {
		functionResponsePart := map[string]any{
			"functionResponse": map[string]any{
				"name": funcResult["name"],
				"response": map[string]any{
					"content": funcResult["result"],
				},
			},
		}
		parts = append(parts, functionResponsePart)
	}

	// Add guidance text part
	guidanceText := buildGuidanceText(tracker)
	textPart := map[string]any{
		"text": guidanceText,
	}
	parts = append(parts, textPart)

	// Add user message with both function responses and guidance
	if err := service.AddUserMessageWithParts(parts); err != nil {
		fmt.Printf("⚠️ Error adding user message with parts: %v\n", err)
	}
}

// Add user guidance after text-only responses
func addUserGuidanceAfterTextOnly(tracker *repository.TaskTracker) {
	guidanceText := buildGuidanceText(tracker)
	if err := service.AddUserMessage(guidanceText); err != nil {
		fmt.Printf("⚠️ Error adding user guidance message: %v\n", err)
	}
}

// Build appropriate guidance text based on current state
func buildGuidanceText(tracker *repository.TaskTracker) string {
	if tracker.TaskCompleted {
		return "If you think the task is complete, call exit_process(). Otherwise, continue with tool calls. Don't reply with filler text."
	}

	// Provide specific guidance based on current state
	var guidance strings.Builder

	if len(tracker.FilesCreated) == 0 && len(tracker.FilesModified) == 0 {
		guidance.WriteString("Create or modify necessary files using create_files or write_file functions. ")
	} else if len(tracker.CommandsRun) == 0 {
		guidance.WriteString("Test your implementation by executing build/run commands using terminal_execute. ")
	}

	guidance.WriteString("If you think the task is complete, call exit_process(). Otherwise, continue with tool calls. Don't reply with filler text.")

	return guidance.String()
}

// Enhanced function execution that returns results
func executeFunctionCallWithResult(function_name string, v map[string]any, tracker *repository.TaskTracker, GoogleAI *handler.GoogleAI) map[string]any {
	parameters, ok := v["parameters"].(map[string]any)
	if !ok {
		parameters = make(map[string]any)
	}

	var result map[string]any

	switch function_name {
	case handler.REQUEST_AI_TOOL:
		handleRequestAIToolEnhanced(parameters, tracker, GoogleAI)
		result = map[string]any{"status": "ai_tool_requested"}

	case handler.GET_PROJECT_STRUCTURE:
		temp_func := handler.GeminiTools()[function_name]
		result = temp_func.Run(parameters)
		if result["error"] == nil {
			tracker.CompletedSteps = append(tracker.CompletedSteps, "Retrieved project structure")
			tracker.ProjectStructureRetrieved = true
		} else {
			tracker.ErrorCount++
			tracker.LastError = fmt.Sprintf("Failed to get project structure: %v", result["error"])
		}
		fmt.Printf("🗂️ Project structure retrieved\n")

	case handler.CREATE_FILES:
		temp_func := handler.GeminiTools()[function_name]
		result = temp_func.Run(parameters)

		if result["error"] == nil {
			if fileNames, ok := parameters["file_names"].([]interface{}); ok {
				for _, fileName := range fileNames {
					if name, ok := fileName.(string); ok {
						tracker.FilesCreated = append(tracker.FilesCreated, name)
					}
				}
			}
			tracker.CompletedSteps = append(tracker.CompletedSteps, "Created file(s)")
		} else {
			tracker.ErrorCount++
			tracker.LastError = fmt.Sprintf("File creation failed: %v", result["error"])
		}

	case handler.WRITE_FILE:
		temp_func := handler.GeminiTools()[function_name]
		result = temp_func.Run(parameters)

		if result["error"] == nil {
			if fileNames, ok := parameters["file_names"].([]interface{}); ok {
				for _, fileName := range fileNames {
					if name, ok := fileName.(string); ok {
						tracker.FilesModified = append(tracker.FilesModified, name)
					}
				}
			}
			tracker.CompletedSteps = append(tracker.CompletedSteps, "Modified file(s)")
		} else {
			tracker.ErrorCount++
			tracker.LastError = fmt.Sprintf("File modification failed: %v", result["error"])
		}

	case handler.TERMINAL_EXECUTE:
		temp_func := handler.GeminiTools()[function_name]
		result = temp_func.Run(parameters)

		if command, ok := parameters["command"].(string); ok {
			if result["error"] == nil {
				tracker.CommandsRun = append(tracker.CommandsRun, command)
				tracker.CompletedSteps = append(tracker.CompletedSteps, fmt.Sprintf("Successfully executed: %s", command))
				tracker.BuildAttempted = true
				tracker.LastSuccessfulCommand = command
				tracker.ConsecutiveSameCommands = 0

				// Check for build/compile commands
				cmdLower := strings.ToLower(command)
				if strings.Contains(cmdLower, "cmake") || strings.Contains(cmdLower, "make") ||
					strings.Contains(cmdLower, "build") || strings.Contains(cmdLower, "compile") {
					tracker.CurrentPhase = "ANALYSIS_AFTER_TEST"
				}
			} else {
				tracker.FailedCommands = append(tracker.FailedCommands, command)
				tracker.ErrorCount++
				errorStr := fmt.Sprintf("%v", result["error"])
				tracker.LastError = errorStr

				// Check for repeated failures
				if len(tracker.FailedCommands) > 1 &&
					tracker.FailedCommands[len(tracker.FailedCommands)-1] ==
						tracker.FailedCommands[len(tracker.FailedCommands)-2] {
					tracker.ConsecutiveSameCommands++
				}

				// Analyze error for dependencies
				if strings.Contains(errorStr, "No such file or directory") ||
					strings.Contains(errorStr, "not found") {
					if strings.Contains(errorStr, ".h") || strings.Contains(errorStr, "glfw") ||
						strings.Contains(errorStr, "glad") || strings.Contains(errorStr, "opengl") {
						depError := fmt.Sprintf("Dependency error: %s", errorStr)
						tracker.DependencyErrors = append(tracker.DependencyErrors, depError)
						tracker.RequiresFixing = true
					}
				}

				fmt.Printf("❌ Command failed: %s\n", command)
				fmt.Printf("Error: %s\n", errorStr)
			}
		}

	case handler.EXIT_PROCESS:
		tracker.TaskCompleted = true
		tracker.CompletedSteps = append(tracker.CompletedSteps, "Task marked as complete")
		result = map[string]any{"status": "task_completed", "message": "Task has been marked as complete"}

	default:
		if tool, exists := handler.GeminiTools()[function_name]; exists {
			result = tool.Run(parameters)
			if result["error"] != nil {
				tracker.ErrorCount++
				tracker.LastError = fmt.Sprintf("%s failed: %v", function_name, result["error"])
			}
		} else {
			fmt.Printf("⚠️ Unknown function: %s\n", function_name)
			result = map[string]any{"error": fmt.Sprintf("Unknown function: %s", function_name)}
		}
		tracker.CompletedSteps = append(tracker.CompletedSteps, fmt.Sprintf("Executed %s", function_name))
	}

	return result
}

func cleanAndValidateText(text string) string {
	if text == "" {
		return ""
	}

	cleanText := strings.TrimSpace(text)

	if cleanText == "[]" || cleanText == "[map[]]" || cleanText == "[[]]" {
		return ""
	}

	if strings.HasPrefix(cleanText, "[map[") && strings.HasSuffix(cleanText, "]]") {
		extracted := extractContentFromMapString(cleanText)
		if extracted == "[]" || strings.TrimSpace(extracted) == "[]" {
			return ""
		}
		return extracted
	}

	cleanText = strings.ReplaceAll(cleanText, "```", "")
	cleanText = strings.TrimSpace(cleanText)

	if cleanText == "[]" || cleanText == "" {
		return ""
	}

	return cleanText
}

func extractContentFromMapString(mapStr string) string {
	textPattern := `text:([^]]+)`
	re := regexp.MustCompile(textPattern)
	matches := re.FindStringSubmatch(mapStr)

	if len(matches) > 1 {
		content := strings.TrimSpace(matches[1])
		content = strings.ReplaceAll(content, "\\n", "\n")
		content = strings.TrimSpace(content)
		return content
	}

	return ""
}

// Reset conversation history when stuck
func ResetConversationHistory(tracker *repository.TaskTracker, GoogleAI **handler.GoogleAI, originalTask string) {
	tracker.ConversationResetCount++
	tracker.EmptyResponseCount = 0
	tracker.StuckCounter = 0

	fmt.Printf("🔥 Resetting conversation history (reset #%d)\n", tracker.ConversationResetCount)

	service.InitializeCache()
	service.SetSystemInstruction(repository.Instructions)

	freshPrompt := buildFreshPromptAfterReset(tracker, originalTask)
	service.AddUserMessage(freshPrompt)

	*GoogleAI = handler.NewGoogleAI(Cfg, freshPrompt)
}

func buildFreshPromptAfterReset(tracker *repository.TaskTracker, originalTask string) string {
	var promptBuilder strings.Builder

	promptBuilder.WriteString(fmt.Sprintf("TASK: %s\n\n", originalTask))
	promptBuilder.WriteString("CURRENT PROGRESS:\n")

	if tracker.ProjectStructureRetrieved {
		promptBuilder.WriteString("✅ Project structure analyzed\n")
	}

	if len(tracker.FilesCreated) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("✅ Files created: %v\n", tracker.FilesCreated))
	}

	if len(tracker.FilesModified) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("✅ Files modified: %v\n", tracker.FilesModified))
	}

	if len(tracker.CommandsRun) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("✅ Commands executed: %v\n", tracker.CommandsRun))
	}

	if len(tracker.FailedCommands) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("❌ Failed commands: %v\n", tracker.FailedCommands))
	}

	promptBuilder.WriteString("\nNEXT ACTION NEEDED:\n")

	if !tracker.ProjectStructureRetrieved {
		promptBuilder.WriteString("1. Call get_project_structure with path=\".\"\n")
	} else if len(tracker.FilesCreated) == 0 && len(tracker.FilesModified) == 0 {
		promptBuilder.WriteString("1. Create or modify necessary files for the task\n")
	} else if len(tracker.CommandsRun) == 0 {
		promptBuilder.WriteString("1. Test the implementation using terminal_execute\n")
	} else {
		promptBuilder.WriteString("1. Verify the task is complete and call exit_process\n")
	}

	promptBuilder.WriteString("\n🚨 CRITICAL: You must make function calls to complete the task!")
	promptBuilder.WriteString("\n❌ DO NOT respond with empty text or '[]'")
	promptBuilder.WriteString("\n✅ Choose ONE specific function to call now!")

	return promptBuilder.String()
}

// Enhanced loop detection with error analysis
func isStuckInAdvancedLoop(tracker *repository.TaskTracker) bool {
	if tracker.EmptyResponseCount > 5 {
		return true
	}

	if tracker.ConsecutiveSameCommands > 2 {
		return true
	}

	if tracker.ProjectStructureRetrieved && tracker.LastFunctionCall == handler.GET_PROJECT_STRUCTURE {
		fmt.Println("🚨 Repeated get_project_structure call detected!")
		return true
	}

	if len(tracker.DependencyErrors) > 2 {
		return true
	}

	if tracker.ConsecutiveReadCalls > 3 {
		return true
	}

	if len(tracker.FailedCommands) > 3 && tracker.ErrorCount > 5 {
		return true
	}

	if tracker.CurrentPhase == "IMPLEMENTATION" && tracker.Iteration > 10 {
		if len(tracker.CommandsRun) == 0 && len(tracker.FilesCreated) > 0 {
			return true
		}
	}

	return false
}

// Enhanced phase determination
func determineEnhancedPhase(tracker *repository.TaskTracker, functionalActions int) string {
	if !tracker.ProjectStructureRetrieved {
		return "DISCOVERY"
	}

	if len(tracker.DependencyErrors) > 1 || tracker.RequiresFixing {
		return "FORCED_DEPENDENCY_FIX"
	}

	if len(tracker.FailedCommands) > 2 && strings.Contains(strings.Join(tracker.FailedCommands, " "), "cmake") {
		return "FORCED_BUILD_FIX"
	}

	if len(tracker.FilesCreated) > 0 && len(tracker.CommandsRun) == 0 && !tracker.BuildAttempted {
		return "FORCED_BUILD"
	}

	if functionalActions == 0 {
		if tracker.ProjectStructureRetrieved {
			return "ANALYSIS"
		}
		return "DISCOVERY"
	}
	if len(tracker.CommandsRun) > 0 && !tracker.TaskCompleted {
		return "ANALYSIS_AFTER_TEST"
	}
	if len(tracker.FilesCreated) > 0 || len(tracker.FilesModified) > 0 {
		return "TESTING"
	}
	if len(tracker.CompletedSteps) > 0 {
		return "IMPLEMENTATION"
	}
	return "COMPLETION"
}

// Enhanced contextual prompt with error analysis
func buildEnhancedContextualPrompt(tracker *repository.TaskTracker, originalTask string) string {
	var promptBuilder strings.Builder

	promptBuilder.WriteString(fmt.Sprintf("ORIGINAL TASK: %s\n\n", originalTask))
	promptBuilder.WriteString(fmt.Sprintf("CURRENT PHASE: %s\n", tracker.CurrentPhase))
	promptBuilder.WriteString(fmt.Sprintf("ITERATION: %d\n", tracker.Iteration))
	promptBuilder.WriteString(fmt.Sprintf("EXECUTION TIME: %v\n\n", time.Since(tracker.StartTime)))

	if len(tracker.FailedCommands) > 0 {
		promptBuilder.WriteString("🚨 FAILED COMMANDS HISTORY:\n")
		for _, cmd := range tracker.FailedCommands {
			promptBuilder.WriteString(fmt.Sprintf("❌ %s\n", cmd))
		}
		promptBuilder.WriteString("\n")
	}

	if len(tracker.DependencyErrors) > 0 {
		promptBuilder.WriteString("🔗 DEPENDENCY ERRORS DETECTED:\n")
		for _, err := range tracker.DependencyErrors {
			promptBuilder.WriteString(fmt.Sprintf("⚠️ %s\n", err))
		}
		promptBuilder.WriteString("🔌 YOU MUST FIX THE BUILD SYSTEM (CMakeLists.txt, package.json, etc.) TO RESOLVE THESE!\n\n")
	}

	if tracker.LastError != "" {
		promptBuilder.WriteString(fmt.Sprintf("🐛 LAST ERROR: %s\n\n", tracker.LastError))
	}

	if tracker.EmptyResponseCount > 0 {
		promptBuilder.WriteString(fmt.Sprintf("🚨 WARNING: %d empty responses detected!\n", tracker.EmptyResponseCount))
		promptBuilder.WriteString("❌ DO NOT respond with empty text or '[]'\n")
		promptBuilder.WriteString("✅ You MUST make a function call!\n\n")
	}

	// Phase-specific instructions
	switch tracker.CurrentPhase {
	case "FORCED_DEPENDENCY_FIX":
		promptBuilder.WriteString("🔧 FORCED DEPENDENCY FIX PHASE:\n")
		promptBuilder.WriteString("You MUST fix the build configuration file (CMakeLists.txt, package.json, etc.)\n")
		promptBuilder.WriteString("For C++ projects with missing dependencies:\n")
		promptBuilder.WriteString("- Update build files to use proper dependency management\n")
		promptBuilder.WriteString("DO NOT just fix include paths - fix the build system!\n\n")

	case "FORCED_BUILD_FIX":
		promptBuilder.WriteString("🔨 FORCED BUILD FIX PHASE:\n")
		promptBuilder.WriteString("The build keeps failing. You MUST:\n")
		promptBuilder.WriteString("1. Analyze WHY the build fails\n")
		promptBuilder.WriteString("2. Fix the root cause in build files\n")
		promptBuilder.WriteString("3. Do NOT repeat the same failing command!\n\n")

	case "FORCED_BUILD":
		promptBuilder.WriteString("🔧 FORCED BUILD PHASE:\n")
		promptBuilder.WriteString("You MUST execute a build/test command now!\n")
		promptBuilder.WriteString("Use terminal_execute with appropriate commands\n")
		promptBuilder.WriteString("DO NOT read files again - execute commands immediately!\n\n")
	}

	if tracker.ConsecutiveReadCalls > 2 {
		promptBuilder.WriteString("🚨 WARNING: Stop reading files! Take ACTION now!\n")
		promptBuilder.WriteString("You must use write_file, create_file, or terminal_execute.\n\n")
	}

	if tracker.ConsecutiveSameCommands > 1 {
		promptBuilder.WriteString("🔥 WARNING: You're repeating the same failed command!\n")
		promptBuilder.WriteString("Analyze the error and fix the underlying issue first!\n\n")
	}

	// Progress summary
	if len(tracker.CompletedSteps) > 0 {
		promptBuilder.WriteString("COMPLETED STEPS (last 5):\n")
		start := len(tracker.CompletedSteps) - 5
		if start < 0 {
			start = 0
		}
		for i := start; i < len(tracker.CompletedSteps); i++ {
			promptBuilder.WriteString(fmt.Sprintf("✅ %s\n", tracker.CompletedSteps[i]))
		}
		promptBuilder.WriteString("\n")
	}

	if len(tracker.FilesCreated) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("FILES CREATED: %v\n", tracker.FilesCreated))
	}
	if len(tracker.FilesModified) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("FILES MODIFIED: %v\n", tracker.FilesModified))
	}
	if len(tracker.CommandsRun) > 0 {
		promptBuilder.WriteString(fmt.Sprintf("SUCCESSFUL COMMANDS: %v\n", tracker.CommandsRun))
	}

	promptBuilder.WriteString("\n🎯 CRITICAL: Choose ONE specific function call to make progress.")
	promptBuilder.WriteString("\n🚫 DO NOT repeat failed actions without fixing the root cause first!")

	return promptBuilder.String()
}

// Enhanced task completion evaluation

// Enhanced REQUEST_AI_TOOL handler
func handleRequestAIToolEnhanced(parameters map[string]any, tracker *repository.TaskTracker, GoogleAI *handler.GoogleAI) {
	queryInterface, exists := parameters["query"]
	if !exists {
		return
	}

	query, ok := queryInterface.(string)
	if !ok || query == "" {
		query = "Continue with the task based on current context"
	}

	contextualQuery := fmt.Sprintf(
		"ORIGINAL TASK: %s\n"+
			"CURRENT PROGRESS: %s\n"+
			"ITERATION: %d\n"+
			"EXECUTION TIME: %v\n"+
			"ERRORS ENCOUNTERED: %d\n"+
			"FAILED COMMANDS: %v\n"+
			"DEPENDENCY ISSUES: %v\n"+
			"LAST ERROR: %s\n"+
			"EMPTY RESPONSES: %d\n"+
			"WARNING: You must take concrete action with function calls.\n"+
			"🚨 DO NOT respond with empty text or '[]' - make a function call!\n"+
			"QUERY: %s\n",
		tracker.OriginalTask,
		generateEnhancedProgressSummary(tracker),
		tracker.Iteration,
		time.Since(tracker.StartTime),
		tracker.ErrorCount,
		tracker.FailedCommands,
		tracker.DependencyErrors,
		tracker.LastError,
		tracker.EmptyResponseCount,
		query)

	fmt.Printf("🔥 AI requesting continuation with enhanced error context\n")

	service.AddUserMessage(contextualQuery)
	GoogleAI.Reset(contextualQuery)
}

// Enhanced progress summary
func generateEnhancedProgressSummary(tracker *repository.TaskTracker) string {
	summary := fmt.Sprintf("Phase: %s, ", tracker.CurrentPhase)
	summary += fmt.Sprintf("Files created: %d, ", len(tracker.FilesCreated))
	summary += fmt.Sprintf("Files modified: %d, ", len(tracker.FilesModified))
	summary += fmt.Sprintf("Successful commands: %d, ", len(tracker.CommandsRun))
	summary += fmt.Sprintf("Failed commands: %d, ", len(tracker.FailedCommands))
	summary += fmt.Sprintf("Errors: %d, ", tracker.ErrorCount)
	summary += fmt.Sprintf("Empty responses: %d", tracker.EmptyResponseCount)
	return summary
}

// initConfig initializes the configuration
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".dost")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		os.Exit(1)
	}

	var err error
	Cfg, err = config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	getWorkingDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error Setting up DOST: %v\n", err)
		os.Exit(1)
	}

	// Initialize JSON cache system
	path := fmt.Sprintf("%s\\.dost\\cache.json", getWorkingDir)
	os.Mkdir(fmt.Sprintf("%s\\.dost", getWorkingDir), 0755)

	// Create the cache.json file if it doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			fmt.Printf("Error Setting up Dost cache: %v\n", err)
			os.Exit(1)
		}
		file.Close()
	}
}

// getArgsWithConversationHistory builds arguments with conversation history for the AI API
func getArgsWithConversationHistory() map[string]any {
	var systemInstruction map[string]any
	var contents []map[string]any

	// Get conversation history from cache
	conversationHistory := service.GetConversationHistory()

	// Handle system instruction separately from contents
	if initialInstruction, ok := conversationHistory["systemInstruction"].(map[string]any); ok {
		systemInstruction = initialInstruction
	}

	// Handle conversation contents, ensuring correct roles and parts structure
	if historyContents, ok := conversationHistory["contents"].([]service.ConversationContent); ok {
		for _, convContent := range historyContents {
			// Convert each ConversationContent struct to the map format expected by the API
			contentMap := map[string]any{
				"role":  convContent.Role,
				"parts": []map[string]any{},
			}

			for _, part := range convContent.Parts {
				// Append a text part to the parts slice
				if part.Text != "" {
					contentMap["parts"] = append(contentMap["parts"].([]map[string]any), map[string]any{"text": part.Text})
				}
				// Append a functionCall part if it exists
				if part.FunctionCall != nil {
					contentMap["parts"] = append(contentMap["parts"].([]map[string]any), map[string]any{"functionCall": part.FunctionCall})
				}
				// Append a functionResponse part if it exists
				if part.FunctionResponse != nil {
					contentMap["parts"] = append(contentMap["parts"].([]map[string]any), map[string]any{"functionResponse": part.FunctionResponse})
				}
			}
			contents = append(contents, contentMap)
		}
	}

	// Add initial context if it's the first run
	if INITIAL_CONTEXT == 1 {
		INITIAL_CONTEXT = 0
		initialCtx := service.InitialContext(map[string]any{})
		initialCtxBytes, _ := json.Marshal(initialCtx)

		// This should be part of the systemInstruction, not contents
		systemInstruction = map[string]any{
			"parts": []map[string]any{
				{"text": fmt.Sprintf("Initial context:\n%s", string(initialCtxBytes))},
				{"text": repository.Instructions},
			},
		}
	}
	// Build the final request structure
	request := map[string]any{
		"systemInstruction": systemInstruction,
		"contents":          contents,
		"tools": []map[string]any{
			{"function_declarations": handler.GetTerminalToolsMap()},
		},
		"API_KEY": Cfg.AI.API_KEY,
	}

	return request
}
