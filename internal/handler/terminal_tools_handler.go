package handler

import (
	"dost/internal/repository"
	"dost/internal/service"
)

const (
	TERMINAL_EXECUTE         = "terminal_execute"
	GET_PROJECT_STRUCTURE    = "get_project_structure"
	CREATE_FILES             = "create_files"
	WRITE_FILE               = "write_file"
	READ_FILES               = "read_files"
	INITIALIZE_GIT_REPO      = "initalize_git_repo"
	TAKE_INPUT_FROM_TERMINAL = "take_input_from_user"
	REQUEST_AI_TOOL          = "request_ai_tool"
	EXIT_PROCESS             = "exit_process"
	FIND_ERRORS              = "find_errors"
)
const (
	// TypeUnspecified means not specified, should not be used.
	TypeUnspecified = "0"
	// TypeString means openAPI string type
	TypeString = "1"
	// TypeNumber means openAPI number type
	TypeNumber = "2"
	// TypeInteger means openAPI integer type
	TypeInteger = "3"
	// TypeBoolean means openAPI boolean type
	TypeBoolean = "4"
	// TypeArray means openAPI array type
	TypeArray = "5"
	// TypeObject means openAPI object type
	TypeObject = "6"
)

var tools []repository.Function = []repository.Function{
	{
		Name: TERMINAL_EXECUTE,
		Description: `Run any valid shell/terminal command in the current working directory. Use for file operations (create, delete, list, copy files, move files, rename files, git functions, etc), navigation (cd, ls), installing packages, running build tools, executing programs, checking versions, and inspecting system state. Always provide the command and optional arguments separately
`,
		Parameters: repository.Parameters{
			Type: TypeObject,
			Properties: map[string]*repository.Properties{
				"command": {
					Type:        TypeString,
					Description: "Command to execute (e.g., go run main.go, npm install, python main.py)",
				},
				"arguments": {
					Type:        TypeString,
					Description: "Optional arguments for the command",
				},
			},
			Required: []string{"command"},
			Optional: []string{"arguments"},
		},
		Service: service.ExecuteCommands,
		Return: repository.Return{
			"error":  "string",
			"output": "string",
		},
	},
	{
		Name: GET_PROJECT_STRUCTURE,
		Description: `Returns the full directory structure of the project at the given path.
Call this before working on unfamiliar projects to understand where files are located.
Respects .gitignore to skip irrelevant files.`,
		Parameters: repository.Parameters{
			Type: TypeObject,
			Properties: map[string]*repository.Properties{
				"path": {
					Type:        TypeString,
					Description: "Path to the project directory (usually '.')",
				},
			},
			Required: []string{"path"},
		},
		Service: service.GetProjectStructure,
		Return: repository.Return{
			"error":  "string",
			"output": "string",
		},
	},
	{
		Name: CREATE_FILES,
		Description: `
Creates new files with the given names and contents.
Make sure that the file names and the file contents are in the same order.
Call this when the files do not exist and you need to create them.
If the files might already exist, first call 'read_files' to check.

`,
		Parameters: repository.Parameters{
			Type: TypeObject,
			Properties: map[string]*repository.Properties{
				"file_names": {
					Type:        TypeArray,
					Items:       &repository.Properties{Type: TypeString},
					Description: "Name of the file to create (with extension)",
				},
				"contents": {
					Type:        TypeArray,
					Items:       &repository.Properties{Type: TypeString},
					Description: "Full content to write in the new file",
				},
			},
			Required: []string{"file_names", "contents"},
		},
		Service: service.CreateFile,
		Return:  repository.Return{"error": "string", "output": "string"},
	},
	{
		Name: WRITE_FILE,
		Description: `Overwrites the entire content of an existing file.
		It opens the file in append mode.
	Always call 'read_files' first to retrieve existing content before making modifications.`,
		Parameters: repository.Parameters{
			Type: TypeObject,
			Properties: map[string]*repository.Properties{
				"file_names": {
					Type:        TypeArray,
					Items:       &repository.Properties{Type: TypeString},
					Description: "Name of the file to create (with extension)",
				},
				"contents": {
					Type:        TypeArray,
					Items:       &repository.Properties{Type: TypeString},
					Description: "Full content to write in the new file",
				},
				"offsets": {
					Type:  TypeArray,
					Items: &repository.Properties{Type: TypeNumber},
					Description: `The number represents a byte offset from the beginning of the file. By setting this value, you can:
If the offsets parameter is not provided, the function will default to overwriting the entire file.
Append Content: If you set the offset to the current size of the file, you can append new content to the end.

Overwrite Content: If you set the offset to a position within the existing file, any new content will overwrite the old content starting from that point.

Create Sparse Files: If you set the offset to a position beyond the current end of the file, the operating system will create a "sparse file" by inserting zero bytes as padding until the specified offset is reached.`,
				},
			},
			Required: []string{"file_names", "contents", "offsets"},
		},
		Service: service.WriteFile,
		Return:  repository.Return{"error": "string", "output": "string"},
	},
	{
		Name: READ_FILES,
		Description: `Reads the content of one or more files and returns them.
Call this whenever you need to:
- Check existing code before modifying it
- Inspect dependencies or configuration
- Validate if a file exists
`,
		Parameters: repository.Parameters{
			Type: TypeObject,
			Properties: map[string]*repository.Properties{
				"file_names": {
					Type: TypeArray,
					Items: &repository.Properties{
						Type:        TypeString,
						Description: "Name of the file to read",
					},
					Description: "List/Slice of file names to read",
				},
			},
			Required: []string{"file_names"},
		},
		Service: service.ReadFiles,
		Return:  repository.Return{"error": "string", "output": "map[string]any"},
	},
	{
		Name: INITIALIZE_GIT_REPO,
		Description: `Initializes a new Git repository in the project directory.
Call this if you detect that Git is not already initialized and version control is needed.`,
		Parameters: repository.Parameters{
			Type: TypeObject,
		},
		Service: service.InitializeGitRepo,
		Return:  repository.Return{"error": "string", "output": "string"},
	},
	{
		Name: TAKE_INPUT_FROM_TERMINAL,
		Description: `Prompts the user in the terminal for manual input.
Use this whenever you are missing essential details, such as:
- File names
- Function parameters
- Configuration values
- User choices`,
		Parameters: repository.Parameters{
			Type: TypeObject,
		},
		Service: service.TakeInputFromTerminal,
		Return:  repository.Return{"error": "string", "output": "string"},
	},
	{
		Name: REQUEST_AI_TOOL,
		Description: `Sends a follow-up request to yourself with the latest context and last 5 conversation turns.
Use this when:
- You have gathered additional information or file content
- You need to continue the reasoning process without losing state
NEVER set index to -1 when using this tool.`,
		Parameters: repository.Parameters{
			Type: TypeObject,
			Properties: map[string]*repository.Properties{
				"query": {
					Type:        TypeString,
					Description: "Follow-up query or instruction to process with the new context",
				},
			},
			Required: []string{"query"},
		},
		Service: service.RequestTool,
		Return:  repository.Return{"error": "string", "output": "object"},
	},
	{
		Name: EXIT_PROCESS,
		Description: `When you feel that the task is completed always call this to exit the process.
		Before calling this function make sure,
		You have completed all the task.
		You have fixed all the bugs.
		User is satisfied with the output.
		`,
		Parameters: repository.Parameters{
			Type:       TypeObject,
			Properties: map[string]*repository.Properties{},
			Required:   []string{},
		},

		Service: service.ExitProcess,
	},
	{
		Name: FIND_ERRORS,
		Description: `
		This tool helps the AI to find Errors in the codebase , by running the compiler/interpreter of the respective programming language.
		The AI can use this tool to quickly identify syntax errors, runtime errors, and other issues in the code.
		A comprehensive error report will be generated, highlighting the exact location and nature of each error.
		This should always be the first step in debugging any code-related issue.
		`,

		Parameters: repository.Parameters{
			Type: TypeObject,
			Properties: map[string]*repository.Properties{
				"command": {
					Type:        TypeString,
					Description: "Command to execute (e.g., go run main.go, npm install, python main.py)",
				},
				"arguments": {
					Type:        TypeString,
					Description: "Required arguments for command, to find the error files",
				},
			},
		},
		Service: service.ExecuteCommands,
	},
}

var toolsMap map[string]repository.Function = make(map[string]repository.Function)

func init() {
	for _, v := range tools {
		toolsMap[v.Name] = v
	}
}
func GeminiTools() map[string]repository.Function {
	return toolsMap
}

func GetTerminalTools() []repository.Function {

	return tools
}
func GetTerminalToolsMap() []map[string]any {
	arrayOfMap := make([]map[string]any, 0)
	for _, v := range tools {
		arrayOfMap = append(arrayOfMap, v.ToObject())
	}
	return arrayOfMap
}

// func GetTerminalToolsMap2() map[string]any {
// 	toolMap := make(map[string]any)
// 	for _, v := range tools {
// 		toolMap[v.Name] = v.ToObject()
// 	}
// 	return toolMap
// }
