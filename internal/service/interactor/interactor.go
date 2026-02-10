package interactor

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

	"github.com/google/uuid"
	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/viper"
)

var ignoreMatcher *gitignore.GitIgnore
var InteractorToolsFunc map[string]repository.Function = make(map[string]repository.Function)
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

func formatFilesForInteractor(filesRead map[string]any) string {
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

type AgentInteractor repository.Agent

const interactorName = "Interactor"
const interactorVersion = "0.1.0"

// NewAgent creates and initializes a new AgentInteractor instance.
func (c *AgentInteractor) NewAgent() {
	model := viper.GetString("INTERACTOR.MODEL")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	endPoints := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse", model)
	id := fmt.Sprintf("interactor-%s", uuid.NewString())

	agentMetadata := repository.AgentMetadata{
		ID:             id,
		Name:           interactorName,
		Version:        interactorVersion,
		Type:           repository.AgentInteractor,
		Instructions:   repository.InteractorInstructions,
		MaxConcurrency: 5,
		Timeout:        10 * time.Minute,
		Tags:           []string{"interactor", "interaction", "user", "conversation"},
		Endpoints:      map[string]string{"http": endPoints},
		Context:        make(map[string]any),
		Status:         "active",
		LastActive:     time.Now(),
	}

	c.Metadata = agentMetadata
	c.Capabilities = InteractorCapabilities

}

func (p *AgentInteractor) Interaction(args map[string]any) map[string]any {
	InitialContext := GetInitialContext()
	InitialContextBytes, err := json.Marshal(InitialContext)
	if err != nil {
		return map[string]any{"error": "Unable to get initial context"}
	}

	var userMessage strings.Builder

	filesContent := formatFilesForInteractor(analysis.FilesRead)
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
				if function, exists := InteractorToolsFunc[name]; exists {
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

func (c *AgentInteractor) RequestAgent(contents []map[string]any) map[string]any {
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
			{"functionDeclarations": GetInteractorCapabilitiesArrayMap()},
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
		client := repository.NewStreamingHTTPClient()
		resp, err := client.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return map[string]any{"error": err.Error(), "output": nil}
			}
			time.Sleep(repository.ExponentialBackoff(attempt))
			continue
		}
		defer resp.Body.Close()

		// Success case - parse streaming response
		if resp.StatusCode == http.StatusOK {
			// Parse SSE stream with real-time display
			streamResp, err := repository.ParseSSEStream(resp.Body, true)
			if err != nil {
				if attempt == maxRetries {
					return map[string]any{"error": err.Error(), "output": nil}
				}
				time.Sleep(repository.ExponentialBackoff(attempt))
				continue
			}

			// Convert to standard output format
			output := repository.ConvertStreamResponseToOutput(streamResp)

			// Save chat history
			historyEntry := repository.BuildChatHistoryFromStream(streamResp, "interactor")
			if historyEntry != nil {
				ChatHistory = append(ChatHistory, historyEntry)
			}

			c.Metadata.LastActive = time.Now()
			return map[string]any{"error": nil, "output": output}
		}

		// Read body for error cases
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			if attempt == maxRetries {
				return map[string]any{"error": err.Error(), "output": nil}
			}
			time.Sleep(repository.ExponentialBackoff(attempt))
			continue
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

var InteractorCapabilities = []repository.Function{}

func GetInteractorCapabilities() []repository.Function {
	return InteractorCapabilities
}

// GetInteractorCapabilitiesArrayMap returns the capabilities as a list of maps for API use.
func GetInteractorCapabilitiesArrayMap() []map[string]any {
	coderMap := make([]map[string]any, 0)
	for _, v := range InteractorCapabilities {
		coderMap = append(coderMap, v.ToObject())
	}
	return coderMap
}

// GetInteractorCapabilitiesMap returns the capabilities as a map for internal use.
func GetInteractorCapabilitiesMap() map[string]repository.Function {
	coderMap := make(map[string]repository.Function)
	for _, v := range InteractorCapabilities {
		coderMap[v.Name] = v
	}
	return coderMap
}
func init() {
	for _, v := range InteractorCapabilities {
		InteractorToolsFunc[v.Name] = v
	}
}
