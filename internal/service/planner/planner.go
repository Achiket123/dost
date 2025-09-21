package planner

import (
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

var ChatHistory = make([]map[string]any, 0)
var ignoreMatcher *gitignore.GitIgnore
var PlannertoolsFunc map[string]repository.Function = make(map[string]repository.Function)

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
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Actions     []string `json:"actions"`
}

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
						return nil
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
						"role":  response.Candidates[0].Content.Role,
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

func Plan(args map[string]any) map[string]any {

	rawNames, _ := args["names"].([]interface{})
	var names []string
	for _, v := range rawNames {
		if s, ok := v.(string); ok {
			names = append(names, s)
		}
	}

	// Convert descriptions
	rawDescriptions, _ := args["descriptions"].([]interface{})
	var descriptions []string
	for _, v := range rawDescriptions {
		if s, ok := v.(string); ok {
			descriptions = append(descriptions, s)
		}
	}

	// Convert actions ([][]string)
	rawActions, _ := args["actions"].([]interface{})
	var actions [][]string
	for _, rawGroup := range rawActions {
		if group, ok := rawGroup.([]interface{}); ok {
			var steps []string
			for _, v := range group {
				if s, ok := v.(string); ok {
					steps = append(steps, s)
				}
			}
			actions = append(actions, steps)
		}
	}

	// Build plans
	var plans []Plans
	for j := range names {
		var plan = Plans{
			Name:        names[j],
			Description: descriptions[j],
			Actions:     actions[j],
		}
		plans = append(plans, plan)
	}

	data, err := json.Marshal(plans)
	if err != nil {
		return map[string]any{
			"error": "Unable To Marshal",
		}
	}
	return map[string]any{"output": string(data)}
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
func ExitProcess(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok && text != "" {
		fmt.Println(text)
	}

	fmt.Println("--- Planning completed successfully! Exiting...")
	return map[string]any{"error": nil, "output": "Planning Completed Successfully", "exit": true}
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
		Description: `Creates a structured plan consisting of multiple tasks or goals. 
Each plan contains:
- A name (to identify the plan clearly).
- A description (to explain the overall goal or purpose).
- A list of ordered actions/steps that should be executed to achieve the plan.

This is useful for breaking down complex queries into structured, executable workflows.`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"names": {
					Type: repository.TypeArray,
					Description: `An array of plan names.
Each string should represent a concise, human-readable title for the plan (e.g., "Setup Git Repository", "Deploy Web Application").
The index of each name corresponds directly to the matching description and actions.`,
					Items: &repository.Properties{
						Type:        repository.TypeString,
						Description: "A single plan name/title.",
					},
				},
				"descriptions": {
					Type: repository.TypeArray,
					Description: `An array of descriptions, one for each plan.
Each description should clearly explain the overall purpose, context, or expected outcome of the plan (e.g., "Initialize a Git repository, commit initial files, and push to remote").`,
					Items: &repository.Properties{
						Type:        repository.TypeString,
						Description: "Detailed explanation for a single plan.",
					},
				},
				"actions": {
					Type: repository.TypeArray,
					Description: `An array of actions for each plan. 
Each element in this array corresponds to a specific plan and contains an ordered list of step-by-step instructions.
Example:
[
  ["Install dependencies", "Initialize Git repo", "Commit changes", "Push to remote"],
  ["Build Docker image", "Push image to registry", "Deploy to Kubernetes"]
]`,
					Items: &repository.Properties{
						Type: repository.TypeArray,
						Description: `List of ordered steps (strings) for one plan. 
Each step should be a clear, executable instruction.`,
						Items: &repository.Properties{
							Type:        repository.TypeString,
							Description: "A single step/instruction inside a plan.",
						},
					},
				},
			},
			Required: []string{"names", "descriptions", "actions"},
		},
		Service: Plan,
	},
}

// 4. Update the init function to include all planner tools
func init() {
	for _, v := range PlannerCapabilities {
		PlannertoolsFunc[v.Name] = v
	}
}
