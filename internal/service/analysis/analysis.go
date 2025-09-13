package analysis

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
	QueryInputs     map[string]any `json:"query_input`
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

// args must and only contains "query"
func (p *AgentAnalysis) Interaction(args map[string]any) map[string]any {
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
				if function, exists := AnalysistoolsFunc[name]; exists {
					result := function.Run(argsData)
					if _, ok = result["exit"].(bool); ok {

						analysisID := result["output"].(string)
						analysis := AnalysisMap[analysisID]
						analysis.QueryInputs = InputData
						AnalysisMap[analysisID] = analysis
						return map[string]any{"analysis-id": result["output"]}
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
				"mode": "ANY", // Changed from "FORCE_CALLS" string to proper object
			},
		},
		"contents": contents,
		"tools": []map[string]any{
			{"functionDeclarations": GetAnalysisToolsMap()}, // Also fixed snake_case to camelCase
		},
	}

	// Marshal request body
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

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
	req.Header.Set("X-goog-api-key", viper.GetString("PLANNER.API_KEY"))

	// Execute request with timeout
	client := &http.Client{Timeout: p.Metadata.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return map[string]any{
			"error":  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)),
			"output": nil,
		}
	}

	// Parse response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

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
		Description: `When you feel that the task is completed always call this to exit the process.
		Before calling this function make sure,
		You have completed all the task.
		You have fixed all the bugs.
		User is satisfied with the output.
		`,
		Parameters: repository.Parameters{
			Type: repository.TypeObject,
			Properties: map[string]*repository.Properties{
				"text": {
					Type:        repository.TypeString,
					Description: "A text which you want to say to user, instead of returning text output give it in this parameter",
				},
				"analysis-id": {
					Type:        repository.TypeString,
					Description: "Id of the last completed analysis",
				},
			},
			Required: []string{"text", "analysis-id"},
		},
		Service: ExitProcess,
		Return: repository.Return{
			"error":  "string",
			"output": "string",
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
