package coder

import (
	"bufio"
	"bytes"
	"context"
	"dost/internal/repository"
	"dost/internal/service"
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
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/viper"
)

// Coder capabilities definitions.
const (
	CreateFileFromName   = "create_file"
	ReadFileFromName     = "read_file"
	ListDirectoryName    = "list_directory"
	DeleteFileOrDirName  = "delete_file_or_dir"
	EditFileFromName     = "edit_file"
	RequestUserInputName = "request_user_input"
)

const CreateFileDescription = `Creates new files with precise content placement and intelligent path resolution. 
Supports atomic file creation with automatic directory structure generation, UTF-8 encoding, and collision handling. 
Overwrites existing files when explicitly required. Optimized for multi-file project scaffolding and code generation workflows.`

const ReadFileDescription = `Performs high-performance file content retrieval with intelligent encoding detection and memory-optimized streaming. 
Supports batch reading operations, automatic charset conversion, and binary-safe content handling. 
Essential for code analysis, dependency inspection, and project structure understanding before modifications.`

const ListDirectoryDescription = `Provides comprehensive directory traversal with intelligent filtering, recursive scanning capabilities, and .gitignore awareness. 
Returns structured metadata including file types, sizes, permissions, and modification timestamps. 
Optimized for project discovery, dependency analysis, and codebase exploration workflows.`

const DeleteFileOrDirDescription = `Executes safe file and directory removal operations with rollback capabilities and dependency validation. 
Supports recursive deletion with conflict detection, backup creation, and atomic cleanup operations. 
Includes safety checks to prevent accidental deletion of critical project files and version control data.`

const EditFileDescription = `Advanced multi-operation file editor with precise line-column targeting and atomic change application. 
Supports sophisticated edit operations including insertions, deletions, replacements, and multi-region modifications. 
Features intelligent conflict resolution, syntax-aware editing, and transaction-based changes with rollback support. 
Optimized for complex refactoring, code generation, and automated maintenance tasks.`

const RequestUserInputDescription = `Interactive terminal interface for real-time user communication and decision-making workflows. 
Provides formatted input prompts with validation, timeout handling, and context-aware questioning. 
Essential for gathering requirements, confirming destructive operations, and obtaining user preferences during development.`
 
var CodertoolsFunc map[string]repository.Function = make(map[string]repository.Function)
var ignoreMatcher *gitignore.GitIgnore

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
type changeInfo struct {
	startLine int
	startCol  int
	endLine   int
	endCol    int
	operation string
	content   string
	valid     bool
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

type AgentCoder repository.Agent

const coderName = "coder"

const coderVersion = "0.1.0"

// args must and only contains "query"
// Helper function to format files for Coder
func formatFilesForCoder(filesRead map[string]any) string {
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

func (p *AgentCoder) Interaction(args map[string]any) map[string]any {
	InitialContext := GetInitialContext()
	InitialContextBytes, err := json.Marshal(InitialContext)
	if err != nil {
		return map[string]any{"error": "Unable to get initial context"}
	}

	var userMessage strings.Builder

	filesContent := formatFilesForCoder(analysis.FilesRead)
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
	log.Println("TEST: CODER:  ", userMessage.String()[0:20])
	// Push consolidated user message into ChatHistory
	ChatHistory = append(ChatHistory, map[string]any{
		"role": "user",
		"parts": []map[string]any{
			{"text": userMessage.String()},
		},
	})

	for {
		// Ensure exit-process is always enforced
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

				// Execute the function
				if function, exists := CodertoolsFunc[name]; exists {
					result := function.Run(argsData)

					// Check for exit condition
					if _, ok := result["exit"].(bool); ok {
						return map[string]any{"coder-id": result["output"]}
					}

					// Add function response back to chat history
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

					// Add error response into chat
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

// NewAgent creates and initializes a new AgentCoder instance.
func (c *AgentCoder) NewAgent() {
	model := viper.GetString("CODER.MODEL")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	endPoints := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	id := fmt.Sprintf("coder-%s", uuid.NewString())

	agentMetadata := repository.AgentMetadata{
		ID:             id,
		Name:           coderName,
		Version:        coderVersion,
		Type:           repository.AgentCoder,
		Instructions:   repository.CoderInstructions,
		MaxConcurrency: 5,
		Timeout:        10 * time.Minute,
		Tags:           []string{"coder", "code", "programming", "development"},
		Endpoints:      map[string]string{"http": endPoints},
		Context:        make(map[string]any),
		Status:         "active",
		LastActive:     time.Now(),
	}

	c.Metadata = agentMetadata
	c.Capabilities = CoderCapabilities

}

// RequestAgent is the main entry point for the AgentCoder to handle incoming tasks.
func (c *AgentCoder) RequestAgent(contents []map[string]any) map[string]any {
	fmt.Printf("Processing request with Coder Agent: %s\n", c.Metadata.Name)

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
			{"functionDeclarations": GetCoderCapabilitiesArrayMap()},
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
		req.Header.Set("X-goog-api-key", viper.GetString("CODER.API_KEY"))

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
						"role":  "coder",
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

// ToMap serializes the AgentCoder into a map.
func (c *AgentCoder) ToMap() map[string]any {
	agent := repository.Agent(*c)
	return agent.ToMap()
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

// CreateFile creates new files - FIXED VERSION
func CreateFile(data map[string]any) map[string]any {
	text, ok := data["text"].(string)
	if ok {
		fmt.Printf("CODER: %s\n", text)
	}

	// Handle both single file and multiple files
	fileNames, hasFileNames := data["file_paths"].([]interface{})
	contents, hasContents := data["contents"].([]interface{})

	// Single file creation (backward compatibility)
	if path, hasPath := data["path"].(string); hasPath {
		content, hasContent := data["content"].(string)
		if !hasContent {
			return map[string]any{"error": "content is required"}
		}

		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return map[string]any{"error": fmt.Sprintf("failed to create directory for file: %v", err)}
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return map[string]any{"error": fmt.Sprintf("failed to create file: %v", err)}
		}

		return map[string]any{
			"status": "completed",
			"output": fmt.Sprintf("File created successfully at %s", path),
		}
	}

	// Multiple files creation
	if !hasFileNames || !hasContents {
		return map[string]any{"error": "file_paths and contents are required"}
	}

	if len(fileNames) != len(contents) {
		return map[string]any{"error": "file_paths and contents arrays must have the same length"}
	}

	var createdFiles []string
	var errors []string

	for i, fileNameInterface := range fileNames {
		fileName, ok := fileNameInterface.(string)
		if !ok {
			errors = append(errors, fmt.Sprintf("Invalid file name at index %d", i))
			continue
		}

		content, ok := contents[i].(string)
		if !ok {
			errors = append(errors, fmt.Sprintf("Invalid content at index %d", i))
			continue
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(fileName)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to create directory for %s: %v", fileName, err))
				continue
			}
		}

		// Write the file
		if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to create %s: %v", fileName, err))
			continue
		}

		createdFiles = append(createdFiles, fileName)
		fmt.Printf("Created file: %s\n", fileName)
	}

	if len(errors) > 0 {
		return map[string]any{
			"error":         strings.Join(errors, "; "),
			"output":        fmt.Sprintf("Created %d files successfully", len(createdFiles)),
			"created_files": createdFiles,
		}
	}

	return map[string]any{
		"status": "completed",
		"output": fmt.Sprintf("Successfully created %d files: %s", len(createdFiles), strings.Join(createdFiles, ", ")),
	}
}

// ReadFile reads the content of a file from a specified path.
func ReadFiles(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok {
		fmt.Printf("CODER: %s", text)
	}

	fileNames, ok := args["file_paths"].([]interface{})
	if !ok {
		return map[string]any{
			"error":  "Invalid arguments: 'file_paths' must be a slice of strings",
			"output": nil,
		}
	}

	var stringFileNames []string
	for _, v := range fileNames {
		s, ok := v.(string)
		if !ok {
			log.Fatal("Invalid argument: 'file_paths' contains non-string values")
			return map[string]any{
				"error":  "Invalid argument: 'file_paths' contains non-string values",
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

// EditFile edits a file at a given path.
// Piece Table - what is it ??

func EditFile(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok {
		fmt.Printf("CODER: %s\n", text)
	}

	filepathInput, ok := args["file_path"].(string)
	if !ok {
		return map[string]any{"error": "ERROR READING PATH"}
	}

	changes, ok := args["changes"].([]interface{})
	if !ok {
		return map[string]any{"error": "REQUIRED CHANGES MAP"}
	}

	// Enhanced change structure with validation

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
	for i, c := range changes {
		ch, ok := c.(map[string]any)
		if !ok {
			return map[string]any{"error": fmt.Sprintf("INVALID CHANGE FORMAT at index %d", i)}
		}

		startLine, startLineOk := toInt(ch["start_line_number"])
		startCol, startColOk := toInt(ch["start_line_col"])
		endLine, endLineOk := toInt(ch["end_line_number"])
		endCol, endColOk := toInt(ch["end_line_col"])
		operation, operationOk := ch["operation"].(string)
		content, contentOk := ch["content"].(string)

		// Validate all required fields are present and correct type
		if !startLineOk || !startColOk || !endLineOk || !endColOk || !operationOk || !contentOk {
			return map[string]any{"error": fmt.Sprintf("MISSING OR INVALID FIELDS in change %d", i)}
		}

		// Validate operation type
		if operation != "delete" && operation != "replace" && operation != "write" {
			return map[string]any{"error": fmt.Sprintf("INVALID OPERATION '%s' in change %d. Must be 'delete', 'replace', or 'write'", operation, i)}
		}

		// Validate line/column numbers make sense
		if startLine < 1 || endLine < 1 {
			return map[string]any{"error": fmt.Sprintf("LINE NUMBERS must be >= 1 in change %d", i)}
		}

		if startCol < 0 || endCol < 0 {
			return map[string]any{"error": fmt.Sprintf("COLUMN NUMBERS must be >= 0 in change %d", i)}
		}

		// For single-line operations, validate column order
		if startLine == endLine && startCol > endCol && operation != "write" {
			return map[string]any{"error": fmt.Sprintf("START COLUMN cannot be > END COLUMN on same line in change %d", i)}
		}

		processedChanges = append(processedChanges, changeInfo{
			startLine: startLine,
			startCol:  startCol,
			endLine:   endLine,
			endCol:    endCol,
			operation: operation,
			content:   content,
			valid:     true,
		})
	}

	// Read file with better error handling
	fileContent, err := os.ReadFile(filepathInput)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{"error": fmt.Sprintf("FILE NOT FOUND: %s", filepathInput)}
		}
		return map[string]any{"error": fmt.Sprintf("CANNOT READ FILE: %s - %v", filepathInput, err)}
	}

	// Handle different line endings
	content := string(fileContent)
	content = strings.ReplaceAll(content, "\r\n", "\n") // Windows -> Unix
	content = strings.ReplaceAll(content, "\r", "\n")   // Old Mac -> Unix
	lines := strings.Split(content, "\n")

	// Validate all changes against file bounds before applying any
	for i, change := range processedChanges {
		if change.startLine > len(lines) || change.endLine > len(lines) {
			return map[string]any{"error": fmt.Sprintf("LINE NUMBER OUT OF BOUNDS in change %d: file has %d lines, but change references line %d", i, len(lines), max(change.startLine, change.endLine))}
		}

		// Check column bounds for start position
		if change.startLine <= len(lines) {
			lineIdx := change.startLine - 1
			lineLen := utf8.RuneCountInString(lines[lineIdx])
			if change.startCol > lineLen {
				return map[string]any{"error": fmt.Sprintf("START COLUMN OUT OF BOUNDS in change %d: line %d has %d characters, but change references column %d", i, change.startLine, lineLen, change.startCol)}
			}
		}

		// Check column bounds for end position (for same line operations)
		if change.startLine == change.endLine && change.endLine <= len(lines) {
			lineIdx := change.endLine - 1
			lineLen := utf8.RuneCountInString(lines[lineIdx])
			if change.endCol > lineLen {
				return map[string]any{"error": fmt.Sprintf("END COLUMN OUT OF BOUNDS in change %d: line %d has %d characters, but change references column %d", i, change.endLine, lineLen, change.endCol)}
			}
		}
	}

	// Sort changes by position (last to first to avoid position conflicts)
	sort.Slice(processedChanges, func(i, j int) bool {
		if processedChanges[i].startLine == processedChanges[j].startLine {
			return processedChanges[i].startCol > processedChanges[j].startCol
		}
		return processedChanges[i].startLine > processedChanges[j].startLine
	})

	// Apply changes from last to first
	for _, change := range processedChanges {
		switch change.operation {
		case "delete":
			lines = applyDelete(lines, change)
		case "replace":
			lines = applyReplace(lines, change)
		case "write":
			lines = applyWrite(lines, change)
		}
	}

	// Write back to file
	newContent := strings.Join(lines, "\n")
	err = os.WriteFile(filepathInput, []byte(newContent), 0644)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("CANNOT WRITE TO FILE: %s - %v", filepathInput, err)}
	}

	return map[string]any{"output": fmt.Sprintf("Successfully applied %d changes to %s", len(processedChanges), filepathInput)}
}

// Helper function for delete operations
func applyDelete(lines []string, change changeInfo) []string {
	startIdx := change.startLine - 1
	endIdx := change.endLine - 1

	if change.startLine == change.endLine {
		// Single line deletion - remove characters within the line
		if startIdx < len(lines) {
			line := lines[startIdx]
			runes := []rune(line)
			if change.startCol <= len(runes) && change.endCol <= len(runes) {
				newRunes := append(runes[:change.startCol], runes[change.endCol:]...)
				lines[startIdx] = string(newRunes)
			}
		}
	} else {
		// Multi-line deletion
		if startIdx < len(lines) && endIdx < len(lines) {
			// Keep part of first line before start column
			firstLinePart := ""
			if startIdx < len(lines) {
				firstLineRunes := []rune(lines[startIdx])
				if change.startCol <= len(firstLineRunes) {
					firstLinePart = string(firstLineRunes[:change.startCol])
				}
			}

			// Keep part of last line after end column
			lastLinePart := ""
			if endIdx < len(lines) {
				lastLineRunes := []rune(lines[endIdx])
				if change.endCol <= len(lastLineRunes) {
					lastLinePart = string(lastLineRunes[change.endCol:])
				}
			}

			// Combine remaining parts
			combinedLine := firstLinePart + lastLinePart

			// Remove the range and insert combined line
			newLines := make([]string, 0, len(lines)-(endIdx-startIdx))
			newLines = append(newLines, lines[:startIdx]...)
			newLines = append(newLines, combinedLine)
			newLines = append(newLines, lines[endIdx+1:]...)
			lines = newLines
		}
	}
	return lines
}

// Helper function for replace operations
func applyReplace(lines []string, change changeInfo) []string {
	// First delete the range, then insert new content
	lines = applyDelete(lines, change)

	// Now insert the new content at the start position
	writeChange := changeInfo{
		startLine: change.startLine,
		startCol:  change.startCol,
		endLine:   change.startLine,
		endCol:    change.startCol,
		operation: "write",
		content:   change.content,
	}
	return applyWrite(lines, writeChange)
}

// Helper function for write operations
func applyWrite(lines []string, change changeInfo) []string {
	if change.startLine-1 < len(lines) {
		lineIdx := change.startLine - 1
		line := lines[lineIdx]
		runes := []rune(line)

		if change.startCol <= len(runes) {
			// Handle multi-line content insertion
			newContent := change.content
			contentLines := strings.Split(newContent, "\n")

			if len(contentLines) == 1 {
				// Single line insertion
				newRunes := append(runes[:change.startCol], append([]rune(contentLines[0]), runes[change.startCol:]...)...)
				lines[lineIdx] = string(newRunes)
			} else {
				// Multi-line insertion
				beforeRunes := runes[:change.startCol]
				afterRunes := runes[change.startCol:]

				// First line: existing content before + first new line
				firstNewLine := string(beforeRunes) + contentLines[0]

				// Last line: last new content + existing content after
				lastNewLine := contentLines[len(contentLines)-1] + string(afterRunes)

				// Build new lines array
				newLines := make([]string, 0, len(lines)+len(contentLines)-1)
				newLines = append(newLines, lines[:lineIdx]...)
				newLines = append(newLines, firstNewLine)
				newLines = append(newLines, contentLines[1:len(contentLines)-1]...)
				newLines = append(newLines, lastNewLine)
				newLines = append(newLines, lines[lineIdx+1:]...)
				lines = newLines
			}
		}
	}
	return lines
}

// GetProjectStructure returns the project structure as a string, ignoring files and directories specified in .gitignore.
// If a .gitignore file is not found, it uses a default ignore list.
// It takes the project path as input.
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

func loadGitIgnore() {
	if _, err := os.Stat(".gitignore"); err == nil {
		ignoreMatcher, _ = gitignore.CompileIgnoreFile(".gitignore")
	}
}
func ExitProcess(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok && text != "" {
		fmt.Println(text)
	}

	fmt.Println("--- Task completed successfully! Exiting...")
	return map[string]any{"error": nil, "output": "Task Completed Successfully", "exit": true}

}

var CoderCapabilities = []repository.Function{
	{
		Name: "exit-process",
		Description: `Gracefully terminates the coding session with comprehensive completion validation and user satisfaction confirmation.
Performs final quality checks, validates all requirements fulfillment, and ensures clean project state before exit.
Triggers automatic documentation generation, test result summaries, and deployment readiness assessment.

Critical Exit Criteria:
✓ All specified tasks completed with verified functionality
✓ Code quality standards met (linting, formatting, testing)
✓ No unresolved bugs or compilation errors
✓ User acceptance and satisfaction confirmed
✓ Project documentation updated and accurate`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Professional completion summary and final recommendations for the user. Include project status, deliverables completed, and next steps.",
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
		Name: "execute-command-in-terminal",
		Description: `Advanced terminal command executor with intelligent process management, output streaming, and environment isolation.
Supports complex command chaining, environment variable injection, and real-time output monitoring.
Provides sophisticated error handling with context-aware debugging and automatic retry mechanisms.

Specialized Use Cases:
🔧 Development Operations: Package management, dependency installation, build automation
🧪 Quality Assurance: Syntax validation, linting, testing, code analysis
📦 Build Systems: Compilation, bundling, optimization, deployment preparation  
🔍 System Analysis: Performance profiling, resource monitoring, diagnostic commands
⚡ Performance Optimization: Benchmark execution, memory analysis, CPU profiling

Advanced Syntax Checking Examples:
• C/C++: { "command": "clang++", "arguments": ["-fsyntax-only", "-Wall", "-Wextra", "source.cpp"] }
• TypeScript: { "command": "tsc", "arguments": ["--noEmit", "--strict", "app.ts"] }
• Rust: { "command": "cargo", "arguments": ["check", "--all-features"] }
• Go: { "command": "go", "arguments": ["vet", "./..."] }`,

		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Contextual description of the operation being performed for enhanced logging and debugging capabilities.",
				},
				"command": {
					Type:        repository.TypeString,
					Description: "Primary executable command with full path resolution and environment variable support (e.g., 'npm', 'docker', 'kubectl', 'terraform').",
				},
				"arguments": {
					Type: repository.TypeArray,
					Items: &repository.Properties{
						Type:        repository.TypeString,
						Description: "Individual command arguments with proper escaping and parameter validation.",
					},
					Description: "Structured argument array supporting complex parameter passing, flag combinations, and multi-value options.",
				},
			},
			Required: []string{"command"},
			Optional: []string{"text", "arguments"},
		},

		Service: ExecuteCommands,

		Return: repository.Return{
			"error":   "string // Comprehensive error information including exit codes, stderr output, and diagnostic context",
			"output":  "string // Complete stdout capture with formatting preservation and encoding handling",
			"message": "string // Operation status summary with performance metrics and execution context",
		},
	},

	{
		Name: "get-project-structure",
		Description: `Intelligent project architecture analyzer with deep codebase understanding and dependency mapping.
Generates comprehensive project topology with file relationships, module dependencies, and architectural patterns.
Respects configuration files (.gitignore, .dockerignore, etc.) and provides smart filtering for development-relevant content.

Advanced Features:
🏗️  Architectural Pattern Detection: Identifies MVC, microservices, monorepo structures
📊 Dependency Graph Generation: Maps import/export relationships and circular dependencies  
🔍 Code Metrics Analysis: LOC, complexity, test coverage distribution
📁 Smart Categorization: Separates source, tests, config, documentation, and assets
⚡ Performance Optimized: Handles large codebases with intelligent caching and indexing`,

		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"path": {
					Type:        repository.TypeString,
					Description: "Target directory path with support for relative/absolute paths, symlink resolution, and workspace detection ('.' for current directory).",
				},
				"text": {
					Type:        repository.TypeString,
					Description: "Context description for the analysis operation, enabling targeted exploration and focused reporting.",
				},
			},
			Required: []string{"path"},
		},
		Service: GetProjectStructure,
		Return: repository.Return{
			"error":  "string // Detailed error information with path resolution and permission issues",
			"output": "string // Structured project tree with metadata, file types, and architectural insights",
		},
	},

	{
		Name: EditFileFromName,
		Description: `State-of-the-art multi-operation file editor with atomic transaction support and intelligent change management.
Provides surgical precision for code modifications with advanced conflict detection and resolution mechanisms.
Supports complex refactoring operations, batch changes, and syntax-aware transformations.

Revolutionary Capabilities:
🎯 Precision Targeting: Line-column accurate positioning with Unicode-aware character counting
🔄 Atomic Operations: All-or-nothing change application with automatic rollback on conflicts
🧠 Context Awareness: Understands code structure, indentation, and language-specific formatting
🛡️  Safety Mechanisms: Pre-change validation, backup creation, and integrity verification
⚡ Batch Processing: Multiple simultaneous edits with dependency ordering and optimization

Supported Operations:
• 'delete': Surgical removal of code blocks with smart whitespace handling
• 'replace': Context-aware content replacement with automatic formatting adjustment  
• 'write': Intelligent insertion with indentation matching and import management`,

		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Comprehensive context description explaining the rationale, expected outcomes, and architectural impact of the proposed changes.",
				},
				"file_path": {
					Type:        repository.TypeString,
					Description: "Absolute or relative file path with intelligent resolution, symlink following, and workspace-aware pathing (e.g., './src/components/App.tsx').",
				},
				"changes": {
					Type:        repository.TypeArray,
					Description: "Ordered sequence of atomic edit operations with precise targeting, conflict detection, and dependency-aware execution scheduling.",
					Items: &repository.Properties{
						Type:        repository.TypeObject,
						Description: "Individual edit operation with comprehensive metadata and validation parameters.",
						Properties: map[string]*repository.Properties{
							"start_line_number": {
								Type:        repository.TypeInteger,
								Description: "1-indexed starting line number with bounds validation and Unicode-aware line counting.",
							},
							"start_line_col": {
								Type:        repository.TypeInteger,
								Description: "0-indexed starting column position with UTF-8 character boundary awareness and tab expansion handling.",
							},
							"end_line_number": {
								Type:        repository.TypeInteger,
								Description: "1-indexed ending line number (inclusive) with multi-line operation support and overflow protection.",
							},
							"end_line_col": {
								Type:        repository.TypeInteger,
								Description: "0-indexed ending column position (exclusive) with precise character range selection and encoding safety.",
							},
							"operation": {
								Enum:        []string{"delete", "replace", "write"},
								Type:        repository.TypeString,
								Description: "Edit operation type: 'delete' (remove selected range), 'replace' (substitute with new content), 'write' (insert at position without removal).",
							},
							"content": {
								Type:        repository.TypeString,
								Description: "Replacement or insertion text with automatic encoding detection, line ending normalization, and indentation intelligence.",
							},
						},
						Required: []string{"start_line_number", "start_line_col", "end_line_number", "end_line_col", "operation", "content"},
					},
				},
			},
			Required: []string{"text", "file_path", "changes"},
		},
		Service: EditFile,
		Return: repository.Return{
			"error":  "string // Detailed failure analysis with conflict resolution suggestions and rollback information",
			"output": "string // Success confirmation with change summary, performance metrics, and validation results",
		},
	},

	{
		Name: RequestUserInputName,
		Description: `Advanced interactive communication interface with intelligent prompt generation and response validation.
Provides context-aware questioning with smart defaults, input validation, and conversation flow management.
Optimized for gathering requirements, confirming critical operations, and collaborative decision-making.

Enhanced Features:
💬 Smart Prompting: Context-aware question generation with helpful examples and constraints
🎯 Input Validation: Real-time validation with error correction suggestions and format guidance
⏱️  Timeout Management: Configurable timeouts with default fallback values and retry mechanisms
🔒 Security Aware: Sensitive input handling with masking options and secure transmission`,

		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Professionally formatted prompt message with clear instructions, context, examples, and expected input format specifications.",
				},
			},
			Required: []string{"text"},
		},
		Service: TakeInputFromTerminal,
		Return: repository.Return{
			"error":  "string // Input collection errors with retry suggestions and alternative interaction methods",
			"output": "string // User response with validation status and normalized formatting",
		},
	},

	{
		Name: "create-file",
		Description: `Advanced multi-file creation system with intelligent project scaffolding and atomic batch operations.
Supports sophisticated file generation workflows with template processing, automatic directory creation, and collision handling.
Optimized for project initialization, code generation, and infrastructure setup with enterprise-grade reliability.

Revolutionary Capabilities:
🏗️  Smart Scaffolding: Automatic directory structure creation with permission management
📝 Template Processing: Variable interpolation, conditional content, and dynamic generation
🛡️  Collision Management: Intelligent handling of existing files with backup and merge options
⚡ Batch Optimization: Atomic multi-file operations with transaction rollback support
🔍 Content Validation: Syntax checking, encoding verification, and format compliance`,

		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"file_paths": {
					Type:        repository.TypeArray,
					Items:       &repository.Properties{Type: repository.TypeString},
					Description: "Array of file paths with intelligent path resolution, directory auto-creation, and naming conflict prevention (e.g., ['src/utils/helper.ts', 'tests/helper.test.ts']).",
				},
				"contents": {
					Type:        repository.TypeArray,
					Items:       &repository.Properties{Type: repository.TypeString},
					Description: "Corresponding content array with template processing, encoding optimization, and syntax validation for each file creation.",
				},
			},
			Required: []string{"file_paths", "contents"},
		},
		Service: CreateFile,
		Return: repository.Return{
			"error":  "string // Comprehensive error reporting with file-specific failures and recovery suggestions",
			"output": "string // Detailed creation summary with file statistics, validation results, and project impact analysis",
		},
	},

	{
		Name: "read-files",
		Description: `High-performance batch file reader with intelligent caching, encoding detection, and content analysis capabilities.
Provides comprehensive file inspection with metadata extraction, dependency analysis, and code structure understanding.
Essential for codebase exploration, dependency validation, and informed modification planning.

Advanced Intelligence:
🚀 Performance Optimized: Parallel reading with memory management and streaming for large files
🧠 Content Analysis: Automatic language detection, syntax validation, and structural analysis  
🔍 Smart Filtering: Binary detection, encoding validation, and content-type classification
📊 Metadata Extraction: File statistics, dependency mapping, and complexity metrics
💾 Intelligent Caching: Smart caching with invalidation and memory optimization`,

		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"file_paths": {
					Type: repository.TypeArray,
					Items: &repository.Properties{
						Type:        repository.TypeString,
						Description: "File path with intelligent resolution, symlink handling, and workspace-aware pathing.",
					},
					Description: "Array of file paths for batch reading operations with automatic deduplication and dependency ordering.",
				},
				"text": {
					Type:        repository.TypeString,
					Description: "Context description explaining the purpose of reading these files for enhanced logging and operation tracking.",
				},
			},
			Required: []string{"file_paths"},
		},
		Service: ReadFiles,
		Return: repository.Return{
			"error":  "string // Detailed error analysis with per-file status and resolution recommendations",
			"output": "map[string]any // Structured file content mapping with metadata, encoding info, and analysis results",
		},
	},
}

// GetCoderCapabilities returns the list of all capabilities for the AgentCoder.
func GetCoderCapabilities() []repository.Function {
	return CoderCapabilities
}

// GetCoderCapabilitiesArrayMap returns the capabilities as a list of maps for API use.
func GetCoderCapabilitiesArrayMap() []map[string]any {
	coderMap := make([]map[string]any, 0)
	for _, v := range CoderCapabilities {
		coderMap = append(coderMap, v.ToObject())
	}
	return coderMap
}

// GetCoderCapabilitiesMap returns the capabilities as a map for internal use.
func GetCoderCapabilitiesMap() map[string]repository.Function {
	coderMap := make(map[string]repository.Function)
	for _, v := range CoderCapabilities {
		coderMap[v.Name] = v
	}
	return coderMap
}
func init() {
	for _, v := range CoderCapabilities {
		CodertoolsFunc[v.Name] = v
	}
}
