package orchestrator

import (
	"bufio"
	"bytes"
	"context"
	"dost/internal/repository"
	"dost/internal/service/analysis"
	"dost/internal/service/coder"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type TaskStatus string

var OrchestratortoolsFunc map[string]repository.Function = make(map[string]repository.Function)

var TaskMap = make(map[string]Task)
var ChatHistory = make([]map[string]any, 0)

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

type Task struct {
	ID           string            `json:"id"`
	Description  string            `json:"description"`
	AgentID      string            `json:"agent_id"`
	Inputs       map[string]any    `json:"inputs"`
	Outputs      map[string]any    `json:"outputs"`
	Status       TaskStatus        `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Dependencies []string          `json:"dependencies"`
	Metadata     map[string]string `json:"metadata"`
}

type AgentOrchestrator repository.Agent

func (p *AgentOrchestrator) Interaction(args map[string]any) map[string]any {
	ChatHistory = append(ChatHistory, map[string]any{
		"role": "user",
		"parts": []map[string]any{
			{
				"text": args["query"],
			},
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
				if function, exists := OrchestratortoolsFunc[name]; exists {
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
func (p *AgentOrchestrator) NewAgent() {
	model := viper.GetString("ORCHESTRATOR.MODEL")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	endPoints := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)

	analysisAgentMeta := repository.AgentMetadata{
		ID:             "analysis-agent-v1",
		Name:           "Orchestrator Agent",
		Version:        "1.0.0",
		Type:           repository.AgentType(repository.AgentOrchestrator),
		Instructions:   repository.OrchestratorInstructions,
		LastActive:     time.Now(),
		MaxConcurrency: 5,
		Timeout:        30 * time.Second,
		Status:         "active",
		Tags:           []string{"analysis", "constraints", "inputs", "outputs", "validation"},
		Endpoints: map[string]string{
			"http": endPoints,
		},
	}
	p.Metadata = analysisAgentMeta
	p.Capabilities = OrchestratorCapabilities
}
func AskAnAgent(args map[string]any) map[string]any {

	text, ok := args["text"].(string)
	if ok {
		fmt.Println(text)
	}

	agentID, ok := args["agent_id"].(string)
	if !ok || agentID == "" {
		return map[string]any{
			"error":  "missing or invalid agent_id",
			"output": nil,
		}
	}

	task, ok := args["task"].(string)
	if !ok {
		return map[string]any{
			"error":  "missing or invalid task",
			"output": nil,
		}
	}

	var result map[string]any

	switch agentID {
	case "analysis":

		anaAgent := analysis.AgentAnalysis{}
		anaAgent.NewAgent()
		query := map[string]any{
			"query": fmt.Sprintf("Here is the Task run an analysis for this task: \" %s\", Don't assume anything ask the user for any clearence you want. Don't hallucinate!", task),
		}
		fmt.Println(query["query"])
		analysisOutPut := anaAgent.Interaction(query)["analysis-id"]
		analysisResult := analysis.AnalysisMap[analysisOutPut.(string)]
		jsonBytes, err := json.MarshalIndent(analysisResult, "", "  ")
		if err != nil {
			fmt.Println("❌ Error marshalling JSON:", err)
			return nil
		}

		result = map[string]any{"analysis": string(jsonBytes)}
	case "planning":
		// result = PlanningAgent(task)
	case "coder":
		analysis_id, ok := args["analysis-id"].(string)
		if !ok {
			return map[string]any{
				"error":  "missing or analysis",
				"output": nil,
			}
		}
		prevAnalysis, ok := analysis.AnalysisMap[analysis_id]
		if !ok {
			fmt.Println(prevAnalysis)
			return map[string]any{
				"error": "analysis not found",
			}
		}
		analysisForCoder, err := json.Marshal(prevAnalysis)
		if err != nil {
			return map[string]any{
				"error": "Error in marshaling",
			}
		}
		anaAgent := coder.AgentCoder{}
		anaAgent.NewAgent()
		query := map[string]any{
			"query": fmt.Sprintf("Analysis for this task generated by Analysis Agent :\n %s \n Here is the Task run an analysis for this task: \n\" %s\"\n,. Don't hallucinate!", analysisForCoder, task),
		}
		fmt.Println(query["query"])
		coderid := anaAgent.Interaction(query)["coder-id"].(string)
		result = map[string]any{"coder": coderid}

	default:
		return map[string]any{
			"error":  fmt.Sprintf("unknown agent: %s", agentID),
			"output": nil,
		}
	}
	fmt.Println(result)

	return map[string]any{
		"error":  nil,
		"output": result,
	}
}

func CreateTasks(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok {
		fmt.Println(text)
	}
	rawTasks, ok := args["tasks"].([]interface{})
	if !ok {
		return map[string]any{"error": "missing or invalid 'tasks' (must be array)", "output": nil}
	}

	var createdTasks []Task

	for _, t := range rawTasks {
		taskArgs, ok := t.(map[string]any)
		if !ok {
			return map[string]any{"error": "each task must be an object", "output": nil}
		}

		id, ok := taskArgs["id"].(string)
		if !ok || id == "" {
			return map[string]any{"error": "each task must have a valid 'id'", "output": nil}
		}

		description, _ := taskArgs["description"].(string)
		agentID, _ := taskArgs["agent_id"].(string)

		task := Task{
			ID:          id,
			Description: description,
			AgentID:     agentID,
			Inputs:      make(map[string]any),
			Outputs:     make(map[string]any),
			Status:      TaskPending,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// Optional fields
		if inputs, ok := taskArgs["inputs"].(map[string]any); ok {
			task.Inputs = inputs
		}
		if deps, ok := taskArgs["dependencies"].([]string); ok {
			task.Dependencies = deps
		}
		if meta, ok := taskArgs["metadata"].(map[string]string); ok {
			task.Metadata = meta
		}

		// Save in TaskMap
		TaskMap[task.ID] = task
		createdTasks = append(createdTasks, task)

		fmt.Printf("✅ Created Task: %s (%s)\n", task.ID, task.Description)
	}

	return map[string]any{
		"error":  nil,
		"output": createdTasks,
	}
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

func ExitProcess(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok && text != "" {
		fmt.Println(text)
	}

	fmt.Println("--- Task completed successfully! Exiting...")
	return map[string]any{"error": nil, "output": "Process exited successfully", "exit": true}

}

func ReadFiles(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok {
		fmt.Printf("ORCH: %s", text)
	}

	fileNames, ok := args["file_names"].([]interface{})
	if !ok {
		return map[string]any{
			"error":  "Invalid arguments: 'file_names' must be a slice of strings",
			"output": nil,
		}
	}

	var stringFileNames []string
	for _, v := range fileNames {
		s, ok := v.(string)
		if !ok {
			return map[string]any{
				"error":  "Invalid argument: 'file_names' contains non-string values",
				"output": nil,
			}
		}
		stringFileNames = append(stringFileNames, s)
	}

	readFiles := make(map[string]any)
	var notFoundFiles []string

	for _, fileName := range stringFileNames {
		file, err := os.Open(fileName)
		if err != nil {
			notFoundFiles = append(notFoundFiles, fileName)
			continue
		}
		defer file.Close()

		// Read all lines first
		scanner := bufio.NewScanner(file)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		// Don't chunk unless necessary - send complete small files

		if err := scanner.Err(); err != nil {
			readFiles[fileName] = fmt.Sprintf("Error reading file: %v", err)
			continue
		}

		// Create proper chunks (non-overlapping)
		chunks := []map[string]any{}
		chunkSize := 40
		if len(lines) < 100 {
			chunks = []map[string]any{{
				"start":   1,
				"end":     len(lines),
				"content": strings.Join(lines, "\n"),
			}}
		} else {
			for i := 0; i < len(lines); i += chunkSize {
				end := i + chunkSize
				if end > len(lines) {
					end = len(lines)
				}

				// Build chunk content
				var chunkContent strings.Builder
				for j := i; j < end; j++ {
					chunkContent.WriteString(lines[j])
					chunkContent.WriteString("\n")
				}

				chunks = append(chunks, map[string]any{
					"start":   i + 1,
					"end":     end,
					"content": chunkContent.String(),
				})
			}

			// Handle empty file edge case
			if len(lines) == 0 {
				chunks = append(chunks, map[string]any{
					"start":   1,
					"end":     0,
					"content": "",
				})
			}
		}

		readFiles[fileName] = chunks
		fmt.Printf("Read file: %s (%d chunks)\n", fileName, len(chunks))
	}

	if len(notFoundFiles) > 0 {
		fmt.Printf("Files not found: %v\n", notFoundFiles)
	}

	return map[string]any{"error": nil, "output": readFiles}
}

func (p *AgentOrchestrator) RequestAgent(contents []map[string]any) map[string]any {
	fmt.Printf("Processing request with Orchestrator Agent: %s\n", p.Metadata.Name)

	// Build request payload
	request := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]any{
				{"text": p.Metadata.Instructions},
			},
		},
		"toolConfig": map[string]any{
			"functionCallingConfig": map[string]any{
				"mode": "ANY",
			},
		},
		"contents": contents,
		"tools": []map[string]any{
			{"functionDeclarations": GetOrchestratorToolsMap()},
		},
	}

	// Marshal request
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	// Retry configuration
	const maxRetries = 5
	const maxWaitTime = 10 * time.Minute

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create HTTP request
		req, err := http.NewRequestWithContext(
			context.Background(),
			"POST",
			p.Metadata.Endpoints["http"],
			bytes.NewBuffer(jsonBody),
		)
		if err != nil {
			return map[string]any{"error": err.Error(), "output": nil}
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-goog-api-key", viper.GetString("ORCHESTRATOR.API_KEY"))

		// Execute request
		client := &http.Client{Timeout: p.Metadata.Timeout}
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

		// Success case
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

			// Save chat history
			if len(response.Candidates) > 0 && len(response.Candidates[0].Content.Parts) > 0 {
				parts := []map[string]any{}
				for _, part := range response.Candidates[0].Content.Parts {
					if part.Text != "" {
						parts = append(parts, map[string]any{"text": part.Text})
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

			p.Metadata.LastActive = time.Now()
			return map[string]any{"error": nil, "output": output}
		}

		// Handle 429 Too Many Requests
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
			fmt.Printf("Orchestrator rate limit hit (attempt %d/%d). Waiting %v...\n",
				attempt+1, maxRetries+1, waitTime)
			time.Sleep(waitTime)
			continue
		}

		// Retry 5xx server errors
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			if attempt == maxRetries {
				return map[string]any{
					"error": fmt.Sprintf("Server error after %d retries. HTTP %d: %s",
						maxRetries, resp.StatusCode, string(bodyBytes)),
					"output": nil,
				}
			}
			fmt.Printf("Orchestrator server error (attempt %d/%d). Retrying...\n",
				attempt+1, maxRetries+1)
			time.Sleep(repository.ExponentialBackoff(attempt))
			continue
		}

		// Fail fast on other 4xx
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

func ExecuteCommands(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok {
		fmt.Println(text)
	}

	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	// Extract command
	cmdStr, ok := args["command"].(string)
	if !ok || cmdStr == "" {
		return map[string]any{"error": "Invalid command"}
	}

	// Extract arguments (as array instead of a single string)
	var argList []string
	if rawArgs, ok := args["arguments"]; ok {
		switch v := rawArgs.(type) {
		case string:
			// split on spaces if user passed a string
			if v != "" {
				argList = strings.Fields(v)
			}
		case []any:
			for _, a := range v {
				if s, ok := a.(string); ok {
					argList = append(argList, s)
				}
			}
		}
	}

	// Handle `cd` separately
	if cmdStr == "cd" {
		if len(argList) == 0 {
			return map[string]any{"error": "cd requires a path"}
		}
		newDir := argList[0]
		if !filepath.IsAbs(newDir) {
			newDir = filepath.Join(wd, newDir)
		}
		if err := os.Chdir(newDir); err != nil {
			return map[string]any{"error": fmt.Sprintf("failed to change directory: %v", err)}
		}
		return map[string]any{"message": fmt.Sprintf("Changed directory to %s", newDir)}
	}

	// Build command properly
	cmd := exec.Command(cmdStr, argList...)
	cmd.Dir = wd
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	log.Default().Printf("Running command in %s: %s %v\n", wd, cmdStr, argList)

	err = cmd.Run()
	if err != nil {
		return map[string]any{
			"error": fmt.Sprintf("command failed: [%s %v] %v || CONSOLE/TERMINAL:%v",
				cmdStr, argList, err, stderrBuf.String()),
		}
	}

	return map[string]any{
		"message": "Command executed successfully",
		"output":  stdoutBuf.String(),
	}
}

var OrchestratorCapabilities = []repository.Function{{
	Name: "execute-command-in-terminal",
	Description: `Run any valid shell/terminal command in the current working directory. 
Use for file operations (create, delete, list, copy files, move files, rename files, git functions, etc), 
navigation (cd, ls), installing packages, running build tools, executing programs, checking versions, and inspecting system state. 
Always provide the command and arguments separately.
Example: { "command": "git", "arguments": ["commit", "-m", "testing through this tool"] }`,

	Parameters: repository.Parameters{
		Type: repository.TypeObject,
		Properties: map[string]*repository.Properties{
			"command": {
				Type:        repository.TypeString,
				Description: "Base command to execute (e.g., git, go, npm, python, ls)",
			},
			"arguments": {
				Type: repository.TypeArray,
				Items: &repository.Properties{
					Type:        repository.TypeString,
					Description: "Each argument passed separately (e.g., [\"commit\", \"-m\", \"msg\"])",
				},
				Description: "Optional arguments for the command as an array",
			},
		},
		Required: []string{"command"},
		Optional: []string{"arguments"},
	},

	Service: ExecuteCommands,

	Return: repository.Return{
		"error":  "string",
		"output": "string",
	},
},

	{
		Name: "take-input-from-terminal",
		Description: `Prompts the user in the terminal for multiple required inputs.
Use this whenever you are missing essential details, such as:
- File names
- Function parameters
- Configuration values
- User choices`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"requirements": {
					Type:        repository.TypeArray,
					Description: "A list of questions or keys to ask the user for input",
					Items: &repository.Properties{
						Type: "string",
					},
				},
				"text": {
					Type:        repository.TypeString,
					Description: "A text which you want to say to user, instead of returning text output give it in this parameter",
				},
			},
			Required: []string{"requirements", "text"},
		},
		Service: TakeInputFromTerminal,
		Return:  repository.Return{"error": "string", "output": "object"},
	},
	{
		Name: "exit-process",
		Description: `When you feel that the task is completed always call this to exit the process.
Before calling this function make sure,
You have completed all the task.
You have fixed all the bugs.
User is satisfied with the output.`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "A text which you want to say to user, instead of returning text output give it in this parameter",
				},
			},
			Required: []string{},
		},
		Service: ExitProcess,
		Return: repository.Return{
			"error":  "string",
			"output": "string",
		},
	},
	{
		Name: "ask-an-agent",
		Description: `Route a task to a specific agent for processing.
Available agents:
- analysis: Performs task analysis and validation
- planning: Creates execution plans
- execution: Executes planned tasks`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "A text which you want to say to user, instead of returning text output give it in this parameter",
				},
				"agent_id": {
					Type:        repository.TypeString,
					Description: "The ID of the agent to route the task to (analysis, planning, execution)",
					Enum:        []string{"analysis", "coder"},
				},
				"task": {
					Type:        repository.TypeString,
					Description: "The task which is to be processed by the agent, if nothing available just give the proper detailed query",
				},
				"analysis-id": {
					Type:        repository.TypeString,
					Description: "The Analysis which you got from analysis agent keep it simple and detailed with each and every analysis and bullet points. Since coder agent will also need it so make sure to give it.",
				},
			},
			Required: []string{"agent_id", "task", "analysis-id"},
		},
		Service: AskAnAgent,
		Return:  repository.Return{"error": "string", "output": "object"},
	},
	{
		Name: "create-tasks",
		Description: `Create multiple tasks and store them in the TaskMap.
Each task must have an ID and can have optional description, agent_id, inputs, dependencies, outputs, and metadata.`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "A text which you want to say to user, instead of returning text output give it in this parameter",
				},
				"tasks": {
					Type:        repository.TypeArray,
					Description: "Array of task objects to create",
					Items: &repository.Properties{
						Type: repository.TypeObject,
						Properties: map[string]*repository.Properties{
							"id": {
								Type:        repository.TypeString,
								Description: "Unique identifier for the task (required)",
							},
							"description": {
								Type:        repository.TypeString,
								Description: "Optional description of the task",
							},
							"agent_id": {
								Type:        repository.TypeString,
								Description: "Agent assigned to this task (optional)",
							},
							"inputs": {
								Type:        repository.TypeObject,
								Description: "Input parameters required for the task",
							},
							"outputs": {
								Type:        repository.TypeObject,
								Description: "Expected or generated outputs for the task",
							},
							"dependencies": {
								Type:        repository.TypeArray,
								Items:       &repository.Properties{Type: repository.TypeString},
								Description: "IDs of tasks this task depends on",
							},
							"metadata": {
								Type: repository.TypeObject,
								Properties: map[string]*repository.Properties{
									"key": {Type: repository.TypeString},
								},
								Description: "Custom metadata key-value pairs",
							},
						},
						Required: []string{"id", "description"},
					},
				},
			},
			Required: []string{"tasks"},
		},
		Service: CreateTasks,
		Return:  repository.Return{"error": "string", "output": "array"},
	},

	{
		Name: "read-files",
		Description: `Read the contents of one or more files from the filesystem.
Use this to analyze code, configuration files, or any text-based files.`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "A text which you want to say to user, instead of returning text output give it in this parameter",
				},
				"file_names": {
					Type:        repository.TypeArray,
					Description: "Array of file paths to read",
					Items: &repository.Properties{
						Type: "string",
					},
				},
			},
			Required: []string{"file_names"},
		},
		Service: ReadFiles,
		Return:  repository.Return{"error": "string", "output": "object"},
	},
}

func init() {
	for _, v := range OrchestratorCapabilities {
		OrchestratortoolsFunc[v.Name] = v
	}
}

func OrchestratorTools() map[string]repository.Function {
	return OrchestratortoolsFunc
}

func GetOrchestratorTools() []repository.Function {
	return OrchestratorCapabilities
}

func GetOrchestratorToolsMap() []map[string]any {
	arrayOfMap := make([]map[string]any, 0)
	for _, v := range OrchestratorCapabilities {
		arrayOfMap = append(arrayOfMap, v.ToObject())
	}
	return arrayOfMap
}
