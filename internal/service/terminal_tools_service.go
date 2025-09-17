package service

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
	"runtime"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	gitignore "github.com/sabhiram/go-gitignore"
)

var TakePermission = false
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

/*
args : {
command : cd | python3
arguments : <folder_name> | main.py
}
*/
func ExecuteCommands(args map[string]any) map[string]any {
	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	cmdStr, ok := args["command"].(string)
	if !ok {
		return map[string]any{"error": "Invalid command"}
	}

	// Extract arguments
	argStr := ""
	if val, ok := args["arguments"].(string); ok && val != "" {
		argStr = val
	}

	fullCmd := cmdStr
	if argStr != "" {
		fullCmd += " " + argStr
	}

	// Special handling for "cd"
	if cmdStr == "cd" {
		newDir := argStr
		if newDir == "" {
			return map[string]any{"error": "cd requires a path"}
		}
		if !filepath.IsAbs(newDir) {
			newDir = filepath.Join(wd, newDir)
		}
		err := os.Chdir(newDir)
		if err != nil {
			return map[string]any{"error": fmt.Sprintf("failed to change directory: %v", err)}
		}
		return map[string]any{"message": fmt.Sprintf("Changed directory to %s", newDir)}
	}

	// Windows path fix for relative paths
	if runtime.GOOS == "windows" && (strings.HasPrefix(fullCmd, "./") || strings.HasPrefix(fullCmd, ".\\")) {
		fullCmd = fullCmd[2:]
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	// Prepare command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", fullCmd)
	} else {
		cmd = exec.Command("sh", "-c", fullCmd)
	}

	// Run in current directory of main process
	cmd.Dir = wd
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	log.Default().Printf("Running command in %s: %s\n", wd, fullCmd)

	err = cmd.Run()
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("command failed: [%v] %v || CONSOLE/TERMINAL:%v", fullCmd, err, stderrBuf.String())}
	}

	return map[string]any{"message": "Command executed successfully", "output": stdoutBuf.String()}
}

// GetProjectStructure returns the project structure as a string, ignoring files and directories specified in .gitignore.
// If a .gitignore file is not found, it uses a default ignore list.
// It takes the project path as input.
func GetProjectStructure(args map[string]any) map[string]any {
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

func CreateFile(args map[string]any) map[string]any {
	/*
		args : {
		file_names : ["test.txt"]
		contents : ["hello world"]
		}
	*/

	file_names := args["file_names"]
	contents := args["contents"]
	if contents == nil || file_names == nil {
		return map[string]any{"error": "Invalid arguments: file_names and contents required", "output": nil}
	}

	// Convert file_names to []interface{}
	fileNamesSlice, ok := file_names.([]interface{})
	if !ok {
		return map[string]any{"error": "Invalid file_names: must be array", "output": nil}
	}

	// Convert contents to []interface{}
	contentsSlice, ok := contents.([]interface{})
	if !ok {
		return map[string]any{"error": "Invalid contents: must be array", "output": nil}
	}

	// Check lengths match
	if len(fileNamesSlice) != len(contentsSlice) {
		return map[string]any{"error": "file_names and contents must have same length", "output": nil}
	}

	var errors []error
	var createdFiles []string

	for i, v := range fileNamesSlice {
		file_name, ok := v.(string)
		if !ok {
			errors = append(errors, fmt.Errorf("file_name at index %d is not a string", i))
			continue
		}

		content, ok := contentsSlice[i].(string)
		if !ok {
			errors = append(errors, fmt.Errorf("content at index %d is not a string", i))
			continue
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(file_name)
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				errors = append(errors, fmt.Errorf("failed to create directory %s: %v", dir, err))
				continue
			}
		}

		err := os.WriteFile(file_name, []byte(content), 0644)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to create %s: %v", file_name, err))
			continue
		}
		fmt.Printf("Created file: %s\n", file_name)
		createdFiles = append(createdFiles, file_name)
	}

	if len(errors) > 0 {
		return map[string]any{
			"error":  fmt.Sprintf("Errors: %v", errors),
			"output": fmt.Sprintf("Partially completed. Created files: %v", createdFiles),
		}
	}

	return map[string]any{
		"error":  nil,
		"output": fmt.Sprintf("Files created successfully: %v", createdFiles),
	}
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
		content := contentsSlice[i].(string)

		// Determine the offset for the current file
		var offset int64
		if hasOffsets {
			offset = int64(offsetsSlice[i].(float64)) // JSON numbers are float64 in Go
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

func ReadFiles(args map[string]any) map[string]any {
	/*
		args : {
		file_names : ["test.txt"]<slice of file_names>
		}
	*/
	fileNames, ok := args["file_names"].([]interface{})
	if !ok {
		return map[string]any{"error": "Invalid arguments: 'file_names' must be a slice of strings", "output": nil}
	}

	var stringFileNames []string
	for _, v := range fileNames {
		s, ok := v.(string)
		if !ok {
			return map[string]any{"error": "Invalid argument: 'file_names' contains non-string values", "output": nil}
		}
		stringFileNames = append(stringFileNames, s)
	}

	ReadFiles := make(map[string]any)
	var notFoundFiles []string

	for _, fileName := range stringFileNames {
		file, err := os.Open(fileName)
		if err != nil {
			notFoundFiles = append(notFoundFiles, fileName)
			continue
		}

		data, err := io.ReadAll(file)
		file.Close()

		if err != nil {
			ReadFiles[fileName] = fmt.Sprintf("Error reading file: %v", err)
			continue
		}

		stringData := string(data)
		ReadFiles[fileName] = stringData
		fmt.Printf("Read file: %s (%d bytes)\n", fileName, len(data))
	}

	if len(notFoundFiles) > 0 {
		fmt.Printf("Files not found: %v\n", notFoundFiles)
	}

	return map[string]any{"error": nil, "output": ReadFiles}
}

// InitializeGitRepo initializes a new git repository in the current directory.
func InitializeGitRepo(map[string]any) map[string]any {
	err := exec.Command("git", "init").Run()
	if err != nil {
		return map[string]any{"error": err, "output": nil}
	}
	return map[string]any{"error": nil, "output": "Git repository initialized successfully"}
}

// Take Input From user for any process
func TakeInputFromTerminal(map[string]any) map[string]any {
	fmt.Print("dost> ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("Error reading input: %v", err), "output": nil}
	}
	// Trim whitespace and newlines
	input = strings.TrimSpace(input)
	if input == "" {
		return map[string]any{"error": nil, "output": "No input provided"}
	}
	return map[string]any{"error": nil, "output": input}
}

// RequestTool makes requests to the Gemini API with conversation history support
func RequestTool(body map[string]any) map[string]any {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Start()
	defer s.Stop()

	API_KEY := body["API_KEY"].(string)
	delete(body, "API_KEY")

	// Ensure the request has the proper structure for conversation history
	if systemInstruction, exists := body["systemInstruction"]; exists {
		// Keep the system instruction as is
		body["systemInstruction"] = systemInstruction
	}

	if contents, exists := body["contents"]; exists {
		// Keep the contents (conversation history) as is
		body["contents"] = contents
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(
		context.Background(),
		"POST",
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
		bytes.NewBuffer(jsonData),
	)

	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-goog-api-key", API_KEY)

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var response map[string]any

		if err = json.Unmarshal(bodyBytes, &response); err != nil {
			return map[string]any{"error": fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)), "output": nil}
		}

		return map[string]any{
			"error": map[string]any{
				"status":      resp.Status,
				"status_code": resp.StatusCode,
				"body":        response,
			},
			"output": nil,
		}
	}

	// Parse the response
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
				newResponse = append(newResponse, map[string]any{"text": part.Text})
			} else if part.FunctionCall.Name != "" {
				newResponse = append(newResponse, map[string]any{
					"text":          "",
					"function_name": part.FunctionCall.Name,
					"parameters":    part.FunctionCall.Args,
				})
			}
		}
	}

	return map[string]any{"error": nil, "output": newResponse}
}

func InitialContext(body map[string]any) map[string]any {
	candidates := []string{
		"main.go", "go.mod",
		"package.json",
		"requirements.txt", "pyproject.toml", "setup.py",
		"pubspec.yaml",
		"pom.xml", "build.gradle",
		"Cargo.toml",
		"composer.json",
		"Makefile", ".env.example",
		"CMakeLists.txt", "Dockerfile",
	}

	var found []string
	for _, file := range candidates {
		path := filepath.Join(".", file)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
		}
	}

	initialFile := ReadFiles(map[string]any{"file_names": found})
	env := os.Getenv("PATH")
	projectStruct := GetProjectStructure(map[string]any{"path": "."})
	operating_system := runtime.GOOS

	return map[string]any{
		"error": nil,
		"output": map[string]any{
			"PATHS":             env,
			"project_structure": projectStruct["output"],
			"initial_files":     initialFile["output"],
			"operating_system":  operating_system,
		},
	}
}

func ExitProcess(map[string]any) map[string]any {
	fmt.Println("🏁 Task completed successfully! Exiting...")
	os.Exit(0)
	return map[string]any{"error": nil, "output": "Process exited successfully"}
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
