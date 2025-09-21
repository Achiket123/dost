package analysis

import (
	"bufio"
	"bytes"
	"context"
	"dost/internal/repository"
	"dost/internal/service"
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
var ignoreMatcher *gitignore.GitIgnore
var AnalysisMap = make(map[string]Analysis, 0)
var AnalysistoolsFunc map[string]repository.Function = make(map[string]repository.Function)
var InputData = make(map[string]any, 0)
var FilesRead = make(map[string]any, 0)
var ChatHistory = make([]map[string]any, 0)

type Analysis struct {
	ID              string         `json:"id"`
	DetailSummary   string         `json:"detail_summary"`
	Summary         string         `json:"summary"`
	Domain          string         `json:"domain"`
	Constraints     Constraints    `json:"constraints"`
	InputsDetected  Inputs         `json:"inputs_detected"`
	Risks           []string       `json:"risks"`
	ExpectedOutput  OutputFormat   `json:"expected_output"`
	OperatingSystem string         `json:"operating_system"`
	QueryInputs     map[string]any `json:"query_input"`
	FilesRead       map[string]any `json:"files_read"`
}

type Constraints struct {
	Hard []string `json:"hard"`
	Soft []string `json:"soft"`
}

type Inputs struct {
	Files       []string `json:"files,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Language    string   `json:"language,omitempty"`
}

type OutputFormat struct {
	Format     string                 `json:"format"`
	Example    map[string]interface{} `json:"example"`
	OutputType string                 `json:"output_type"`
}

type AgentAnalysis repository.Agent

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

// args must and only contains "query"
// Helper function to format files for Gemini
func formatFilesForGemini(filesRead map[string]any) string {
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

func (p *AgentAnalysis) Interaction(args map[string]any) map[string]any {
	InitialContext := GetInitialContext()
	InitialContextBytes, err := json.Marshal(InitialContext)
	if err != nil {
		return map[string]any{"error": "Unable to get initial context"}
	}

	// Build consolidated user message with proper formatting
	var userMessage strings.Builder

	// Add files content if any
	filesContent := formatFilesForGemini(FilesRead)
	if filesContent != "" {
		userMessage.WriteString(filesContent)
		userMessage.WriteString("\n")
	}

	// Add initial context
	userMessage.WriteString("=== INITIAL CONTEXT ===\n")
	userMessage.WriteString(string(InitialContextBytes))
	userMessage.WriteString("\n\n")

	// Add the actual query
	userMessage.WriteString("=== QUERY ===\n")
	if query, ok := args["query"].(string); ok {
		userMessage.WriteString(query)
	}
	log.Println("TEST: ANALYSIS: ", userMessage.String()[0:20])
	// Add as a single consolidated user message
	ChatHistory = append(ChatHistory, map[string]any{
		"role": "user",
		"parts": []map[string]any{
			{
				"text": userMessage.String(),
			},
		},
	})

	for {
		// Check if last message was from model and add exit instruction if needed
		if len(ChatHistory) > 0 && ChatHistory[len(ChatHistory)-1]["role"] == "model" {
			ChatHistory = append(ChatHistory, map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{
						"text": "If you feel there is no task left and you have created the Analysis by calling the put-analysis-agent-output function call then and then only call exit-process. Because only that can stop you and finish the program. Don't respond with text, no text output should be there, call the exit-process. Mark It the put-analysis-agent-output should be called before exit. PERIOD",
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
				if function, exists := AnalysistoolsFunc[name]; exists {
					result := function.Run(argsData)

					// Check for exit condition
					if _, ok := result["exit"].(bool); ok {
						analysisID, ok := result["output"].(string)
						if ok {
							analysis := AnalysisMap[analysisID]
							analysis.QueryInputs = InputData
							AnalysisMap[analysisID] = analysis
							return map[string]any{"analysis-id": result["output"]}
						} else {
							return map[string]any{"analysis-id": result}
						}
					}

					// Add function response to chat history with proper structure
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

					// Add error response to chat history
					ChatHistory = append(ChatHistory, map[string]any{
						"role": "user",
						"parts": []map[string]any{
							{
								"text": fmt.Sprintf("Error: Function '%s' not found", name),
							},
						},
					})
				}
			}
		}

		// Continue the conversation loop
		fmt.Println("---")
	}
}

func (p *AgentAnalysis) NewAgent() {
	model := viper.GetString("ANALYSIS.MODEL")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	endPoints := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)

	analysisAgentMeta := repository.AgentMetadata{
		ID:             "analysis-agent-v1",
		Name:           "Analysis Agent",
		Version:        "1.0.0",
		Type:           repository.AgentType(repository.AgentAnalysis),
		Instructions:   repository.AnalysisInstructions,
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
	p.Capabilities = AnalysisCapabilities
}

func (p *AgentAnalysis) RequestAgent(contents []map[string]any) map[string]any {
	fmt.Printf("Processing request with Analysis Agent: %s\n", p.Metadata.Name)

	// Build request payload for the AI model
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
			{"functionDeclarations": GetAnalysisToolsMap()},
		},
	}

	// Marshal request body
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
		req.Header.Set("X-goog-api-key", viper.GetString("ANALYSIS.API_KEY"))

		// Execute request with timeout
		client := &http.Client{Timeout: p.Metadata.Timeout}
		resp, err := client.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return map[string]any{"error": err.Error(), "output": nil}
			}
			// Wait before retrying network errors
			time.Sleep(repository.ExponentialBackoff(attempt))
			continue
		}
		defer resp.Body.Close()

		// Read response body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			if attempt == maxRetries {
				return map[string]any{"error": err.Error(), "output": nil}
			}
			time.Sleep(repository.ExponentialBackoff(attempt))
			continue
		}

		// Handle successful response
		if resp.StatusCode == http.StatusOK {
			var response repository.Response
			if err = json.Unmarshal(bodyBytes, &response); err != nil {
				return map[string]any{"error": err.Error(), "output": nil}
			}

			// Extract both text and function calls
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

			// Add the model's response to chat history
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

			return map[string]any{"error": nil, "output": output}
		}

		// Handle rate limit errors (429)
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == maxRetries {
				return map[string]any{
					"error": fmt.Sprintf("Rate limit exceeded after %d retries. HTTP %d: %s",
						maxRetries, resp.StatusCode, string(bodyBytes)),
					"output": nil,
				}
			}

			// Parse retry delay from response
			retryDelay := repository.ParseRetryDelay(string(bodyBytes))

			// Use provided delay or fallback to exponential backoff
			var waitTime time.Duration
			if retryDelay > 0 {
				waitTime = retryDelay
			} else {
				waitTime = repository.ExponentialBackoff(attempt)
			}

			// Cap wait time to reasonable maximum
			if waitTime > maxWaitTime {
				waitTime = maxWaitTime
			}

			fmt.Printf("Rate limit hit (attempt %d/%d). Waiting %v before retry...\n",
				attempt+1, maxRetries+1, waitTime)

			time.Sleep(waitTime)
			continue
		}

		// Handle other HTTP errors
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			// Server errors - retry with backoff
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

		// Client errors (400-499) except 429 - don't retry
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

// PutAgentOutput converts a generic map into AnalysisAgentOutput and prints/stores the JSON.
func PutAgentOutput(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok && text != "" {
		fmt.Println(text)
	}

	data, err := json.Marshal(args)
	if err != nil {
		fmt.Println("Error marshaling args:", err)
		return map[string]any{"error": err}
	}
	var output Analysis
	if err := json.Unmarshal(data, &output); err != nil {
		fmt.Println(" Error unmarshaling into AnalysisAgentOutput:", err)
		return map[string]any{"error": err}
	}
	output.OperatingSystem = runtime.GOOS
	output.FilesRead = FilesRead
	AnalysisMap[output.ID] = output

	fmt.Printf("… Stored analysis for task %s\n", output.ID)

	return map[string]any{"error": nil, "output": output.ID}
}

func TakeInputFromTerminal(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if !ok {
		return map[string]any{"error": "No Text Provided"}
	}
	fmt.Println(text)

	requirements, ok := args["requirements"].([]any)
	if !ok || len(requirements) == 0 {
		return map[string]any{
			"error":  nil,
			"output": map[string]string{"message": "No requirements provided"},
		}
	}

	results := make(map[string]string)
	reader := bufio.NewReader(os.Stdin)

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
		InputData[question] = results[question]
	}

	return map[string]any{"error": nil, "output": results}
}

func ExitProcess(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok && text != "" {
		fmt.Println(text)
	}

	fmt.Println("--- Task completed successfully! Exiting...")
	return map[string]any{"error": nil, "output": args["analysis-id"], "exit": true}

}
func ReadFiles(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok {
		fmt.Printf("ANALYSIS: %s", text)
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
		FilesRead[fileName] = chunks
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

var AnalysisCapabilities = []repository.Function{
	{
		Name: "get-project-strucuture",
		Description: `Returns the full directory structure of the project at the given path.
		Call this before working on unfamiliar projects to understand where files are located.
		Respects .gitignore to skip irrelevant files.`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"path": {
					Type:        repository.TypeString,
					Description: "Path to the project directory (usually '.')",
				},
				"text": {
					Type:        repository.TypeString,
					Description: "A text which you want to say to user, instead of returning text output give it in this parameter",
				},
			},
			Required: []string{"path"},
		},
		Service: GetProjectStructure,
		Return: repository.Return{
			"error":  "string",
			"output": "string",
		},
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
	{
		Name: "put-analysis-agent-output",
		Description: `Stores the output of the analysis agent in the in-memory map.
Use this after completing an analysis to persist the structured result (constraints, inputs, risks, expected output).`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"id": {
					Type:        repository.TypeString,
					Description: "Unique identifier for the analysis task",
				},
				"detail_summary": {
					Type:        repository.TypeString,
					Description: "Detailed summary of the analysis",
				},
				"summary": {
					Type:        repository.TypeString,
					Description: "Concise summary of the analysis",
				},
				"domain": {
					Type:        repository.TypeString,
					Description: "Domain or area of the task (e.g., software, AI, data)",
				},
				"constraints": {
					Type: repository.TypeObject,
					Properties: map[string]*repository.Properties{
						"hard": {
							Type:        repository.TypeArray,
							Items:       &repository.Properties{Type: repository.TypeString},
							Description: "Hard constraints (must follow)",
						},
						"soft": {
							Type:        repository.TypeArray,
							Items:       &repository.Properties{Type: repository.TypeString},
							Description: "Soft constraints (nice to follow)",
						},
					},
				},
				"inputs_detected": {
					Type: repository.TypeObject,
					Properties: map[string]*repository.Properties{
						"files": {
							Type:        repository.TypeArray,
							Items:       &repository.Properties{Type: repository.TypeString},
							Description: "Files detected in the project",
						},
						"environment": {
							Type:        repository.TypeString,
							Description: "Execution environment (Go, Python, Docker, etc.)",
						},
						"language": {
							Type:        repository.TypeString,
							Description: "Programming language(s) involved",
						},
					},
				},
				"risks": {
					Type:        repository.TypeArray,
					Items:       &repository.Properties{Type: repository.TypeString},
					Description: "List of identified risks or blockers",
				},
				"expected_output": {
					Type: repository.TypeObject,
					Properties: map[string]*repository.Properties{
						"format": {
							Type:        repository.TypeString,
							Description: "Format of the output (JSON, text, etc.)",
						},
						"example": {
							Type:        repository.TypeObject,
							Description: "Example of expected output structure",
						},
						"output_type": {
							Type:        repository.TypeString,
							Description: "Type of output (code, report, data, etc.)",
						},
					},
				},
				"text": {
					Type:        repository.TypeString,
					Description: "A text which you want to say to user, instead of returning text output give it in this parameter",
				},
			},
			Required: []string{
				"id",
				"detail_summary",
				"summary",
				"domain",
				"constraints",
				"inputs_detected",
				"risks",
				"expected_output",
				"text",
			},
		},
		Service: PutAgentOutput,
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
		Description: `When you feel that the task is fully completed, always call this function to exit the process.
⚠️ Important:
- Before calling, make sure all steps are done.
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
		Name: "execute-command-in-terminal",
		Description: `Execute ANY valid shell/terminal command with full system access in the current working directory.
This tool provides COMPLETE command-line interface capabilities, enabling:

🚀 PROJECT MANAGEMENT & SETUP:
- Initialize projects: npm init, cargo new, go mod init, django-admin startproject
- Setup frameworks: create-react-app, vue create, ng new, rails new
- Generate scaffolding: rails generate, ng generate, vue add
- Project templates: cookiecutter, yeoman generators, custom templates

📁 FILE & DIRECTORY OPERATIONS:
- Navigation: cd, ls, pwd, find, locate, which, whereis
- File management: cp, mv, rm, mkdir, rmdir, touch, ln
- Permissions: chmod, chown, chgrp, umask, getfacl, setfacl
- Content operations: cat, head, tail, grep, sed, awk, sort, uniq
- Archives: tar, zip, unzip, gzip, gunzip, 7z
- File comparison: diff, cmp, comm

🔧 DEVELOPMENT TOOLS & BUILD SYSTEMS:
- Compilers: gcc, g++, clang, rustc, javac, tsc, babel
- Interpreters: python, node, ruby, php, lua, perl
- Build tools: make, cmake, ninja, gradle, maven, ant, bazel
- Task runners: npm run, yarn, pnpm, gulp, grunt, webpack
- Linters: eslint, pylint, rubocop, clippy, checkstyle
- Formatters: prettier, black, gofmt, rustfmt, clang-format

📦 PACKAGE MANAGEMENT:
- Node.js: npm, yarn, pnpm (install, update, audit, publish)
- Python: pip, pip3, pipenv, poetry, conda
- Rust: cargo (build, test, publish, update)
- Go: go get, go mod (download, tidy, vendor)
- Ruby: gem, bundle
- PHP: composer
- Java: mvn, gradle
- System packages: apt, yum, brew, choco, pacman

🔄 VERSION CONTROL (GIT & OTHERS):
- Git operations: init, clone, add, commit, push, pull, merge, rebase
- Branch management: checkout, branch, switch, merge, rebase
- History: log, show, diff, blame, reflog
- Stashing: stash, stash pop, stash apply
- Remote management: remote add, fetch, pull, push
- Tags: tag, tag -a, tag -d
- Other VCS: svn, hg, bzr

🗄️ DATABASE OPERATIONS:
- SQL databases: mysql, psql, sqlite3, sqlcmd
- NoSQL: mongo, redis-cli, couchdb
- Migrations: migrate, flyway, liquibase
- Dumps & restores: mysqldump, pg_dump, mongodump

🌐 NETWORK & API OPERATIONS:
- HTTP requests: curl, wget, httpie
- Network tools: ping, telnet, netstat, ss, lsof
- DNS: nslookup, dig, host
- Certificates: openssl, certbot
- API testing: postman-cli, newman

🐳 CONTAINERIZATION & ORCHESTRATION:
- Docker: build, run, exec, ps, images, compose
- Kubernetes: kubectl (apply, get, describe, logs, exec)
- Container registries: docker push, docker pull
- Orchestration: docker-compose, docker swarm

☁️ CLOUD & INFRASTRUCTURE:
- AWS CLI: aws s3, ec2, lambda, cloudformation
- Azure CLI: az vm, az storage, az webapp
- Google Cloud: gcloud compute, gsutil
- Terraform: plan, apply, destroy
- Ansible: ansible-playbook, ansible-vault

🔍 SYSTEM MONITORING & DEBUGGING:
- Process management: ps, top, htop, kill, killall, jobs
- System info: uname, whoami, id, groups, env
- Disk usage: df, du, fdisk, lsblk
- Memory: free, vmstat
- Performance: iostat, sar, strace, ltrace
- Logs: journalctl, tail -f, grep logs

🧪 TESTING & QUALITY ASSURANCE:
- Unit tests: pytest, jest, go test, cargo test, rspec
- Integration tests: newman, postman, cypress
- Load testing: ab, siege, wrk, jmeter
- Security scans: nmap, nikto, owasp-zap
- Code coverage: coverage.py, nyc, gocov
- Benchmarking: hyperfine, bench, criterion

🔐 SECURITY & ENCRYPTION:
- SSH operations: ssh, scp, ssh-keygen, ssh-add
- Encryption: gpg, openssl, age
- Certificates: certbot, openssl req, keytool
- Password management: pass, 1password-cli
- Security scanning: bandit, safety, audit

📊 DATA PROCESSING & ANALYSIS:
- Text processing: sed, awk, grep, sort, cut, tr
- JSON/XML: jq, xmlstarlet, yq
- CSV processing: csvkit, miller
- Data conversion: pandoc, iconv
- Statistical tools: R, octave

🎯 AUTOMATION & SCRIPTING:
- Shell scripting: bash, zsh, fish, powershell
- Task scheduling: cron, at, systemd timers
- Process automation: expect, tmux, screen
- Workflow tools: github-cli, gitlab-ci

🖥️ SYSTEM ADMINISTRATION:
- Service management: systemctl, service, launchctl
- User management: useradd, usermod, passwd, su, sudo
- Network configuration: ifconfig, ip, route
- Firewall: iptables, ufw, firewall-cmd
- Package repositories: add-apt-repository, yum-config-manager

🔧 LANGUAGE-SPECIFIC TOOLS:
- Node.js: node, npm, yarn, npx, nvm
- Python: python, pip, virtualenv, conda, jupyter
- Go: go build, go test, go mod, go generate
- Rust: cargo build, cargo test, cargo publish
- Java: java, javac, maven, gradle
- C/C++: gcc, g++, make, cmake, gdb
- .NET: dotnet build, dotnet run, dotnet test
- Ruby: ruby, gem, bundle, rails
- PHP: php, composer, artisan

⚡ PERFORMANCE & OPTIMIZATION:
- Profiling: perf, gprof, valgrind, pprof
- Benchmarking: time, hyperfine, ab
- Memory analysis: valgrind, AddressSanitizer
- Code optimization: compiler flags, link-time optimization

🔄 CI/CD & DEPLOYMENT:
- GitHub Actions: gh workflow, gh run
- Jenkins: jenkins-cli
- Docker deployment: docker deploy, docker service
- Serverless: serverless deploy, sam deploy
- Static sites: netlify, vercel

SYNTAX CHECKING & VALIDATION:
- C/C++: g++ -fsyntax-only, clang -fsyntax-only
- Python: python -m py_compile, python -m flake8
- JavaScript: node --check, eslint
- Go: go vet, go fmt -n
- Rust: cargo check
- JSON: jq empty, python -m json.tool
- YAML: yamllint, python -c "import yaml"
- XML: xmllint --noout

ADVANCED OPERATIONS:
- Parallel processing: parallel, xargs -P
- Process monitoring: watch, timeout
- Binary analysis: objdump, nm, readelf, strings
- System calls: strace, dtrace
- Network debugging: tcpdump, wireshark-cli

IMPORTANT USAGE NOTES:
- This tool has FULL system access - use responsibly
- Can modify files, install software, change system settings
- Can access network resources and external APIs
- Can start/stop services and processes
- Always verify commands before execution in production
- Use appropriate error handling and validation

EXAMPLES:

Project Setup:
{ "command": "npx", "arguments": ["create-react-app", "my-app", "--template", "typescript"] }
{ "command": "cargo", "arguments": ["new", "my-rust-project"] }
{ "command": "django-admin", "arguments": ["startproject", "mysite"] }

Development:
{ "command": "npm", "arguments": ["install", "express", "cors", "dotenv"] }
{ "command": "go", "arguments": ["mod", "init", "github.com/user/project"] }
{ "command": "pip", "arguments": ["install", "-r", "requirements.txt"] }

Git Operations:
{ "command": "git", "arguments": ["clone", "https://github.com/user/repo.git"] }
{ "command": "git", "arguments": ["add", ".", "&&", "git", "commit", "-m", "Initial commit"] }
{ "command": "git", "arguments": ["push", "origin", "main"] }

Testing & Quality:
{ "command": "pytest", "arguments": ["tests/", "-v", "--coverage"] }
{ "command": "eslint", "arguments": ["src/", "--fix"] }
{ "command": "cargo", "arguments": ["test", "--release"] }

System Operations:
{ "command": "docker", "arguments": ["build", "-t", "myapp", "."] }
{ "command": "systemctl", "arguments": ["start", "nginx"] }
{ "command": "curl", "arguments": ["-X", "POST", "https://api.example.com/data"] }

This tool is your gateway to the ENTIRE command-line ecosystem. Use it to automate, build, deploy, test, monitor, and manage any aspect of software development and system administration.`,

		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Optional context message for logging, debugging, or providing additional information about the command execution.",
				},
				"command": {
					Type:        repository.TypeString,
					Description: "The base command to execute. Can be any valid system command, tool, or executable available in the PATH or specified with full path.",
				},
				"arguments": {
					Type: repository.TypeArray,
					Items: &repository.Properties{
						Type:        repository.TypeString,
						Description: "Individual command arguments, flags, options, and parameters.",
					},
					Description: "Array of command arguments. Each element should be a separate argument (proper shell escaping handled automatically).",
				},
			},
			Required: []string{"command"},
			Optional: []string{"text", "arguments"},
		},

		Service: ExecuteCommands,

		Return: repository.Return{
			"error":   "string // Detailed error message including command, arguments, exit code, and stderr output if execution fails.",
			"output":  "string // Complete stdout output from the executed command.",
			"message": "string // Success confirmation message with command execution details.",
		},
	},
}

func init() {
	for _, v := range AnalysisCapabilities {
		AnalysistoolsFunc[v.Name] = v
	}
}

func AnalysisTools() map[string]repository.Function {
	return AnalysistoolsFunc
}

func GetAnalysisTools() []repository.Function {
	return AnalysisCapabilities
}

func GetAnalysisToolsMap() []map[string]any {
	arrayOfMap := make([]map[string]any, 0)
	for _, v := range AnalysisCapabilities {
		arrayOfMap = append(arrayOfMap, v.ToObject())
	}
	return arrayOfMap
}
