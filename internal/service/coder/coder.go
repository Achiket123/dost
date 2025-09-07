package coder

import (
	"bytes"
	"context"
	"dost/internal/repository"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// CoderAgent is an autonomous agent specialized in code generation, analysis, and execution.
type CoderAgent repository.Agent

// coderName defines the name of the coder agent
const coderName = "coder"

// coderVersion defines the version of the coder agent
const coderVersion = "0.1.0"

// CoderInstructions provides the system instructions for the coder agent's LLM.
const CoderInstructions = `You are a highly skilled and resourceful coder agent. Your primary function is to write, debug, and refactor code, as well as to analyze technical specifications and produce high-quality, executable code.

## CRITICAL: FUNCTION-ONLY RESPONSES
**YOU MUST ONLY RESPOND WITH FUNCTION CALLS. NO TEXT RESPONSES ALLOWED.**
**NEVER return plain text, code blocks, or explanations directly.**
**ALL communication must be through the provided function capabilities.**

Your core responsibilities include:
1. Use create_file function to write complete code files
2. Use execute_code function to test implementations  
3. Use read_file function to analyze existing code
4. Use edit_file function to modify code
5. Use analyze_code function to review implementations
6. Use debug_code function to fix issues
7. **Tool Usage**: You **must** use these functions when the task requires interaction with the file system

## Available Functions (Use Exclusively)
- create_file: Create new files with complete content
- read_file: Read existing files
- edit_file: Modify existing files
- execute_code: Run and test code
- analyze_code: Analyze code quality
- debug_code: Fix code issues
- list_directory: Explore file structure
- create_directory: Create directories
- delete_file_or_dir: Remove files/directories
- refactor_code: Improve code structure
- generate_tests: Create test cases

**CRITICAL: C Programming Language Requirements**
For C programming tasks, you MUST provide COMPLETE implementations:

1. **Never Create Stub Functions**: All function declarations in .h files MUST have corresponding complete implementations in .c files
2. **Complete Function Bodies**: Every function must contain working code, not TODO comments or empty bodies  
3. **Header-Implementation Pairs**: For every .h file created, create the corresponding .c file with full implementations
4. **Working Main Function**: Always provide a main.c with a complete main() function that demonstrates the functionality
5. **Proper Includes**: Use correct #include directives and library dependencies
6. **Compilation Ready**: Code must compile with gcc without errors

**C Implementation Example Process**:
For creating "math_utils.h":
1. Use create_file with path="math_utils.h" and complete header declarations
2. Use create_file with path="math_utils.c" and COMPLETE function implementations (never stubs)
3. Use create_file with path="main.c" that uses and tests the functions
4. Use execute_code to test the implementation

## Function Usage Rules
1. **MANDATORY**: Every response must be a function call
2. **NO CODE BLOCKS**: Never return  code  blocks directly  
3. **COMPLETE IMPLEMENTATIONS**: Always provide working code, never stubs
4. **TEST YOUR CODE**: Use execute_code after creating files
5. **HANDLE ERRORS**: Use debug_code if execution fails

You must always strive to produce clean, efficient, and well-commented code. When writing code, assume a modern development environment unless specified otherwise. Before returning a code solution, you should consider edge cases, potential errors, and best practices.

When a user provides a coding task, your **first and foremost** action should be to determine what functions to call. Use create_file for new files, execute_code for testing, and other functions as needed. Do not generate text describing what you will do; simply call the appropriate functions directly.`

// NewAgent creates and initializes a new CoderAgent instance.
func (c *CoderAgent) NewAgent() *CoderAgent {
	model := viper.GetString("ORCHESTRATOR.MODEL")
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

	agent := repository.Agent{
		Metadata:     agentMetadata,
		Capabilities: GetCoderCapabilities(),
	}
	coderAgent := CoderAgent(agent)
	return &coderAgent
}

// RequestAgent is the main entry point for the CoderAgent to handle incoming tasks.
func (c *CoderAgent) RequestAgent(contents []map[string]any) map[string]any {
	fmt.Printf("Processing request with Coder Agent: %s\n", c.Metadata.Name)

	request := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]any{
				{"text": c.Metadata.Instructions},
			},
		},
		"contents": contents,
		"tools": []map[string]any{
			{"function_declarations": GetCoderCapabilitiesArrayMap()},
		},
	}

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

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

	client := &http.Client{Timeout: c.Metadata.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return map[string]any{
			"error":  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)),
			"output": nil,
		}
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	var response repository.Response
	if err = json.Unmarshal(bodyBytes, &response); err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	newResponse := make([]map[string]any, 0)
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				// REJECT TEXT RESPONSES - Force function-only responses
				newResponse = append(newResponse, map[string]any{
					"error": "Text responses not allowed. Must use function calls only.",
					"rejected_text": part.Text,
				})

			} else if part.FunctionCall != nil && part.FunctionCall.Name != "" {
				if funcResult := c.executeCoderFunction(part.FunctionCall.Name, part.FunctionCall.Args); funcResult != nil {
					newResponse = append(newResponse, map[string]any{
						"function_name": part.FunctionCall.Name,
						"parameters":    part.FunctionCall.Args,
						"result":        funcResult,
					})
				} else {
					newResponse = append(newResponse, map[string]any{
						"function_name": part.FunctionCall.Name,
						"parameters":    part.FunctionCall.Args,
					})
				}
			}
		}
	}

	c.Metadata.LastActive = time.Now()

	// If no valid function calls, return error
	if len(newResponse) == 0 {
		return map[string]any{
			"error": "No function calls received. Coder must respond with function calls only.",
			"output": nil,
		}
	}

	return map[string]any{"error": nil, "output": newResponse}
}

// executeCoderFunction executes a coder capability based on the function name.
func (c *CoderAgent) executeCoderFunction(funcName string, args map[string]any) map[string]any {
	capabilities := GetCoderCapabilitiesMap()
	if capability, exists := capabilities[funcName]; exists {
		return capability.Service(args)
	}
	return nil
}

// ToMap serializes the CoderAgent into a map.
func (c *CoderAgent) ToMap() map[string]any {
	agent := repository.Agent(*c)
	return agent.ToMap()
}

// GetCoderMap creates a map representation of the coder agent for external use.
func GetCoderMap() map[string]any {
	var cagent CoderAgent
	agent := cagent.NewAgent()
	capsJSON, _ := json.Marshal(agent.Capabilities)
	var caps []map[string]any
	json.Unmarshal(capsJSON, &caps)
	return map[string]any{
		"agent":        coderName,
		"metadata":     agent.Metadata,
		"capabilities": caps,
	}
}

// Coder capabilities definitions.
const (
	ExecuteCodeName      = "execute_code"
	CreateFileFromName   = "create_file"
	ReadFileFromName     = "read_file"
	ListDirectoryName    = "list_directory"
	AnalyzeCodeName      = "analyze_code"
	DebugCodeName        = "debug_code"
	RefactorCodeName     = "refactor_code"
	GenerateTestsName    = "generate_tests"
	CreateDirectoryName  = "create_directory"
	DeleteFileOrDirName  = "delete_file_or_dir"
	EditFileFromName     = "edit_file"
	RequestUserInputName = "request_user_input"
)

const ExecuteCodeDescription = "Executes a given code snippet in a specified language and returns the output or error."
const CreateFileDescription = "Creates a new file at a specified path with the given content. Overwrites if the file exists."
const ReadFileDescription = "Reads the content of a file from a specified path."
const ListDirectoryDescription = "Lists all files and subdirectories in a given directory path."
const AnalyzeCodeDescription = "Analyzes a code snippet to identify potential issues, best practices, or provide an explanation."
const DebugCodeDescription = "Identifies and fixes bugs in a given code snippet or file."
const RefactorCodeDescription = "Refactors a code snippet to improve its structure and readability without changing its functionality."
const GenerateTestsDescription = "Generates unit tests for a given function or code block."
const CreateDirectoryDescription = "Creates a new directory at the specified path."
const DeleteFileOrDirDescription = "Deletes a file or directory at the specified path."
const EditFileDescription = "Edits a file at a given path by applying a specified change or replacement to its content."
const RequestUserInputDescription = "Prompts the user for a single line of input from the terminal and returns the response."

var CoderCapabilities = []repository.Function{
	{
		Name:        ExecuteCodeName,
		Description: ExecuteCodeDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"code": {
					Type:        "string",
					Description: "The code snippet to execute.",
				},
				"language": {
					Type:        "string",
					Description: "The programming language of the code.",
					Enum:        []string{"python", "go", "javascript", "bash", "java", "c"},
				},
				"dependencies": {
					Type:        "array",
					Description: "A list of dependencies to install before running the code.",
					Items:       &repository.Properties{Type: "string"},
				},
				"filename": {
					Type:        "string",
					Description: "The name of the file to save the code to before execution.",
				},
			},
			Required: []string{"code", "language"},
		},
		Service: ExecuteCode,
	},
	{
		Name:        CreateFileFromName,
		Description: CreateFileDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"path": {
					Type:        "string",
					Description: "The path of the file to create.",
				},
				"content": {
					Type:        "string",
					Description: "The content to write to the file.",
				},
			},
			Required: []string{"path", "content"},
		},
		Service: CreateFile,
	},
	{
		Name:        ReadFileFromName,
		Description: ReadFileDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"path": {
					Type:        "string",
					Description: "The path of the file to read.",
				},
			},
			Required: []string{"path"},
		},
		Service: ReadFile,
	},
	{
		Name:        ListDirectoryName,
		Description: ListDirectoryDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"path": {
					Type:        "string",
					Description: "The path of the directory to list.",
				},
			},
			Required: []string{"path"},
		},
		Service: ListDirectory,
	},
	{
		Name:        AnalyzeCodeName,
		Description: AnalyzeCodeDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"code": {
					Type:        "string",
					Description: "The code snippet to analyze.",
				},
				"language": {
					Type:        "string",
					Description: "The language of the code.",
				},
			},
			Required: []string{"code", "language"},
		},
		Service: AnalyzeCode,
	},
	{
		Name:        DebugCodeName,
		Description: DebugCodeDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"code": {
					Type:        "string",
					Description: "The code snippet to debug.",
				},
				"language": {
					Type:        "string",
					Description: "The language of the code.",
				},
				"error_message": {
					Type:        "string",
					Description: "The error message received from execution.",
				},
			},
			Required: []string{"code", "language"},
		},
		Service: DebugCode,
	},
	{
		Name:        RefactorCodeName,
		Description: RefactorCodeDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"code": {
					Type:        "string",
					Description: "The code snippet to refactor.",
				},
				"reason": {
					Type:        "string",
					Description: "The reason for refactoring.",
				},
			},
			Required: []string{"code", "reason"},
		},
		Service: RefactorCode,
	},
	{
		Name:        GenerateTestsName,
		Description: GenerateTestsDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"code": {
					Type:        "string",
					Description: "The code snippet for which to generate tests.",
				},
				"language": {
					Type:        "string",
					Description: "The language of the code.",
				},
			},
			Required: []string{"code", "language"},
		},
		Service: GenerateTests,
	},
	{
		Name:        CreateDirectoryName,
		Description: CreateDirectoryDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"path": {
					Type:        "string",
					Description: "The path of the directory to create.",
				},
			},
			Required: []string{"path"},
		},
		Service: CreateDirectory,
	},
	{
		Name:        DeleteFileOrDirName,
		Description: DeleteFileOrDirDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"path": {
					Type:        "string",
					Description: "The path of the file or directory to delete.",
				},
			},
			Required: []string{"path"},
		},
		Service: DeleteFileOrDir,
	},
	{
		Name:        EditFileFromName,
		Description: EditFileDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"path": {
					Type:        "string",
					Description: "The path of the file to edit.",
				},
				"changes": {
					Type:        "string",
					Description: "The content to replace in the file, or a full new content.",
				},
			},
			Required: []string{"path", "changes"},
		},
		Service: EditFile,
	},
	{
		Name:        RequestUserInputName,
		Description: RequestUserInputDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"prompt": {
					Type:        "string",
					Description: "The message to display to the user when requesting input.",
				},
			},
			Required: []string{"prompt"},
		},
		Service: func(data map[string]any) map[string]any {
			return nil
		},
	},
}

// GetCoderCapabilities returns the list of all capabilities for the CoderAgent.
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

// Helper function to extract a code block and its language from a string.
func extractCodeBlock(text string) (string, string, bool) {
	re := regexp.MustCompile("(?s)```([a-zA-Z0-9]+)\\n(.*?)```")
	matches := re.FindStringSubmatch(text)
	if len(matches) == 3 {
		return strings.TrimSpace(matches[2]), strings.TrimSpace(matches[1]), true
	}
	return "", "", false
}

// Helper function to get a file extension from a language name.
func getFileExtension(language string) string {
	switch strings.ToLower(language) {
	case "python":
		return "py"
	case "go":
		return "go"
	case "javascript":
		return "js"
	case "c":
		return "c"
	case "bash":
		return "sh"
	case "java":
		return "java"
	default:
		return "txt"
	}
}

// ExecuteCode executes a given code snippet in a temporary file and returns the output.
func ExecuteCode(data map[string]any) map[string]any {
	code, ok := data["code"].(string)
	if !ok {
		return map[string]any{"error": "code is required"}
	}
	language, ok := data["language"].(string)
	if !ok {
		return map[string]any{"error": "language is required"}
	}

	filename, ok := data["filename"].(string)
	if !ok || filename == "" {
		tempFile, err := os.CreateTemp("", "dost-code-*.py")
		if err != nil {
			return map[string]any{"error": fmt.Sprintf("failed to create temporary file: %v", err)}
		}
		filename = tempFile.Name()
		tempFile.Close()
	}

	if err := os.WriteFile(filename, []byte(code), 0644); err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to write code to file: %v", err)}
	}

	defer os.Remove(filename)

	fmt.Printf("Executing code in %s from file %s:\n", language, filename)

	var cmd *exec.Cmd
	switch language {
	case "python":
		cmd = exec.Command("python", filename)
	case "go":
		cmd = exec.Command("go", "run", filename)
	case "javascript":
		cmd = exec.Command("node", filename)
	case "bash":
		cmd = exec.Command("bash", filename)
	case "c":
		// For C, we need to compile and then run
		execFile := filename[:len(filename)-len(filepath.Ext(filename))]
		compileCmd := exec.Command("gcc", filename, "-o", execFile, "-lm")
		var compileErr bytes.Buffer
		compileCmd.Stderr = &compileErr
		if err := compileCmd.Run(); err != nil {
			os.Remove(execFile)
			return map[string]any{
				"status": "failed",
				"error":  fmt.Sprintf("compilation failed: %v", compileErr.String()),
			}
		}
		defer os.Remove(execFile)
		cmd = exec.Command(execFile)
	case "java":
		return map[string]any{"error": "Java execution is not yet implemented"}
	default:
		return map[string]any{"error": fmt.Sprintf("unsupported language: %s", language)}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return map[string]any{
			"status": "failed",
			"error":  fmt.Sprintf("execution failed: %v", stderr.String()),
		}
	}

	return map[string]any{
		"status": "completed",
		"output": stdout.String(),
	}
}

// CreateFile creates a new file at a specified path with the given content.
func CreateFile(data map[string]any) map[string]any {
	path, ok := data["path"].(string)
	if !ok {
		return map[string]any{"error": "path is required"}
	}
	content, ok := data["content"].(string)
	if !ok {
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
		"status":  "completed",
		"message": fmt.Sprintf("File created successfully at %s", path),
	}
}

// ReadFile reads the content of a file from a specified path.
func ReadFile(data map[string]any) map[string]any {
	path, ok := data["path"].(string)
	if !ok {
		return map[string]any{"error": "path is required"}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to read file: %v", err)}
	}

	return map[string]any{
		"status":  "completed",
		"content": string(content),
	}
}

// ListDirectory lists all files and subdirectories in a given directory path.
func ListDirectory(data map[string]any) map[string]any {
	path, ok := data["path"].(string)
	if !ok {
		return map[string]any{"error": "path is required"}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to list directory: %v", err)}
	}

	var fileNames []string
	for _, entry := range entries {
		fileNames = append(fileNames, entry.Name())
	}

	return map[string]any{
		"status":  "completed",
		"entries": fileNames,
	}
}

// AnalyzeCode analyzes a code snippet to identify potential issues, best practices, or provide an explanation.
func AnalyzeCode(data map[string]any) map[string]any {
	_, ok := data["code"].(string)
	if !ok {
		return map[string]any{"error": "code is required"}
	}
	language, ok := data["language"].(string)
	if !ok {
		return map[string]any{"error": "language is required"}
	}

	analysisResult := fmt.Sprintf("Analysis for %s code completed. This is a placeholder for a detailed analysis.", language)

	return map[string]any{
		"status": "completed",
		"output": analysisResult,
	}
}

// DebugCode identifies and fixes bugs in a given code snippet or file.
func DebugCode(data map[string]any) map[string]any {
	_, ok := data["code"].(string)
	if !ok {
		return map[string]any{"error": "code is required"}
	}
	language, ok := data["language"].(string)
	if !ok {
		return map[string]any{"error": "language is required"}
	}
	errorMessage, _ := data["error_message"].(string)

	debugResult := fmt.Sprintf("Debugging complete for %s code. Identified a potential fix related to: %s. This is a placeholder.", language, errorMessage)

	return map[string]any{
		"status": "completed",
		"output": debugResult,
	}
}

// RefactorCode refactors a code snippet to improve its structure and readability without changing its functionality.
func RefactorCode(data map[string]any) map[string]any {
	_, ok := data["code"].(string)
	if !ok {
		return map[string]any{"error": "code is required"}
	}
	reason, _ := data["reason"].(string)

	refactoredCode := fmt.Sprintf("Refactored code snippet based on the reason: %s. This is a placeholder.", reason)

	return map[string]any{
		"status": "completed",
		"output": refactoredCode,
	}
}

// GenerateTests generates unit tests for a given function or code block.
func GenerateTests(data map[string]any) map[string]any {
	_, ok := data["code"].(string)
	if !ok {
		return map[string]any{"error": "code is required"}
	}
	language, ok := data["language"].(string)
	if !ok {
		return map[string]any{"error": "language is required"}
	}

	tests := fmt.Sprintf("Test code generated for %s. This is a placeholder.", language)

	return map[string]any{
		"status": "completed",
		"output": tests,
	}
}

// CreateDirectory creates a new directory at the specified path.
func CreateDirectory(data map[string]any) map[string]any {
	path, ok := data["path"].(string)
	if !ok {
		return map[string]any{"error": "path is required"}
	}

	err := os.MkdirAll(path, 0755)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to create directory: %v", err)}
	}

	return map[string]any{
		"status":  "completed",
		"message": fmt.Sprintf("Directory created successfully at %s", path),
	}
}

// DeleteFileOrDir deletes a file or directory at the specified path.
func DeleteFileOrDir(data map[string]any) map[string]any {
	path, ok := data["path"].(string)
	if !ok {
		return map[string]any{"error": "path is required"}
	}

	err := os.RemoveAll(path)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to delete: %v", err)}
	}

	return map[string]any{
		"status":  "completed",
		"message": fmt.Sprintf("Deleted successfully at %s", path),
	}
}

// EditFile edits a file at a given path.
func EditFile(data map[string]any) map[string]any {
	path, ok := data["path"].(string)
	if !ok {
		return map[string]any{"error": "path is required"}
	}
	changes, ok := data["changes"].(string)
	if !ok {
		return map[string]any{"error": "changes are required"}
	}

	originalContent, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to read file for editing: %v", err)}
	}

	newContent := strings.Replace(string(originalContent), "placeholder", changes, -1)

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to write edited content to file: %v", err)}
	}

	return map[string]any{
		"status":  "completed",
		"message": fmt.Sprintf("File edited successfully at %s", path),
	}
}
