package coder

// import (
// 	"dost/internal/repository"
// )

// // Capability constants
// const (
// 	GenerateCodeName = "generate_code"
// 	EditCodeName     = "edit_code"
// 	ExplainCodeName  = "explain_code"
// 	DebugCodeName    = "debug_code"
// 	RunCodeName      = "run_code"
// 	TestCodeName     = "test_code"
// 	RefactorCodeName = "refactor_code"
// 	DocumentCodeName = "document_code"
// )

// var CoderCapabilities = []repository.Function{
// 	{
// 		Name:        GenerateCodeName,
// 		Description: "Generate new code in a specified language based on natural language instructions.",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"language":    {Type: "string", Description: "Programming language to generate code in (e.g., go, python, js)."},
// 				"instruction": {Type: "string", Description: "High-level description of the code to generate."},
// 				"context":     {Type: "string", Description: "Optional project context or constraints."},
// 			},
// 			Required: []string{"language", "instruction"},
// 		},
// 		Service: GenerateCode,
// 	},
// 	{
// 		Name:        EditCodeName,
// 		Description: "Modify existing code based on given edit instructions.",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"language":    {Type: "string", Description: "Programming language of the code."},
// 				"code":        {Type: "string", Description: "The existing code to edit."},
// 				"instruction": {Type: "string", Description: "Edit instruction, describing what should change."},
// 			},
// 			Required: []string{"language", "code", "instruction"},
// 		},
// 		Service: EditCode,
// 	},
// 	{
// 		Name:        ExplainCodeName,
// 		Description: "Explain what a given piece of code does in natural language.",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"language":     {Type: "string", Description: "Programming language of the code."},
// 				"code":         {Type: "string", Description: "The code snippet to explain."},
// 				"detail_level": {Type: "string", Description: "Optional: summary, detailed, or line_by_line explanation."},
// 			},
// 			Required: []string{"language", "code"},
// 		},
// 		Service: ExplainCode,
// 	},
// 	{
// 		Name:        DebugCodeName,
// 		Description: "Analyze code for potential errors, bugs, or anti-patterns.",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"language": {Type: "string", Description: "Programming language of the code."},
// 				"code":     {Type: "string", Description: "The code snippet to debug."},
// 			},
// 			Required: []string{"language", "code"},
// 		},
// 		Service: DebugCode,
// 	},
// 	{
// 		Name:        RunCodeName,
// 		Description: "Execute code in a sandboxed environment and return the result (stdout, stderr, exit code).",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"language": {Type: "string", Description: "Programming language to run."},
// 				"code":     {Type: "string", Description: "Code to execute."},
// 				"inputs":   {Type: repository.TypeArray, Items: &repository.Properties{Type: "string"}, Description: "Optional inputs for the program."},
// 			},
// 			Required: []string{"language", "code"},
// 		},
// 		Service: RunCode,
// 	},
// 	{
// 		Name:        TestCodeName,
// 		Description: "Generate or run unit tests for the given code.",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"language":         {Type: "string", Description: "Programming language of the code."},
// 				"code":             {Type: "string", Description: "The code to test."},
// 				"test_instruction": {Type: "string", Description: "Optional: description of what to test."},
// 			},
// 			Required: []string{"language", "code"},
// 		},
// 		Service: TestCode,
// 	},
// 	{
// 		Name:        RefactorCodeName,
// 		Description: "Improve code readability, maintainability, or performance.",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"language":    {Type: "string", Description: "Programming language of the code."},
// 				"code":        {Type: "string", Description: "The code to refactor."},
// 				"style_guide": {Type: "string", Description: "Optional style guide to follow (e.g., PEP8, Go style)."},
// 			},
// 			Required: []string{"language", "code"},
// 		},
// 		Service: RefactorCode,
// 	},
// 	{
// 		Name:        DocumentCodeName,
// 		Description: "Generate documentation and docstrings for the provided code.",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"language": {Type: "string", Description: "Programming language of the code."},
// 				"code":     {Type: "string", Description: "The code snippet to document."},
// 				"format":   {Type: "string", Description: "Optional output format (markdown, html, docstring)."},
// 			},
// 			Required: []string{"language", "code"},
// 		},
// 		Service: DocumentCode,
// 	},
// }

// // Helpers for orchestrator
// func GetCoderCapabilities() []repository.Function {
// 	return CoderCapabilities
// }

// func GetCoderCapabilitiesArrayMap() []map[string]any {
// 	coderMap := make([]map[string]any, 0)
// 	for _, v := range CoderCapabilities {
// 		coderMap = append(coderMap, v.ToObject())
// 	}
// 	return coderMap
// }

// func GetCoderCapabilitiesMap() map[string]repository.Function {
// 	coderMap := make(map[string]repository.Function)
// 	for _, v := range CoderCapabilities {
// 		coderMap[v.Name] = v
// 	}
// 	return coderMap
// }
