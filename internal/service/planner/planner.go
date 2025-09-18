package planner

import (
	"bytes"
	"context"
	"dost/internal/repository"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	gitignore "github.com/sabhiram/go-gitignore"

	"github.com/spf13/viper"
)

var ChatHistory = make([]map[string]any, 0)
var ignoreMatcher *gitignore.GitIgnore
var PlannertoolsFunc map[string]repository.Function = make(map[string]repository.Function)

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

type Plans struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Actions     []string `json:"actions"`
}

type AgentPlanner repository.Agent

func (p *AgentPlanner) Interaction(args map[string]any) map[string]any {

	ChatHistory = append(ChatHistory, []map[string]any{

		{

			"role": "user",
			"parts": []map[string]any{
				{
					"text": args["query"],
				},
			},
		},
	}...)

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
				if function, exists := PlannertoolsFunc[name]; exists {
					result := function.Run(argsData)
					fmt.Println(result)
					if _, ok = result["exit"].(bool); ok {
						return nil
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

func (c *AgentPlanner) RequestAgent(contents []map[string]any) map[string]any {
	fmt.Printf("Processing request with Planner Agent: %s\n", c.Metadata.Name)

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
			{"functionDeclarations": GetPlannerArrayMap()},
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
		req.Header.Set("X-goog-api-key", viper.GetString("PLANNER.API_KEY"))

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

func (p *AgentPlanner) NewAgent() {
	model := viper.GetString("PLANNER.MODEL")
	if model == "" {
		model = "gemini-1.5-pro"
	}

	instructions := repository.PlannerInstructions

	PlannerAgentMeta := repository.AgentMetadata{
		ID:             "Planner-agent-v1",
		Name:           "Planner Agent",
		Version:        "1.0.0",
		Type:           repository.AgentType(repository.AgentPlanner),
		Instructions:   instructions,
		LastActive:     time.Now(),
		MaxConcurrency: 5,
		Timeout:        30 * time.Second,
		Status:         "active",
		Tags:           []string{"Planner", "constraints", "inputs", "outputs", "validation"},
		Endpoints: map[string]string{
			"http": fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model),
		},
	}

	p.Metadata = PlannerAgentMeta
	p.Capabilities = PlannerCapabilities
	PlannertoolsFunc = make(map[string]repository.Function)
	for _, tool := range PlannerCapabilities {
		PlannertoolsFunc[tool.Name] = tool
	}
}

// Plan will use openAI API and give steps to do a work
func Plan(args map[string]any) map[string]any {
	name, ok := args["name"].(string)
	instructions, ok2 := args["instructions"].(string)

	if !ok || !ok2 {
		return map[string]any{"error": "insufficient parameters"}
	}

	// create tasks
	// Define a struct to hold the task data

	// Marshal the tasks data into a JSON string

	// Print the JSON string to the console

	return map[string]any{"output": "Plan Created name is " + name + " plan are " + instructions}
}

func GetPlannerArrayMap() map[string]any {
	return map[string]any{
		"create_plan": map[string]any{
			"name":        "create_plan",
			"description": "Creates a plan with a name and instructions",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The name of the plan",
						"required":    true,
					},
					"instructions": map[string]any{
						"type":        "string",
						"description": "The instructions for the plan",
						"required":    true,
					},
				},
			},
		},
		"generate_tasklist": map[string]any{
			"name":        "generate_tasklist",
			"description": "Generates a task list with a name and instructions",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The name of the task list",
						"required":    true,
					},
					"instructions": map[string]any{
						"type":        "string",
						"description": "The instructions for the task list",
						"required":    true,
					},
				},
			},
		},
	}
}

var PlannerCapabilities = []repository.Function{
	{
		Name:        "create_plan",
		Description: "Creates a plan with a name and instructions",
		Service:     Plan,
	},
	{
		Name:        "generate_tasklist",
		Description: "Generates a task list with a name and instructions",
		Service:     GenerateTasklist,
	},
}

// GenerateTasklist will take input from user and generate a tasklist and return to user
func GenerateTasklist(args map[string]any) map[string]any {
	name, ok := args["name"].(string)
	instructions, ok2 := args["instructions"].(string)

	if !ok || !ok2 {
		return map[string]any{"error": "insufficient parameters"}
	}
	return map[string]any{"output": "Tasklist name is " + name + " tasklist instructions are " + instructions}
}
