package orchestrator

import (
	"bufio"
	"bytes"
	"context"
	"dost/internal/repository"
	"dost/internal/service"
	"dost/internal/service/analysis"

	"dost/internal/service/coder"
	"dost/internal/service/planner"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	gitignore "github.com/sabhiram/go-gitignore"

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

var ignoreMatcher *gitignore.GitIgnore

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

func (p *AgentOrchestrator) Interaction(args map[string]any) map[string]any {
	InitialContext := GetInitialContext()
	InitialContextBytes, err := json.Marshal(InitialContext)
	if err != nil {
		return map[string]any{"error": "Unable to get initial context"}
	}

	// Build consolidated user message
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
		// Check if last model response requires exit instruction
		if len(ChatHistory) > 0 && ChatHistory[len(ChatHistory)-1]["role"] == "model" {
			ChatHistory = append(ChatHistory, map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{
						"text": "If you feel there is no task left and nothing to do, call exit-process. Because only that can stop you and finish the program. Don't respond with text, no text output should be there, call the exit-process. PERIOD",
					},
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

			switch partType {
			case "text":
				if text, ok := part["data"].(string); ok {
					fmt.Println("Agent:", text)
				}

			case "functionCall":
				name, nameOK := part["name"].(string)
				argsData, argsOK := part["args"].(map[string]any)
				if !nameOK || !argsOK {
					fmt.Println("Error: invalid function call data")
					continue
				}

				fmt.Println("Calling function:", name)
				if function, exists := OrchestratortoolsFunc[name]; exists {
					result := function.Run(argsData)

					// Check for exit condition
					if _, ok := result["exit"].(bool); ok {
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

					if outputStr, ok := result["output"].(string); ok {
						fmt.Println("Result:", outputStr)
					}
				} else {
					fmt.Printf("Function %s not found\n", name)

					ChatHistory = append(ChatHistory, map[string]any{
						"role": "user",
						"parts": []map[string]any{
							{"text": fmt.Sprintf("Error: Function '%s' not found", name)},
						},
					})
				}
			}
		}

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
		ID:             "Orchestrator-agent-v1",
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
		analysisOutPutStr, ok := analysisOutPut.(string)
		if ok {
			analysisResult := analysis.AnalysisMap[analysisOutPutStr]
			jsonBytes, err := json.MarshalIndent(analysisResult, "", "  ")
			if err != nil {
				fmt.Println("X Error marshalling JSON:", err)
				return nil
			}
			result = map[string]any{"analysis": string(jsonBytes)}
		} else {
			result = map[string]any{"analysis": analysisOutPut}
		}

	case "planner":
		planAgent := planner.AgentPlanner{}
		planAgent.NewAgent()
		planAgent.Interaction(map[string]any{
			"query": fmt.Sprintf("Here is the Task do a detail planning for this task: \" %s\", Don't assume anything ask the user for any clearence you want. Don't hallucinate!", task),
		})
	case "coder":
		// Enhanced version with proper analysis checking and query enhancement
		analysis_id, hasAnalysisId := args["analysis-id"].(string)
		task, hasTask := args["task"].(string)

		if !hasTask {
			return map[string]any{
				"error":  "missing task parameter",
				"output": nil,
			}
		}

		var query map[string]any
		anaAgent := coder.AgentCoder{}
		anaAgent.NewAgent()

		// Check if analysis ID is provided and valid
		if hasAnalysisId && analysis_id != "" {
			prevAnalysis, analysisExists := analysis.AnalysisMap[analysis_id]

			if analysisExists {
				// Analysis found - include it in the query
				analysisForCoder, err := json.Marshal(prevAnalysis)
				if err != nil {
					return map[string]any{
						"error": "Error marshaling analysis data",
					}
				}

				// Enhanced query with analysis context
				query = map[string]any{
					"query": fmt.Sprintf(`CONTEXT: Previous Analysis Available
=====================================
%s

CURRENT TASK
=============
%s

INSTRUCTIONS
============
- Use the above analysis as context to inform your approach
- Build upon the insights from the previous analysis
- Ensure consistency with previous findings where applicable
- If the analysis contradicts the current task, prioritize the task requirements
- Provide detailed reasoning for your implementation decisions
- Do not hallucinate or make assumptions not supported by the analysis or task requirements`,
						string(analysisForCoder), task),
				}

				fmt.Printf("Using existing analysis (ID: %s) for enhanced query\n", analysis_id)
			} else {
				// Analysis ID provided but not found - log warning and proceed with basic query
				fmt.Printf("Warning: Analysis ID '%s' not found in AnalysisMap, proceeding with basic query\n", analysis_id)

				query = map[string]any{
					"query": fmt.Sprintf(`TASK
=====
%s

INSTRUCTIONS
============
- Analyze the task requirements thoroughly
- Provide a comprehensive implementation approach
- Consider edge cases and potential issues
- Ensure code quality and best practices
- Do not hallucinate or make unfounded assumptions`, task),
				}
			}
		} else {
			// No analysis ID provided - use enhanced basic query
			fmt.Println("No analysis ID provided, using enhanced basic query")

			query = map[string]any{
				"query": fmt.Sprintf(`TASK
=====
%s

INSTRUCTIONS
============
- Perform thorough analysis of the task requirements
- Break down complex requirements into manageable components
- Consider potential challenges and solutions
- Provide clear implementation strategy
- Follow coding best practices and standards
- Ensure comprehensive error handling
- Do not make assumptions beyond what's explicitly stated`, task),
			}
		}

		fmt.Printf("Generated Query:\n%s\n", query["query"])

		// Execute the agent interaction
		coderid := anaAgent.Interaction(query)["coder-id"].(string)
		result = map[string]any{
			"coder": coderid,
		}

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
		fmt.Printf("ORCHESTRATOR: %s", text)
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

		// Read all lines, skipping empty ones
		scanner := bufio.NewScanner(file)
		var lines []string
		lineNumber := 1
		for scanner.Scan() {
			line := scanner.Text()
			// Skip empty lines but track line numbers
			if strings.TrimSpace(line) != "" {
				// Add line number prefix to non-empty lines
				numberedLine := fmt.Sprintf("%d: %s", lineNumber, line)
				lines = append(lines, numberedLine)
			}
			lineNumber++
		}

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
					if j < end-1 { // Don't add newline after last line in chunk
						chunkContent.WriteString("\n")
					}
				}

				chunks = append(chunks, map[string]any{
					"start":   i + 1,
					"end":     end,
					"content": chunkContent.String(),
				})
			}

			// Handle empty file edge case (all lines were empty)
			if len(lines) == 0 {
				chunks = append(chunks, map[string]any{
					"start":   1,
					"end":     0,
					"content": "",
				})
			}
		}

		readFiles[fileName] = chunks
		// Also append to the global FilesRead map
		analysis.FilesRead[fileName] = chunks
		fmt.Printf("Read file: %s (%d non-empty lines, %d chunks)\n", fileName, len(lines), len(chunks))
	}

	if len(notFoundFiles) > 0 {
		fmt.Printf("Files not found: %v\n", notFoundFiles)
	}

	return map[string]any{"error": nil, "output": readFiles}
}

func WriteFile(args map[string]any) map[string]any {
	/*
		args : {
			file_names : [ "test.txt" ],
			contents : [ "hello world" ],
			offsets: [ 0 ]
		}
	*/
	fileNames, hasFileNames := args["file_names"]
	contents, hasContents := args["contents"]
	offsets, hasOffsets := args["offsets"]

	if !hasFileNames || !hasContents {
		return map[string]any{"error": "Invalid arguments: file_names and contents required", "output": nil}
	}

	// Type assertions
	fileNamesSlice, ok1 := fileNames.([]interface{})
	contentsSlice, ok2 := contents.([]interface{})
	offsetsSlice, _ := offsets.([]interface{})

	if !ok1 || !ok2 {
		return map[string]any{"error": "Invalid arguments: file_names and contents must be slices", "output": nil}
	}
	// Check if offsets are provided and if the length matches
	if hasOffsets && len(offsetsSlice) != len(fileNamesSlice) {
		return map[string]any{"error": "Invalid arguments: offsets must have the same length as file_names", "output": nil}
	}

	if len(fileNamesSlice) != len(contentsSlice) {
		return map[string]any{"error": "Invalid arguments: file_names and contents must have the same length", "output": nil}
	}

	var errors []error
	var modifiedFiles []string

	for i, v := range fileNamesSlice {
		fileName := v.(string)
		if isCodingFile(fileName) {
			return map[string]any{
				"error": fmt.Sprintf("OPERATION BLOCKED: '%s' appears to be a programming/coding file. This function is restricted to text-only files (.txt, .md, .rst, .log, .csv, .json, .xml, .yaml, .yml, .html, .css, .ini, .cfg, .conf, .rtf, .tex) to prevent accidental modification of source code.", fileName),
			}
		}

		// SECURITY CHECK: Verify this is an allowed text file type
		if !isAllowedTextFile(fileName) {
			return map[string]any{
				"error": fmt.Sprintf("OPERATION BLOCKED: '%s' file type is not allowed. Only text files (.txt, .md, .rst, .log, .csv, .json, .xml, .yaml, .yml, .html, .css, .ini, .cfg, .conf, .rtf, .tex) can be edited by this function.", fileName),
			}
		}
		content := contentsSlice[i].(string)

		// Determine the offset for the current file
		var offset int64
		if hasOffsets {
			offset = int64(offsetsSlice[i].(float64))
			if offset < 0 {
				errors = append(errors, fmt.Errorf("invalid offset for file %s: offset must be non-negative", fileName))
				continue
			}
		}

		// Check if file exists. If not, open it to create it.
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to open/create file %s: %v", fileName, err))
			continue
		}
		defer file.Close()

		// Seek to the specified offset before writing
		_, err = file.Seek(offset, os.SEEK_SET)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to seek to offset for file %s: %v", fileName, err))
			continue
		}

		// Write the content to the file
		_, err = file.WriteString(content)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to write to file %s: %v", fileName, err))
			continue
		}

		fmt.Printf("Modified file: %s at offset %d\n", fileName, offset)
		modifiedFiles = append(modifiedFiles, fileName)
	}

	if len(errors) > 0 {
		return map[string]any{
			"error":  fmt.Sprintf("Errors: %v", errors),
			"output": fmt.Sprintf("Partially completed. Modified files: %v", modifiedFiles),
		}
	}

	return map[string]any{
		"error":  nil,
		"output": fmt.Sprintf("Files modified successfully: %v", modifiedFiles),
	}
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
						"role":  "orchestrator",
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
	fmt.Println(">DOST\\")
	fmt.Printf("%s %v", cmdStr, argList)
	if !service.TakePermission {
		fmt.Printf("\nAbout to run command in %s:\n> %s\nPress ENTER to continue or Ctrl+C to cancel...", wd, argList)
		bufio.NewReader(os.Stdin).ReadBytes('\n') // wait for Enter
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
func EditFile(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok {
		fmt.Printf("ORCHESTRATOR: %s\n", text)
	}

	filepathInput, ok := args["file_path"].(string)
	if !ok {
		return map[string]any{"error": "ERROR READING PATH"}
	}

	// SECURITY CHECK: Verify this is not a coding file
	if isCodingFile(filepathInput) {
		return map[string]any{
			"error": fmt.Sprintf("OPERATION BLOCKED: '%s' appears to be a programming/coding file. This function is restricted to text-only files (.txt, .md, .rst, .log, .csv, .json, .xml, .yaml, .yml, .html, .css, .ini, .cfg, .conf, .rtf, .tex) to prevent accidental modification of source code.", filepathInput),
		}
	}

	// SECURITY CHECK: Verify this is an allowed text file type
	if !isAllowedTextFile(filepathInput) {
		return map[string]any{
			"error": fmt.Sprintf("OPERATION BLOCKED: '%s' file type is not allowed. Only text files (.txt, .md, .rst, .log, .csv, .json, .xml, .yaml, .yml, .html, .css, .ini, .cfg, .conf, .rtf, .tex) can be edited by this function.", filepathInput),
		}
	}

	changes, ok := args["changes"].([]interface{})
	if !ok {
		return map[string]any{"error": "REQUIRED CHANGES MAP"}
	}

	// Convert changes into structured format
	type changeInfo struct {
		startLine int
		startCol  int
		endLine   int
		endCol    int
		operation string
		content   string
	}

	toInt := func(v any) (int, bool) {
		switch val := v.(type) {
		case float64:
			return int(val), true
		case int:
			return val, true
		default:
			return 0, false
		}
	}

	var processedChanges []changeInfo
	for _, c := range changes {
		ch, ok := c.(map[string]any)
		if !ok {
			return map[string]any{"error": "INVALID CHANGE FORMAT"}
		}

		startLine, _ := toInt(ch["start_line_number"])
		startCol, _ := toInt(ch["start_line_col"])
		endLine, _ := toInt(ch["end_line_number"])
		endCol, _ := toInt(ch["end_line_col"])
		operation, _ := ch["operation"].(string)
		content, _ := ch["content"].(string)

		processedChanges = append(processedChanges, changeInfo{
			startLine: startLine,
			startCol:  startCol,
			endLine:   endLine,
			endCol:    endCol,
			operation: operation,
			content:   content,
		})
	}

	// Sort changes by start line & col (descending to avoid position conflicts)
	sort.Slice(processedChanges, func(i, j int) bool {
		if processedChanges[i].startLine == processedChanges[j].startLine {
			return processedChanges[i].startCol > processedChanges[j].startCol
		}
		return processedChanges[i].startLine > processedChanges[j].startLine
	})

	// Read entire file into memory first
	fileContent, err := os.ReadFile(filepathInput)
	if err != nil {
		return map[string]any{"error": "CANNOT READ INPUT FILE: " + err.Error()}
	}

	lines := strings.Split(string(fileContent), "\n")

	// Apply changes from last to first (to maintain line numbers)
	for _, change := range processedChanges {
		switch change.operation {
		case "delete":
			if change.startLine > 0 && change.endLine <= len(lines) {
				// Delete lines (1-indexed to 0-indexed)
				start := change.startLine - 1
				end := change.endLine
				if end > len(lines) {
					end = len(lines)
				}
				lines = append(lines[:start], lines[end:]...)
			}

		case "replace":
			if change.startLine > 0 && change.startLine <= len(lines) {
				lineIdx := change.startLine - 1
				if lineIdx < len(lines) {
					line := lines[lineIdx]
					if change.startCol <= len(line) && change.endCol <= len(line) {
						newLine := line[:change.startCol] + change.content + line[change.endCol:]
						lines[lineIdx] = newLine
					}
				}
			}

		case "write":
			if change.startLine > 0 && change.startLine <= len(lines) {
				lineIdx := change.startLine - 1
				if lineIdx < len(lines) {
					line := lines[lineIdx]
					if change.startCol <= len(line) {
						newLine := line[:change.startCol] + change.content + line[change.startCol:]
						lines[lineIdx] = newLine
					}
				}
			}
		}
	}

	// Write back to original file
	newContent := strings.Join(lines, "\n")
	err = os.WriteFile(filepathInput, []byte(newContent), 0644)
	if err != nil {
		return map[string]any{"error": "CANNOT WRITE TO FILE: " + err.Error()}
	}

	return map[string]any{
		"output": fmt.Sprintf("Successfully edited text file: %s", filepathInput),
	}
}
func GetProjectStructure(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok && text != "" {
		fmt.Println(text)
	}

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
func loadGitIgnore() {
	if _, err := os.Stat(".gitignore"); err == nil {
		ignoreMatcher, _ = gitignore.CompileIgnoreFile(".gitignore")
	}
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
		}
		if defaultIgnore[entry.Name()] {
			// ✅ Skip this directory and its contents completely
			if entry.IsDir() {
				continue
			}
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
			// 🚫 Don't go inside ignored directories
			if !defaultIgnore[entry.Name()] {
				err := getProjectStructureRecursive(entryPath, subPrefix, builder)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
func isAllowedTextFile(filePath string) bool {
	// Get file extension and convert to lowercase
	ext := strings.ToLower(filepath.Ext(filePath))

	// Define allowed text file extensions
	allowedExtensions := map[string]bool{
		".txt":  true, // Plain text files
		".md":   true, // Markdown files
		".rst":  true, // reStructuredText files
		".log":  true, // Log files
		".csv":  true, // Comma-separated values
		".json": true, // JSON data files
		".xml":  true, // XML files
		".yaml": true, // YAML files
		".yml":  true, // YAML files (alternate extension)
		".html": true, // HTML markup (content files, not code)
		".css":  true, // CSS stylesheets (content files, not code)
		".ini":  true, // Configuration files
		".cfg":  true, // Configuration files
		".conf": true, // Configuration files
		".rtf":  true, // Rich Text Format
		".tex":  true, // LaTeX files
		"":      true, // Files without extension (often text files)
	}

	return allowedExtensions[ext]
}

// isCodingFile checks if the file extension belongs to programming/coding files
func isCodingFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	codingExtensions := map[string]bool{

		".go":    true, // Go
		".py":    true, // Python
		".js":    true, // JavaScript
		".ts":    true, // TypeScript
		".jsx":   true, // React JSX
		".tsx":   true, // TypeScript JSX
		".java":  true, // Java
		".c":     true, // C
		".cpp":   true, // C++
		".cc":    true, // C++
		".cxx":   true, // C++
		".h":     true, // C/C++ headers
		".hpp":   true, // C++ headers
		".php":   true, // PHP
		".rb":    true, // Ruby
		".swift": true, // Swift
		".kt":    true, // Kotlin
		".rs":    true, // Rust
		".vue":   true, // Vue.js
		".scala": true, // Scala
		".r":     true, // R
		".m":     true, // Objective-C/MATLAB
		".pl":    true, // Perl
		".lua":   true, // Lua
		".dart":  true, // Dart
		".elm":   true, // Elm
		".clj":   true, // Clojure
		".hs":    true, // Haskell
		".f90":   true, // Fortran
		".pas":   true, // Pascal
		".asm":   true, // Assembly

		".sh":   true, // Shell scripts
		".bash": true, // Bash scripts
		".zsh":  true, // Zsh scripts
		".fish": true, // Fish scripts
		".bat":  true, // Windows batch files
		".cmd":  true, // Windows command files
		".ps1":  true, // PowerShell scripts

		".sql": true, // SQL files

		".makefile":   true, // Makefiles
		".dockerfile": true, // Docker files
		".gradle":     true, // Gradle build files
		".maven":      true, // Maven files
		".cmake":      true, // CMake files
	}

	return codingExtensions[ext]
}

var OrchestratorCapabilities = []repository.Function{{
	Name: "execute-command-in-terminal",
	Description: `Run any valid shell/terminal command in the current working directory. 
Use for:
- File operations (create, delete, list, copy files, move files, rename files)
- Navigation (cd, ls)
- Package management (npm, pip, go, dart, flutter, etc.)
- Build tools (make, go build, flutter build, etc.)
- Executing programs, checking versions, inspecting system state
- Git operations (status, diff, add, commit, push, pull, log, branch, etc.)

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
		Name:        "edit-file",
		Description: "Edits ONLY text-based files (txt, md, rst, log, csv, json, xml, yaml, yml, html, css, ini, cfg, conf, rtf, tex) at a given path by applying specified changes. This function is STRICTLY RESTRICTED to non-coding files to prevent accidental modification of source code. Programming files (.go, .py, .js, .java, .c, .cpp, .h, .php, .rb, .swift, .kt, .rs, .ts, .jsx, .tsx, .vue, .scala, .sh, .bat, .ps1, .sql, .r, .m, .pl, .lua, .dart, .elm, .clj, .hs, .f90, .pas, .asm, etc.) are explicitly blocked for safety.",
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Additional context or instructions provided by the AI for the edit operation.",
				},
				"file_path": {
					Type:        repository.TypeString,
					Description: "The path of the TEXT FILE to edit (only .txt, .md, .rst, .log, .csv, .json, .xml, .yaml, .yml, .html, .css, .ini, .cfg, .conf, .rtf, .tex files allowed). Example: './folder/document.txt' or './notes.md'. Programming files are blocked.",
				},
				"changes": {
					Type:        repository.TypeArray,
					Description: "A list of changes to apply to the text file, with each change specifying a selection and an operation.",
					Items: &repository.Properties{
						Type:        repository.TypeObject,
						Description: "Operation Meta data",
						Properties: map[string]*repository.Properties{
							"start_line_number": {
								Type:        repository.TypeInteger,
								Description: "The starting line number (1-based) of the change range.",
							},
							"start_line_col": {
								Type:        repository.TypeInteger,
								Description: "The starting column number (0-based) of the change range.",
							},
							"end_line_number": {
								Type:        repository.TypeInteger,
								Description: "The ending line number (1-based) of the change range.",
							},
							"end_line_col": {
								Type:        repository.TypeInteger,
								Description: "The ending column number (0-based) of the change range.",
							},
							"operation": {
								Enum:        []string{"delete", "replace", "write"},
								Type:        repository.TypeString,
								Description: "The operation to perform on the selected range. Options: 'delete' (remove text), 'replace' (replace text with new content), or 'write' (insert new content at start).",
							},
							"content": {
								Type:        repository.TypeString,
								Description: "The replacement or insertion text to apply. Required for 'replace' and 'write' operations.",
							},
						},
						Required: []string{"start_line_number", "start_line_col", "end_line_number", "end_line_col", "operation", "content"},
					},
				},
			},
			Required: []string{"text", "file_path", "changes"},
		},
		Service: EditFile,
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
		Description: `When you feel that the task is fully completed, always call this function to exit the process.
⚠️ Important:
- Before calling, make sure all steps are done, bugs are fixed, and the user is satisfied.
- Instead of just returning raw text, always compile everything (final output, explanations, summaries, code, results, etc.) into a single clear text message.
- Put that final compiled text inside the "text" parameter. This is what the user will see as the final answer.`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "The final compiled text output for the user (summary, results, explanations, etc.)",
				},
			},
			Required: []string{"text"},
		},
		Service: ExitProcess,
		Return: repository.Return{
			"error":  "string",
			"output": "string",
		},
	},
	{
		Name: "write-file",
		Description: `Write or overwrite file contents with precise control over offsets and data placement.  
This tool provides COMPLETE file manipulation capabilities, enabling:

📝 BASIC FILE OPERATIONS:
- Full overwrite: Replace the entire content of an existing file with new data.
- Append mode: Add content at the end of the file by setting offset equal to file size.
- Partial overwrite: Modify specific portions of a file without affecting the rest.
- Sparse file creation: Insert padding/zero-bytes automatically when offset exceeds file size.

📂 USE CASES:
- Create or update configuration files (e.g., .env, config.json, settings.yaml).
- Maintain logs, append new entries without altering existing data.
- Replace corrupted sections of a binary/text file.
- Write test fixtures or datasets programmatically.
- Manage source code updates while preserving line/byte positions.

⚙️ ADVANCED FEATURES:
- Multi-file support: Operate on multiple files in one call.
- Precise byte-level control: Offsets allow low-level file manipulations.
- Data replacement: Insert/overwrite any part of a file with new content slices.
- Sparse handling: Automatically generates empty byte padding for large jumps in offset.
- Integration: Always call 'read_files' first to load current file state before modification.

🚫 SAFETY & USAGE NOTES:
- ⚠️ Be careful when overwriting: it PERMANENTLY replaces existing file content.
- Always confirm offsets and file names to avoid unintentional data loss.
- Ensure file extensions match expected format (.txt, .json, .go, .py, etc.).
- Using offsets incorrectly can corrupt structured files (e.g., JSON, XML).
- Use sparse file creation only if downstream systems can handle them properly.

📊 COMMON PATTERNS:
- Full overwrite:
  { "file_names": ["config.json"], "contents": ["{ \"debug\": true }"], "offsets": [0] }

- Append to file:
  { "file_names": ["log.txt"], "contents": ["New log entry"], "offsets": [<current_file_size>] }

- Overwrite at position:
  { "file_names": ["data.bin"], "contents": ["FF00AA"], "offsets": [128] }

- Create sparse file:
  { "file_names": ["huge.dat"], "contents": ["End Marker"], "offsets": [1000000000] }

🖥️ EXAMPLES:
- Replace entire README.md with new content.
- Append a new migration entry into a SQL file.
- Patch a corrupted section of a binary executable.
- Programmatically generate structured text/data files.
- ONLY text-based files (txt, md, rst, log, csv, json, xml, yaml, yml, html, css, ini, cfg, conf, rtf, tex) at a given path by applying specified changes. This function is STRICTLY RESTRICTED to non-coding files to prevent accidental modification of source code. Programming files (.go, .py, .js, .java, .c, .cpp, .h, .php, .rb, .swift, .kt, .rs, .ts, .jsx, .tsx, .vue, .scala, .sh, .bat, .ps1, .sql, .r, .m, .pl, .lua, .dart, .elm, .clj, .hs, .f90, .pas, .asm, etc.) are explicitly blocked for safety.
This tool is your gateway to precise file manipulation and content management. Use it to create, append, overwrite, and manage files across any project lifecycle with full byte-level control.`,

		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"file_names": {
					Type:        repository.TypeArray,
					Items:       &repository.Properties{Type: repository.TypeString},
					Description: "List of file names (with extensions) where operations will be performed. Each file corresponds to matching index of 'contents' and 'offsets'.",
				},
				"contents": {
					Type:        repository.TypeArray,
					Items:       &repository.Properties{Type: repository.TypeString},
					Description: "Full content strings to write into files. Each entry aligns with 'file_names' and 'offsets'.",
				},
				"offsets": {
					Type:  repository.TypeArray,
					Items: &repository.Properties{Type: repository.TypeNumber},
					Description: `Array of byte offsets controlling write position for each file:
- Omit or set 0 → Overwrite entire file from beginning.
- Equal to file size → Append new content at the end.
- Within file size → Overwrite content starting from that offset.
- Beyond file size → Create sparse file with zero-padding.`,
				},
			},
			Required: []string{"file_names", "contents", "offsets"},
		},

		Service: WriteFile,

		Return: repository.Return{
			"error":  "string // Error details if write fails (invalid path, permissions, etc.).",
			"output": "string // Confirmation details such as bytes written, offsets applied, and file names affected.",
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
					Enum:        []string{"analysis", "coder", "planner"},
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
		Description: `Read and analyze multiple files efficiently in a single operation. 
	
	IMPORTANT: This function can read MULTIPLE files simultaneously - pass ALL file paths you need in the file_names array rather than calling this function multiple times for individual files.
	
	Key features:
	- Reads multiple files in one call (more efficient than multiple separate calls)
	- Automatically skips empty lines to save tokens and improve clarity
	- Adds line numbers to each line for precise reference and debugging
	- Chunks large files automatically for better processing
	- Stores read files globally for reuse across the session
	
	Best practices:
	- Pass ALL required file paths in a single call: ["file1.go", "file2.md", "file3.txt"]
	- Use when you need to analyze code structure, configuration files, documentation, or any text-based content
	- Ideal for cross-file analysis, dependency checking, or comprehensive codebase review
	
	Example usage scenarios:
	- Code analysis: ["main.go", "utils.go", "config.yaml"]
	- Documentation review: ["README.md", "CHANGELOG.md", "API.md"]
	- Configuration audit: ["docker-compose.yml", ".env", "nginx.conf"]
	
	The function returns structured data with line numbers, making it easy to reference specific parts of files in subsequent analysis.`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Optional message to display to the user explaining what files you're reading and why, instead of just returning output silently",
				},
				"file_names": {
					Type:        repository.TypeArray,
					Description: "Array of file paths to read simultaneously. IMPORTANT: Include ALL files you need in this single array rather than making multiple function calls. Examples: ['main.go', 'config.yaml'], ['src/app.js', 'package.json', 'README.md']",
					Items: &repository.Properties{
						Type: "string",
					},
				},
			},
			Required: []string{"file_names"},
		},
		Service: ReadFiles,
		Return: repository.Return{
			"error":  "string - null if successful, error message if failed",
			"output": "object - map of filename to chunks with line numbers and content",
		},
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
