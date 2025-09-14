package coder

import (
	"bufio"
	"bytes"
	"context"
	"dost/internal/repository"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

const CreateFileDescription = "Creates a new file at a specified path with the given content. Overwrites if the file exists."
const ReadFileDescription = "Reads the content of a file from a specified path."
const ListDirectoryDescription = "Lists all files and subdirectories in a given directory path."
const DeleteFileOrDirDescription = "Deletes a file or directory at the specified path."
const EditFileDescription = "Edits a file at a given path by applying a specified change or replacement to its content."
const RequestUserInputDescription = "Prompts the user for a single line of input from the terminal and returns the response."

var ChatHistory = make([]map[string]any, 0)
var CodertoolsFunc map[string]repository.Function = make(map[string]repository.Function)
var ignoreMatcher *gitignore.GitIgnore

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

type AgentCoder repository.Agent

const coderName = "coder"

const coderVersion = "0.1.0"

// File Handling Structures
type Piece struct {
	Buffer string // "original" or "add"
	Start  int
	Length int
}

type PieceTable struct {
	Original string  // immutable original buffer
	Add      string  // append-only buffer
	Pieces   []Piece // sequence of text slices
}

// args must and only contains "query"
func (p *AgentCoder) Interaction(args map[string]any) map[string]any {
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
		fmt.Println(output)
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
				if function, exists := CodertoolsFunc[name]; exists {
					result := function.Run(argsData)
					if _, ok = result["exit"].(bool); ok {
						return map[string]any{"coder-id": result["output"]}
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
					break

				}
			}
		}

		// Continue the conversation loop
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

// executeCoderFunction executes a coder capability based on the function name.
func (c *AgentCoder) executeCoderFunction(funcName string, args map[string]any) map[string]any {
	capabilities := GetCoderCapabilitiesMap()
	if capability, exists := capabilities[funcName]; exists {
		return capability.Service(args)
	}
	return nil
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
	fileNames, hasFileNames := data["file_names"].([]interface{})
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
		return map[string]any{"error": "file_names and contents are required"}
	}

	if len(fileNames) != len(contents) {
		return map[string]any{"error": "file_names and contents arrays must have the same length"}
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

	// Sort changes by start line & col (ascending, so streaming works correctly)
	sort.Slice(processedChanges, func(i, j int) bool {
		if processedChanges[i].startLine == processedChanges[j].startLine {
			return processedChanges[i].startCol < processedChanges[j].startCol
		}
		return processedChanges[i].startLine < processedChanges[j].startLine
	})

	// Prepare temp file with same extension
	ext := filepath.Ext(filepathInput)
	tmpFile := fmt.Sprintf("%s_%d%s", strings.TrimSuffix(filepathInput, ext), time.Now().UnixNano(), ext)

	inFile, err := os.Open(filepathInput)
	if err != nil {
		return map[string]any{"error": "CANNOT OPEN INPUT FILE"}
	}
	defer inFile.Close()

	outFile, err := os.Create(tmpFile)
	if err != nil {
		return map[string]any{"error": "CANNOT CREATE TEMP FILE"}
	}
	defer outFile.Close()

	scanner := bufio.NewScanner(inFile)
	writer := bufio.NewWriter(outFile)

	currentLine := 1
	changeIdx := 0

	for scanner.Scan() {
		line := scanner.Text()

		// If there are changes relevant to this line
		for changeIdx < len(processedChanges) &&
			processedChanges[changeIdx].startLine <= currentLine &&
			processedChanges[changeIdx].endLine >= currentLine {

			change := processedChanges[changeIdx]

			switch change.operation {
			case "delete":
				if currentLine >= change.startLine && currentLine <= change.endLine {
					// Skip writing this range
					if currentLine == change.endLine {
						changeIdx++
					}
					goto nextLine
				}
			case "replace":
				if currentLine == change.startLine && currentLine == change.endLine {
					newLine := line[:change.startCol] + change.content + line[change.endCol:]
					_, _ = writer.WriteString(newLine + "\n")
					changeIdx++
					goto nextLine
				}
			case "write":
				if currentLine == change.startLine {
					newLine := line[:change.startCol] + change.content + line[change.startCol:]
					_, _ = writer.WriteString(newLine + "\n")
					changeIdx++
					goto nextLine
				}
			}
		}

		// Normal write if no changes applied
		_, _ = writer.WriteString(line + "\n")

	nextLine:
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		return map[string]any{"error": "ERROR SCANNING FILE"}
	}

	// Flush writer
	_ = writer.Flush()

	// Replace original file with edited file
	_ = os.Remove(filepathInput)
	_ = os.Rename(tmpFile, filepathInput)

	return map[string]any{"output": "Successfully written the content you provided"}
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
	fmt.Println(builder.String())
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

// --------------------------------------------------//
//
//									//
//	IMPORTANT						//
//									//
//
// --------------------------------------------------//
func NewPieceTable(content string) *PieceTable {
	return &PieceTable{
		Original: content,
		Add:      "",
		Pieces:   []Piece{{Buffer: "original", Start: 0, Length: len(content)}},
	}
}

func (pt *PieceTable) String() string {
	var builder strings.Builder
	for _, piece := range pt.Pieces {
		switch piece.Buffer {
		case "original":
			builder.WriteString(pt.Original[piece.Start : piece.Start+piece.Length])
		case "add":
			builder.WriteString(pt.Add[piece.Start : piece.Start+piece.Length])
		}
	}
	return builder.String()
}

func (pt *PieceTable) Insert(pos int, text string) {
	// Append to add buffer
	addStart := len(pt.Add)
	pt.Add += text

	newPiece := Piece{Buffer: "add", Start: addStart, Length: len(text)}

	offset := 0
	for i, piece := range pt.Pieces {
		if offset+piece.Length >= pos {
			// Found the piece where insertion happens
			relPos := pos - offset

			// Split piece into before + new + after
			before := Piece{Buffer: piece.Buffer, Start: piece.Start, Length: relPos}
			after := Piece{Buffer: piece.Buffer, Start: piece.Start + relPos, Length: piece.Length - relPos}

			newPieces := []Piece{}
			if before.Length > 0 {
				newPieces = append(newPieces, before)
			}
			newPieces = append(newPieces, newPiece)
			if after.Length > 0 {
				newPieces = append(newPieces, after)
			}

			pt.Pieces = append(pt.Pieces[:i], append(newPieces, pt.Pieces[i+1:]...)...)
			return
		}
		offset += piece.Length
	}
}

func (pt *PieceTable) Delete(start, length int) {
	end := start + length
	offset := 0
	newPieces := []Piece{}

	for _, piece := range pt.Pieces {
		if offset+piece.Length <= start || offset >= end {
			// Outside deletion range, keep piece
			newPieces = append(newPieces, piece)
		} else {
			// Overlaps deletion range
			relStart := max(0, start-offset)
			relEnd := min(piece.Length, end-offset)

			if relStart > 0 {
				newPieces = append(newPieces, Piece{
					Buffer: piece.Buffer,
					Start:  piece.Start,
					Length: relStart,
				})
			}
			if relEnd < piece.Length {
				newPieces = append(newPieces, Piece{
					Buffer: piece.Buffer,
					Start:  piece.Start + relEnd,
					Length: piece.Length - relEnd,
				})
			}
		}
		offset += piece.Length
	}
	pt.Pieces = newPieces
}

var CoderCapabilities = []repository.Function{
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
		Name: "execute-command-in-terminal",
		Description: `Run any valid shell/terminal command in the current working directory.
Use this for:
- File operations (create, delete, list, copy, move, rename, git).
- Navigation (cd, ls).
- Installing packages, running build tools.
- Executing programs, checking versions, inspecting system state.
- Running compilers/interpreters in syntax-check mode (e.g., g++ -fsyntax-only, python -m py_compile, node --check, go vet).
This is especially useful for analyzing syntax errors in different programming languages.
Always provide the command and arguments separately.

Example:
{ "command": "git", "arguments": ["commit", "-m", "testing through this tool"] }

For syntax checking:
{ "command": "g++", "arguments": ["-fsyntax-only", "file.cpp"] }
{ "command": "python", "arguments": ["-m", "py_compile", "script.py"] }
{ "command": "node", "arguments": ["--check", "app.js"] }`,

		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Optional message or context string for logging/debugging.",
				},
				"command": {
					Type:        repository.TypeString,
					Description: "Base command to execute (e.g., git, go, npm, python, ls).",
				},
				"arguments": {
					Type: repository.TypeArray,
					Items: &repository.Properties{
						Type:        repository.TypeString,
						Description: "Each argument passed separately (e.g., [\"commit\", \"-m\", \"msg\"]).",
					},
					Description: "Optional arguments for the command as an array.",
				},
			},
			Required: []string{"command"},
			Optional: []string{"text", "arguments"},
		},

		Service: ExecuteCommands,

		Return: repository.Return{
			"error":   "string // Error message if the command failed, including stderr output.",
			"output":  "string // Captured stdout (program output).",
			"message": "string // Success message when execution completes.",
		},
	},
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
		Name:        EditFileFromName,
		Description: EditFileDescription,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Additional context or instructions provided by the AI for the edit operation.",
				},
				"file_path": {
					Type:        repository.TypeString,
					Description: "The path of the file to edit. Example: './folder/file.go'.",
				},
				"changes": {
					Type:        repository.TypeArray,
					Description: "A list of changes to apply to the file, with each change specifying a selection and an operation.",
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
		Name:        RequestUserInputName,
		Description: RequestUserInputDescription,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "The message to display to the user when requesting input.",
				},
			},
			Required: []string{"text"},
		},
		Service: TakeInputFromTerminal,
	},
	{
		Name: "create-file",
		Description: `
Creates new files with the given names and contents.
Make sure that the file names and the file contents are in the same order.
Call this when the files do not exist and you need to create them.
If the files might already exist, first call 'read_files' to check.

`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"file_names": {
					Type:        repository.TypeArray,
					Items:       &repository.Properties{Type: repository.TypeString},
					Description: "Name of the file to create (with extension)",
				},
				"contents": {
					Type:        repository.TypeArray,
					Items:       &repository.Properties{Type: repository.TypeString},
					Description: "Full content to write in the new file",
				},
			},
			Required: []string{"file_names", "contents"},
		},
		Service: CreateFile,
		Return:  repository.Return{"error": "string", "output": "string"},
	},
	{
		Name: "read-files",
		Description: `Reads the content of one or more files and returns them.
Call this whenever you need to:
- Check existing code before modifying it
- Inspect dependencies or configuration
- Validate if a file exists
`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"file_names": {
					Type: repository.TypeArray,
					Items: &repository.Properties{
						Type:        repository.TypeString,
						Description: "Name of the file to read",
					},
					Description: "List/Slice of file names to read",
				},
				"text": {
					Type:        repository.TypeString,
					Description: "A text which you want to say to user, instead of returning text output give it in this parameter",
				},
			},
			Required: []string{"file_names"},
		},
		Service: ReadFiles,
		Return:  repository.Return{"error": "string", "output": "map[string]any"},
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
