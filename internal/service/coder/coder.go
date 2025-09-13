package coder

import (
	"bufio"
	"bytes"
	"context"
	"dost/internal/repository"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
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
					fmt.Println("CODERAGENT", result)
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

	request := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]any{
				{"text": c.Metadata.Instructions},
			},
		},
		"toolConfig": map[string]any{
			"functionCallingConfig": map[string]any{
				"mode": "ANY", // Changed from "FORCE_CALLS" string to proper object
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

	fmt.Println("HELLO 21")

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

	// Extract both text and function calls (like analysis agent)
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

	// Add the model's response to chat history (like analysis agent)
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

		scanner := bufio.NewScanner(file)
		chunks := []map[string]any{}

		var builder strings.Builder
		lineCount := 0
		chunkSize := 40 // number of lines per chunk

		for scanner.Scan() {
			line := scanner.Text()
			builder.WriteString(line)
			builder.WriteString("\n")
			lineCount++

			if lineCount%chunkSize == 0 {
				chunks = append(chunks, map[string]any{
					"start":   lineCount - chunkSize + 1,
					"end":     lineCount,
					"content": builder.String(),
				})
				builder.Reset()
			} else {
				// snapshot for progressive reading
				chunks = append(chunks, map[string]any{
					"start":   lineCount - (lineCount % chunkSize) + 1,
					"end":     lineCount,
					"content": builder.String(),
				})
			}
		}

		if err := scanner.Err(); err != nil {
			readFiles[fileName] = fmt.Sprintf("Error reading file: %v", err)
			continue
		}

		readFiles[fileName] = chunks
		fmt.Printf("Read file: %s (%d chunks)\n", fileName, len(chunks))
	}

	if len(notFoundFiles) > 0 {
		fmt.Printf("Files not found: %v\n", notFoundFiles)
	}

	return map[string]any{"error": nil, "output": readFiles}
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

// DeleteFileOrDir deletes a file or directory at the specified path.
func DeleteFileOrDir(data map[string]any) map[string]any {
	text, ok := data["text"].(string)
	if ok {
		fmt.Printf("CODER: %s", text)
	}
	path, ok := data["path"].(string)
	if !ok {
		return map[string]any{"error": "path is required"}
	}

	err := os.RemoveAll(path)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to delete: %v", err)}
	}

	return map[string]any{
		"status": "completed",
		"output": fmt.Sprintf("Deleted successfully at %s", path),
	}
}

// EditFile edits a file at a given path.
// Piece Table - what is it ??

func EditFile(args map[string]any) map[string]any {
	text, ok := args["text"].(string)
	if ok {
		fmt.Printf("CODER: %s", text)
	}
	filepath, ok := args["file_path"].(string)
	if !ok {
		return map[string]any{"error": "ERROR READING PATH"}
	}
	changes, ok := args["changes"].([]map[string]any)
	if !ok {
		return map[string]any{"error": "REQUIRED CHANGES MAP"}
	}
	// Step 1: Load file
	data, err := os.ReadFile(filepath)
	if err != nil {
		return map[string]any{"error": "ERROR READING FILE"}
	}
	pt := NewPieceTable(string(data))

	// Step 2: Convert line+col into absolute position
	lines := strings.Split(string(data), "\n")

	posFromLineCol := func(line, col int) int {
		pos := 0
		for i := 0; i < line-1; i++ {
			pos += len(lines[i]) + 1 // +1 for newline
		}
		return pos + col
	}

	for _, ch := range changes {
		StartLine, ok := ch["start_line"].(int)
		if !ok {
			return map[string]any{"error": "ERROR READING StartLine"}
		}
		StartCol, ok := ch["start_col"].(int)
		if !ok {
			return map[string]any{"error": "ERROR READING StartCol"}
		}
		EndLine, ok := ch["end_line"].(int)
		if !ok {
			return map[string]any{"error": "ERROR READING EndLine"}
		}
		EndCol, ok := ch["end_col"].(int)
		if !ok {
			return map[string]any{"error": "ERROR READING EndCol"}
		}
		Operation, ok := ch["operation"].(string)
		if !ok {
			return map[string]any{"error": "ERROR READING Operation"}
		}
		Content, ok := ch["contents"].(string)
		if !ok {
			return map[string]any{"error": "ERROR READING Content"}
		}
		start := posFromLineCol(StartLine, StartCol)
		end := posFromLineCol(EndLine, EndCol)

		switch Operation {
		case "delete":
			pt.Delete(start, end-start)
		case "replace":
			pt.Delete(start, end-start)
			pt.Insert(start, Content)
		case "write":
			pt.Insert(start, Content)
		default:
			return map[string]any{"error": "UNKNOWN OPERATIONS"}
		}
	}

	// Step 3: Save back to file
	newContent := pt.String()
	err = os.WriteFile(filepath, []byte(newContent), 0644)
	if err != nil {
		return map[string]any{"error": "UNABLE TO WRITE THE DATA IN FILE"}
	} else {
		return map[string]any{"output": "Successfully Written The content you provided"}
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
		Name:        DeleteFileOrDirName,
		Description: DeleteFileOrDirDescription,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "Additional context or instructions provided by the AI for the edit operation.",
				},
				"path": {
					Type:        repository.TypeString,
					Description: "The path of the file or directory to delete.",
				},
			},
			Required: []string{"text", "path"},
		},
		Service: DeleteFileOrDir,
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
