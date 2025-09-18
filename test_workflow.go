package main

// import (
// 	"dost/internal/service/orchestrator"
// 	"fmt"
// 	"os"
// 	"path/filepath"
// 	"strings"
// )

// // Test the enhanced multi-agent workflow orchestrator
// func main() {
// 	fmt.Println("🧪 Testing DOST Multi-Agent Workflow Orchestrator")
// 	fmt.Println(strings.Repeat("=", 60))

// 	// Test cases for different types of requests
// 	testCases := []struct {
// 		name    string
// 		request string
// 		expect  []string // expected files to be created
// 	}{
// 		{
// 			name:    "REST API Request",
// 			request: "Build me a REST API in Go with JWT authentication",
// 			expect:  []string{"main.go", "go.mod", "server.go", "auth.go"},
// 		},
// 		{
// 			name:    "Web Application Request",
// 			request: "Create a simple web application with HTML, CSS, and JavaScript",
// 			expect:  []string{"index.html", "styles.css", "script.js"},
// 		},
// 		{
// 			name:    "Python Script Request",
// 			request: "Create utility scripts for data processing in Python",
// 			expect:  []string{"main.py", "utils.py", "README.md"},
// 		},
// 		{
// 			name:    "General Coding Request",
// 			request: "Implement a simple calculator program",
// 			expect:  []string{}, // Will be determined by coder
// 		},
// 	}

// 	for i, tc := range testCases {
// 		fmt.Printf("\n🧪 Test Case %d: %s\n", i+1, tc.name)
// 		fmt.Printf("📝 Request: %s\n", tc.request)

// 		// Process the request
// 		result := orchestrator.ProcessNaturalLanguageRequest(tc.request)

// 		// Validate results
// 		validateTestResult(tc, result)
// 	}

// 	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
// 	fmt.Println("🎯 All tests completed!")
// 	fmt.Println("📁 Check the './dost' directory for generated workflow files")
// }

// func validateTestResult(tc struct {
// 	name    string
// 	request string
// 	expect  []string
// }, result map[string]any) {
// 	status, _ := result["status"].(string)
// 	workflowID, _ := result["workflow_id"].(string)
// 	totalTasks, _ := result["total_tasks"].(int)
// 	successfulTasks, _ := result["successful_tasks"].(int)

// 	fmt.Printf("  📋 Workflow ID: %s\n", workflowID)
// 	fmt.Printf("  ✅ Status: %s\n", status)
// 	fmt.Printf("  📊 Tasks: %d total, %d successful\n", totalTasks, successfulTasks)

// 	// Check if workflow completed successfully
// 	if status == "completed" || status == "partial_success" {
// 		fmt.Printf("  ✅ Workflow completed successfully\n")
// 	} else {
// 		fmt.Printf("  ❌ Workflow failed with status: %s\n", status)
// 	}

// 	// Check created files
// 	if filesCreated, ok := result["files_created"].([]string); ok && len(filesCreated) > 0 {
// 		fmt.Printf("  📁 Files created: %d\n", len(filesCreated))
// 		for _, file := range filesCreated {
// 			fmt.Printf("    ✨ %s\n", file)
// 		}
// 	}

// 	// Verify expected files if specified
// 	if len(tc.expect) > 0 {
// 		fmt.Printf("  🔍 Checking expected files...\n")
// 		for _, expectedFile := range tc.expect {
// 			if fileExists(expectedFile) {
// 				fmt.Printf("    ✅ %s exists\n", expectedFile)
// 			} else {
// 				fmt.Printf("    ❌ %s missing\n", expectedFile)
// 			}
// 		}
// 	}

// 	// Check verification results
// 	if verification, ok := result["verification"].(map[string]any); ok {
// 		if verified, ok := verification["verified"].(bool); ok {
// 			if verified {
// 				fmt.Printf("  ✅ Verification passed\n")
// 			} else {
// 				fmt.Printf("  ⚠️ Verification found issues\n")
// 				if issues, ok := verification["issues"].([]string); ok {
// 					for _, issue := range issues {
// 						fmt.Printf("    ❗ %s\n", issue)
// 					}
// 				}
// 			}
// 		}
// 	}

// 	// Check output files
// 	outputDir := "./dost"
// 	expectedOutputFiles := []string{
// 		fmt.Sprintf("task_responses_%s.json", workflowID),
// 		fmt.Sprintf("workflow_%s.json", workflowID),
// 		fmt.Sprintf("workflow_%s.log", workflowID),
// 		"changes.log",
// 	}

// 	fmt.Printf("  📄 Output files:\n")
// 	for _, outFile := range expectedOutputFiles {
// 		fullPath := filepath.Join(outputDir, outFile)
// 		if fileExists(fullPath) {
// 			fmt.Printf("    ✅ %s\n", outFile)
// 		} else {
// 			fmt.Printf("    ❌ %s missing\n", outFile)
// 		}
// 	}
// }

// func fileExists(filePath string) bool {
// 	_, err := os.Stat(filePath)
// 	return err == nil
// }
