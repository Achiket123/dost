package planner

import (
	"bufio"
	"bytes"
	"context"
	"dost/internal/repository"
	"dost/internal/service/analysis"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	gitignore "github.com/sabhiram/go-gitignore"

	"github.com/spf13/viper"
)

var ignoreMatcher *gitignore.GitIgnore
var PlannertoolsFunc map[string]repository.Function = make(map[string]repository.Function)

var ChatHistory = make([]map[string]any, 0)
var defaultIgnore = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".venv":        true,
	".env":         true,
	".idea":        true,
	".vscode":      true,
	"__pycache__":  true,
	".dost":        true,
}

type Plans struct {
	PlanID          string   `json:"planId"`
	Title           string   `json:"title"`
	Objective       string   `json:"objective"`
	Assumptions     []string `json:"assumptions"`
	Steps           []Step   `json:"steps"`
	SuccessCriteria []string `json:"successCriteria"`
	Fallbacks       []string `json:"fallbacks"`
}

// Step represents an individual step in the plan.
type Step struct {
	ID           int      `json:"id"`
	Description  string   `json:"description"`
	Agent        string   `json:"agent"`
	Inputs       []string `json:"inputs"`
	Outputs      []string `json:"outputs"`
	Dependencies []int    `json:"dependencies"`
}

// Global storage for plans
var PlansMap = make(map[string]Plans, 0)

type AgentPlanner repository.Agent

type InitialContext struct {
	OS              string
	Arch            string
	User            string
	Shell           string
	CWD             string
	GoVersion       string
	FolderStructure map[string]any
	InstalledTools  []string
	EnvVars         map[string]string
	ProjectFiles    []string
	ProjectType     string
	GitBranch       string
	InternetAccess  bool
	AgentRole       string
	Capabilities    []string
	Timezone        string
	SessionID       string
}

func GetInitialContext() InitialContext {
	ctx := InitialContext{
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		User:            os.Getenv("USERNAME"),
		Shell:           detectDefaultShell(),
		CWD:             mustGetWorkingDir(),
		FolderStructure: GetProjectStructure(map[string]any{"path": "./"}),
		GoVersion:       runtime.Version(),
		InstalledTools:  detectTools(),
		EnvVars:         getImportantEnvVars(),
		ProjectFiles:    scanProjectFiles(),
		ProjectType:     detectProjectType(),
		GitBranch:       getGitBranch(),
		InternetAccess:  checkInternet(),
		Timezone:        getLocalTimezone(),
		SessionID:       generateSessionID(),
	}
	return ctx
}
func GetProjectStructure(args map[string]any) map[string]any {
	loadGitIgnore()
	path := args["path"].(string)
	var builder strings.Builder
	builder.WriteString(path + "\n")
	err := getProjectStructureRecursive(path, "", &builder)
	if err != nil {
		return map[string]any{"error": err, "output": nil}
	}

	if builder.String() == "." || builder.String() == "" {
		return map[string]any{"error": nil, "output": "<empty directory>"}
	}
	return map[string]any{"error": nil, "output": builder.String()}
}
func getProjectStructureRecursive(path string, prefix string, builder *strings.Builder) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for i, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())

		// skip ignored entries
		if ignoreMatcher != nil {
			relPath, _ := filepath.Rel(".", entryPath)
			if ignoreMatcher.MatchesPath(relPath) {
				continue
			}
		} else if defaultIgnore[entry.Name()] {
			continue
		}

		// draw branch
		connector := "├──"
		if i == len(entries)-1 {
			connector = "└──"
		}
		builder.WriteString(prefix + connector + " " + entry.Name() + "\n")

		// recursively descend
		if entry.IsDir() {
			subPrefix := prefix
			if i == len(entries)-1 {
				subPrefix += "    "
			} else {
				subPrefix += "│   "
			}
			err := getProjectStructureRecursive(entryPath, subPrefix, builder)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func loadGitIgnore() {
	if _, err := os.Stat(".gitignore"); err == nil {
		ignoreMatcher, _ = gitignore.CompileIgnoreFile(".gitignore")
	}
}
func detectDefaultShell() string {
	if runtime.GOOS == "windows" {
		// prefer PowerShell if present
		if _, err := exec.LookPath("powershell"); err == nil {
			return "powershell"
		}
		return "cmd"
	}
	return os.Getenv("SHELL")
}

func mustGetWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func detectTools() []string {
	var found []string
	val := os.Getenv("PATH")
	found = strings.Split(val, ";")
	return found
}

func getImportantEnvVars() map[string]string {
	keys := []string{"PATH", "GOROOT", "GOPATH", "JAVA_HOME"}
	env := make(map[string]string)
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		}
	}
	return env
}

func scanProjectFiles() []string {
	files := []string{}
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if strings.HasSuffix(path, ".go") ||
				path == "go.mod" || path == "package.json" || path == "requirements.txt" ||
				path == "Dockerfile" || path == "README.md" {
				files = append(files, path)
			}
		}
		return nil
	})
	return files
}

func detectProjectType() string {
	if _, err := os.Stat("go.mod"); err == nil {
		return "Go project"
	}
	if _, err := os.Stat("package.json"); err == nil {
		return "Node.js project"
	}
	if _, err := os.Stat("requirements.txt"); err == nil {
		return "Python project"
	}
	return "Unknown"
}

func getGitBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func checkInternet() bool {
	cmd := exec.Command("ping", "-c", "1", "8.8.8.8")
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "8.8.8.8")
	}
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func getLocalTimezone() string {
	_, tz := time.Now().Zone()
	return fmt.Sprintf("%d min offset", tz/60)
}

func generateSessionID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Helper function to format files for Orchestrator
func formatFilesForOrchestrator(filesRead map[string]any) string {
	if len(filesRead) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString("=== FILES CONTENT ===\n\n")

	for fileName, fileData := range filesRead {
		result.WriteString(fmt.Sprintf("FILE: %s\n", fileName))
		result.WriteString("=" + strings.Repeat("=", len(fileName)+6) + "\n")

		if chunks, ok := fileData.([]map[string]any); ok {
			for _, chunk := range chunks {
				if content, exists := chunk["content"].(string); exists {
					result.WriteString(content)
					result.WriteString("\n")
				}
			}
		}
		result.WriteString("\n" + strings.Repeat("-", 50) + "\n\n")
	}

	return result.String()
}

func (p *AgentPlanner) Interaction(args map[string]any) map[string]any {
	InitialContext := GetInitialContext()
	InitialContextBytes, err := json.Marshal(InitialContext)
	if err != nil {
		return map[string]any{"error": "Unable to get initial context"}
	}

	var userMessage strings.Builder

	// Add files content (from analysis)
	filesContent := formatFilesForOrchestrator(analysis.FilesRead)
	if filesContent != "" {
		userMessage.WriteString(filesContent)
		userMessage.WriteString("\n")
	}

	// Add initial context
	userMessage.WriteString("=== INITIAL CONTEXT ===\n")
	userMessage.WriteString(string(InitialContextBytes))
	userMessage.WriteString("\n\n")

	// Add query
	userMessage.WriteString("=== QUERY ===\n")
	if query, ok := args["query"].(string); ok {
		userMessage.WriteString(query)
	}
	log.Println("TEST: ORCHESTRATOR:  ", userMessage.String()[0:20])

	// Push consolidated user message into ChatHistory
	ChatHistory = append(ChatHistory, map[string]any{
		"role": "user",
		"parts": []map[string]any{
			{"text": userMessage.String()},
		},
	})

	for {
		// fmt.Println("Current ChatHistory:", ChatHistory)
		if ChatHistory[len(ChatHistory)-1]["role"] == "model" {
			ChatHistory = append(ChatHistory, map[string]any{
				"role": "user",
				"parts": map[string]any{
					"text": "If you feel there is not task left and nothing to do , call exit-process. Because only that can stop you and finish the program. Don't Respond with text , No text output should be there , call the exit-process. PERIOD",
				},
			})
		}
		output := p.RequestAgent(ChatHistory)

		if output["error"] != nil {
			fmt.Println("Error:", output["error"])
			os.Exit(1)
		}

		outputData, ok := output["output"].([]map[string]any)
		if !ok {
			fmt.Println("ERROR CONVERTING OUTPUT")
			return nil
		}

		if len(outputData) == 0 {
			fmt.Println("No output received")
			continue
		}

		// Process each output part
		for _, part := range outputData {
			partType, hasType := part["type"].(string)
			if !hasType {
				continue
			}

			if partType == "text" {
				// Handle text response
				if text, ok := part["data"].(string); ok {
					fmt.Println("Agent:", text)
				}
			} else if partType == "functionCall" {
				// Handle function call
				name, nameOK := part["name"].(string)
				argsData, argsOK := part["args"].(map[string]any)

				if !nameOK || !argsOK {
					fmt.Println("Error: invalid function call data")
					continue
				}

				fmt.Println("Calling function:", name)

				// Execute the function
				if function, exists := PlannertoolsFunc[name]; exists {
					result := function.Run(argsData)
					fmt.Println(result)
					if _, ok = result["exit"].(bool); ok {

						return map[string]any{}
					}
					// Add function response to chat history
					ChatHistory = append(ChatHistory, map[string]any{
						"role": "user",
						"parts": []map[string]any{
							{
								"functionResponse": map[string]any{
									"name":     name,
									"response": result,
								},
							},
						},
					})

					// Display result if it's a string
					if outputStr, ok := result["output"].(string); ok {
						fmt.Println("Result:", outputStr)
					}
				} else {
					fmt.Printf("Function %s not found\n", name)
				}
			}
		}

		// Continue the conversation loop
		fmt.Println("---")
	}
}

func (c *AgentPlanner) RequestAgent(contents []map[string]any) map[string]any {
	fmt.Printf("Processing request with Planner Agent: %s\n", c.Metadata.Name)

	// Build request payload
	request := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]any{
				{"text": c.Metadata.Instructions},
			},
		},
		"toolConfig": map[string]any{
			"functionCallingConfig": map[string]any{
				"mode": "ANY",
			},
		},
		"contents": contents,
		"tools": []map[string]any{
			{"functionDeclarations": GetPlannerArrayMap()},
		},
	}

	// Marshal request body
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	// Retry config
	const maxRetries = 5
	const maxWaitTime = 10 * time.Minute

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create HTTP request
		req, err := http.NewRequestWithContext(
			context.Background(),
			"POST",
			c.Metadata.Endpoints["http"],
			bytes.NewBuffer(jsonBody),
		)
		if err != nil {
			return map[string]any{"error": err.Error(), "output": nil}
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-goog-api-key", viper.GetString("PLANNER.API_KEY"))

		// Execute request with timeout
		client := &http.Client{Timeout: c.Metadata.Timeout}
		resp, err := client.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return map[string]any{"error": err.Error(), "output": nil}
			}
			time.Sleep(repository.ExponentialBackoff(attempt))
			continue
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			if attempt == maxRetries {
				return map[string]any{"error": err.Error(), "output": nil}
			}
			time.Sleep(repository.ExponentialBackoff(attempt))
			continue
		}

		// Handle success
		if resp.StatusCode == http.StatusOK {
			var response repository.Response
			if err = json.Unmarshal(bodyBytes, &response); err != nil {
				return map[string]any{"error": err.Error(), "output": nil}
			}

			output := []map[string]any{}
			for _, cand := range response.Candidates {
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						output = append(output, map[string]any{
							"type": "text",
							"data": part.Text,
						})
					}
					if part.FunctionCall != nil {
						output = append(output, map[string]any{
							"type": "functionCall",
							"name": part.FunctionCall.Name,
							"args": part.FunctionCall.Args,
						})
					}
				}
			}

			// Save to ChatHistory
			if len(response.Candidates) > 0 && len(response.Candidates[0].Content.Parts) > 0 {
				parts := []map[string]any{}
				for _, part := range response.Candidates[0].Content.Parts {
					if part.Text != "" {
						parts = append(parts, map[string]any{
							"text": part.Text,
						})
					}
					if part.FunctionCall != nil {
						parts = append(parts, map[string]any{
							"functionCall": map[string]any{
								"name": part.FunctionCall.Name,
								"args": part.FunctionCall.Args,
							},
						})
					}
				}
				if len(parts) > 0 {
					ChatHistory = append(ChatHistory, map[string]any{
						"role":  "planner",
						"parts": parts,
					})
				}
			}

			c.Metadata.LastActive = time.Now()
			return map[string]any{"error": nil, "output": output}
		}

		// Handle rate limits
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == maxRetries {
				return map[string]any{
					"error": fmt.Sprintf("Rate limit exceeded after %d retries. HTTP %d: %s",
						maxRetries, resp.StatusCode, string(bodyBytes)),
					"output": nil,
				}
			}
			retryDelay := repository.ParseRetryDelay(string(bodyBytes))
			waitTime := retryDelay
			if waitTime <= 0 {
				waitTime = repository.ExponentialBackoff(attempt)
			}
			if waitTime > maxWaitTime {
				waitTime = maxWaitTime
			}
			fmt.Printf("Rate limit hit (attempt %d/%d). Waiting %v before retry...\n",
				attempt+1, maxRetries+1, waitTime)
			time.Sleep(waitTime)
			continue
		}

		// Retry on server errors
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			if attempt == maxRetries {
				return map[string]any{
					"error": fmt.Sprintf("Server error after %d retries. HTTP %d: %s",
						maxRetries, resp.StatusCode, string(bodyBytes)),
					"output": nil,
				}
			}
			fmt.Printf("Server error (attempt %d/%d). Waiting %v before retry...\n",
				attempt+1, maxRetries+1, repository.ExponentialBackoff(attempt))
			time.Sleep(repository.ExponentialBackoff(attempt))
			continue
		}

		// Client errors (400–499) except 429 → don't retry
		return map[string]any{
			"error":  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)),
			"output": nil,
		}
	}

	return map[string]any{
		"error":  fmt.Sprintf("Max retries (%d) exceeded", maxRetries),
		"output": nil,
	}
}

func (p *AgentPlanner) NewAgent() {
	model := viper.GetString("PLANNER.MODEL")
	if model == "" {
		model = "gemini-1.5-pro"
	}

	instructions := repository.PlannerInstructions

	PlannerAgentMeta := repository.AgentMetadata{
		ID:             "Planner-agent-v1",
		Name:           "Planner Agent",
		Version:        "1.0.0",
		Type:           repository.AgentType(repository.AgentPlanner),
		Instructions:   instructions,
		LastActive:     time.Now(),
		MaxConcurrency: 5,
		Timeout:        30 * time.Second,
		Status:         "active",
		Tags:           []string{"Planner", "constraints", "inputs", "outputs", "validation"},
		Endpoints: map[string]string{
			"http": fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model),
		},
	}

	p.Metadata = PlannerAgentMeta
	p.Capabilities = PlannerCapabilities
	PlannertoolsFunc = make(map[string]repository.Function)
	for _, tool := range PlannerCapabilities {
		PlannertoolsFunc[tool.Name] = tool
	}
}

func GetPlannerArrayMap() []map[string]any {
	arrayOfMap := make([]map[string]any, 0)
	for _, v := range PlannerCapabilities {
		arrayOfMap = append(arrayOfMap, v.ToObject())
	}
	return arrayOfMap
}

// GenerateTasklist will take input from user and generate a tasklist and return to user
func GenerateTasklist(args map[string]any) map[string]any {
	name, ok := args["name"].(string)
	instructions, ok2 := args["instructions"].(string)

	if !ok || !ok2 {
		return map[string]any{"error": "insufficient parameters"}
	}
	return map[string]any{"output": "Tasklist name is " + name + " tasklist instructions are " + instructions}
}

// Key changes to make planner agent return output like coder agent:

// 1. Add an exit-process function to PlannerCapabilities

// Enhanced validation and error handling
func PutPlanForAgent(args map[string]any) map[string]any {
	log.Printf("DEBUG: Received args for PutPlanForAgent: %+v", args)

	// Check if args is empty or nil
	if args == nil || len(args) == 0 {
		return map[string]any{
			"success": false,
			"error":   "No parameters provided. I need you to call gather-plan-requirements first to collect the necessary information.",
			"action":  "call gather-plan-requirements function to collect plan details from user",
		}
	}

	// Create Plans struct directly from args (no nested "plan" object)
	var plan Plans

	// Extract title
	if title, ok := args["title"].(string); ok {
		plan.Title = title
	}

	// Extract objective
	if objective, ok := args["objective"].(string); ok {
		plan.Objective = objective
	}

	// Extract planId (optional)
	if planId, ok := args["planId"].(string); ok {
		plan.PlanID = planId
	}

	// Extract steps
	if stepsData, ok := args["steps"].([]any); ok {
		for i, stepData := range stepsData {
			if stepMap, ok := stepData.(map[string]any); ok {
				step := Step{
					ID: i + 1, // Auto-assign ID
				}

				if desc, ok := stepMap["description"].(string); ok {
					step.Description = desc
				}

				if agent, ok := stepMap["agent"].(string); ok {
					step.Agent = agent
				} else {
					step.Agent = "GeneralAgent" // Default agent
				}

				// Handle inputs array
				if inputs, ok := stepMap["inputs"].([]any); ok {
					for _, input := range inputs {
						if inputStr, ok := input.(string); ok {
							step.Inputs = append(step.Inputs, inputStr)
						}
					}
				}

				// Handle outputs array
				if outputs, ok := stepMap["outputs"].([]any); ok {
					for _, output := range outputs {
						if outputStr, ok := output.(string); ok {
							step.Outputs = append(step.Outputs, outputStr)
						}
					}
				}

				// Handle dependencies array
				if deps, ok := stepMap["dependencies"].([]any); ok {
					for _, dep := range deps {
						if depInt, ok := dep.(float64); ok {
							step.Dependencies = append(step.Dependencies, int(depInt))
						}
					}
				}

				plan.Steps = append(plan.Steps, step)
			}
		}
	}

	// Extract assumptions (optional)
	if assumptions, ok := args["assumptions"].([]any); ok {
		for _, assumption := range assumptions {
			if assumptionStr, ok := assumption.(string); ok {
				plan.Assumptions = append(plan.Assumptions, assumptionStr)
			}
		}
	}

	// Extract successCriteria (optional)
	if criteria, ok := args["successCriteria"].([]any); ok {
		for _, criterion := range criteria {
			if criterionStr, ok := criterion.(string); ok {
				plan.SuccessCriteria = append(plan.SuccessCriteria, criterionStr)
			}
		}
	}

	// Extract fallbacks (optional)
	if fallbacks, ok := args["fallbacks"].([]any); ok {
		for _, fallback := range fallbacks {
			if fallbackStr, ok := fallback.(string); ok {
				plan.Fallbacks = append(plan.Fallbacks, fallbackStr)
			}
		}
	}

	// Generate unique plan ID if not provided
	if plan.PlanID == "" {
		plan.PlanID = fmt.Sprintf("plan_%d", time.Now().Unix())
	}

	// Enhanced validation with specific guidance
	validationErrors := []string{}

	if plan.Title == "" {
		validationErrors = append(validationErrors, "Missing required field: 'title'")
	}

	if plan.Objective == "" {
		validationErrors = append(validationErrors, "Missing required field: 'objective'")
	}

	if len(plan.Steps) == 0 {
		validationErrors = append(validationErrors, "Missing required field: 'steps' (must have at least one step)")
	}

	if len(validationErrors) > 0 {
		return map[string]any{
			"success": false,
			"error":   "Plan validation failed: " + strings.Join(validationErrors, ", "),
			"requirements": []string{
				"title: A descriptive name for your plan",
				"objective: A clear statement of what this plan achieves",
				"steps: An array of at least one step with 'description' field",
			},
			"example": `{
  "plan": {
    "title": "Setup Development Environment",
    "objective": "Configure a complete development environment for the project",
    "steps": [
      {
        "description": "Install required dependencies",
        "agent": "SystemAgent"
      },
      {
        "description": "Configure database connection", 
        "agent": "DatabaseAgent"
      }
    ]
  }
}`,
		}
	}

	// Validate and fix steps
	for i, step := range plan.Steps {
		if step.Description == "" {
			return map[string]any{
				"success": false,
				"error":   fmt.Sprintf("Step %d is missing required 'description' field", i+1),
				"hint":    "Each step must have a clear description of what should be done",
			}
		}

		// Auto-assign agent if missing
		if step.Agent == "" {
			plan.Steps[i].Agent = "GeneralAgent"
		}

		// Auto-assign ID if missing
		if step.ID == 0 {
			plan.Steps[i].ID = i + 1
		}
	}

	// Store the plan
	PlansMap[plan.PlanID] = plan

	// Return successful response
	return map[string]any{
		"success": true,
		"planId":  plan.PlanID,
		"message": fmt.Sprintf("Plan '%s' created successfully with %d steps", plan.Title, len(plan.Steps)),
		"plan":    plan,
	}
}
func getKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func ExitProcess(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok && text != "" {
		fmt.Println(text)
	}
	marshabData, err := json.Marshal(PlansMap)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	return map[string]any{"error": nil, "output": string(marshabData), "exit": true}
}

func TakeInputFromTerminal(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if !ok {
		return map[string]any{"error": "No Text Provided"}
	}
	fmt.Println(text)

	requirements, ok := args["requirements"].([]any)
	reader := bufio.NewReader(os.Stdin)

	// Case 1: No requirements -> just take a single input
	if !ok || len(requirements) == 0 {
		fmt.Print("dost> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return map[string]any{
				"error":  fmt.Sprintf("Error reading input: %v", err),
				"output": nil,
			}
		}
		input = strings.TrimSpace(input)
		if input == "" {
			return map[string]any{"error": nil, "output": "<no input provided>"}
		}
		return map[string]any{"error": nil, "output": input}
	}

	// Case 2: Requirements exist -> ask each question
	results := make(map[string]string)
	for _, req := range requirements {
		question, ok := req.(string)
		if !ok {
			continue
		}

		fmt.Printf("dost> %s: ", question)
		input, err := reader.ReadString('\n')
		if err != nil {
			return map[string]any{
				"error":  fmt.Sprintf("Error reading input: %v", err),
				"output": nil,
			}
		}

		input = strings.TrimSpace(input)
		if input == "" {
			results[question] = "<no input provided>"
		} else {
			results[question] = input
		}
	}

	return map[string]any{"error": nil, "output": results}
}

// 3. Add exit-process to PlannerCapabilities array
var PlannerCapabilities = []repository.Function{
	{
		Name: "exit-process",
		Description: `Gracefully terminates the planning session with comprehensive completion validation and user satisfaction confirmation.
Performs final quality checks, validates all requirements fulfillment, and ensures clean planning state before exit.
Triggers automatic documentation generation, plan summaries, and implementation readiness assessment.

Critical Exit Criteria:
✓ All specified planning tasks completed with verified structure
✓ Plan quality standards met (clarity, feasibility, completeness)
✓ No unresolved planning issues or missing requirements
✓ User acceptance and satisfaction confirmed
✓ Planning documentation updated and accurate`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Professional completion summary and final recommendations for the user. Include planning status, deliverables completed, and next steps.",
				},
			},
			Required: []string{},
		},
		Service: ExitProcess,
		Return: repository.Return{
			"error":  "string // System error if graceful exit fails",
			"output": "string // Final completion report and recommendations",
		},
	},

	{
		Name: "create-plan",
		Description: `Creates a structured execution plan. Provide plan fields directly (not nested under 'plan' key).

REQUIRED FIELDS:
- title: Descriptive name for the plan  
- objective: Clear statement of what the plan achieves
- steps: Array of at least one step, each with:
  - description: What this step does (REQUIRED)
  - agent: Which agent executes this step (optional, defaults to "GeneralAgent")

OPTIONAL FIELDS:
- planId: Unique identifier (auto-generated if missing)
- assumptions: List of prerequisites  
- successCriteria: How to measure success
- fallbacks: What to do if steps fail

EXAMPLE USAGE:
{
  "title": "User Registration System",
  "objective": "Implement secure user signup process", 
  "steps": [
    {
      "description": "Create user database schema",
      "agent": "DatabaseAgent"
    },
    {
      "description": "Build registration API endpoint", 
      "agent": "BackendAgent"
    }
  ]
}`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"planId": {
					Type:        repository.TypeString,
					Description: "Unique plan identifier (auto-generated if not provided)",
				},
				"title": {
					Type:        repository.TypeString,
					Description: "REQUIRED: Descriptive title for the plan",
				},
				"objective": {
					Type:        repository.TypeString,
					Description: "REQUIRED: Clear objective statement",
				},
				"steps": {
					Type:        repository.TypeArray,
					Description: "REQUIRED: At least one execution step",
					Items: &repository.Properties{
						Type: repository.TypeObject,
						Properties: map[string]*repository.Properties{
							"id":           {Type: repository.TypeInteger, Description: "Step number (auto-assigned if missing)"},
							"description":  {Type: repository.TypeString, Description: "REQUIRED: What this step does"},
							"agent":        {Type: repository.TypeString, Description: "Which agent executes (defaults to GeneralAgent)"},
							"inputs":       {Type: repository.TypeArray, Items: &repository.Properties{Type: repository.TypeString}},
							"outputs":      {Type: repository.TypeArray, Items: &repository.Properties{Type: repository.TypeString}},
							"dependencies": {Type: repository.TypeArray, Items: &repository.Properties{Type: repository.TypeInteger}},
						},
						Required: []string{"description"},
					},
				},
				"assumptions":     {Type: repository.TypeArray, Items: &repository.Properties{Type: repository.TypeString}},
				"successCriteria": {Type: repository.TypeArray, Items: &repository.Properties{Type: repository.TypeString}},
				"fallbacks":       {Type: repository.TypeArray, Items: &repository.Properties{Type: repository.TypeString}},
			},
			Required: []string{"title", "objective", "steps"},
		},
		Service: PutPlanForAgent,
	},
	{
		Name:        "take-input-from-terminal",
		Description: `Collects input from the user via terminal interaction.`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Prompt text to display to user",
				},
				"requirements": {
					Type:        repository.TypeArray,
					Description: "List of specific questions to ask (optional)",
					Items:       &repository.Properties{Type: repository.TypeString},
				},
			},
			Required: []string{"text", "requirements"},
		},
		Service: TakeInputFromTerminal,
	},
}

// 4. Update the init function to include all planner tools
func init() {
	for _, v := range PlannerCapabilities {
		PlannertoolsFunc[v.Name] = v
	}
}
