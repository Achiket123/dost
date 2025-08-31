package coder

// import (
// 	"bytes"
// 	"context"
// 	"dost/internal/repository"

// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"time"

// 	"github.com/google/uuid"
// 	"github.com/spf13/viper"
// )

// var coderName = "coder"
// var coderVersion = "0.1.0"

// type CoderAgent repository.Agent

// func (c *CoderAgent) NewAgent() *CoderAgent {

// 	model := viper.GetString("ORCHESTRATOR.MODEL")
// 	endPoints := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
// 	agentMetadata := repository.AgentMetadata{
// 		ID:             fmt.Sprintf("coder-%s", uuid.NewString()),
// 		Name:           coderName,
// 		Version:        coderVersion,
// 		Type:           repository.AgentCoder,
// 		Instructions:   repository.CoderInstructions,
// 		MaxConcurrency: 3,
// 		Timeout:        5 * time.Minute,
// 		Tags:           []string{"coder", "agent"},
// 		Endpoints:      map[string]string{"http": endPoints},
// 	}
// 	agent := repository.Agent{
// 		Metadata:     agentMetadata,
// 		Capabilities: GetCoderCapabilities(),
// 	}
// 	//Registering the AGENT
// 	coder := CoderAgent(agent)
// 	return &coder
// }

// func (p *CoderAgent) RequestAgent(contents []map[string]any) map[string]any {
// 	// Build request payload for the AI model
// 	fmt.Println("REQ :", p.Metadata.Name)
// 	request := map[string]any{
// 		"systemInstruction": map[string]any{
// 			"parts": []map[string]any{
// 				{"text": p.Metadata.Instructions},
// 			},
// 		},
// 		"contents": contents,
// 		"tools": []map[string]any{
// 			{"function_declarations": GetCoderCapabilitiesArrayMap()},
// 		},
// 	}

// 	// Marshal request body
// 	jsonBody, err := json.Marshal(request)
// 	if err != nil {
// 		return map[string]any{"error": err.Error(), "output": nil}
// 	}

// 	// Create HTTP request
// 	req, err := http.NewRequestWithContext(
// 		context.Background(),
// 		"POST",
// 		p.Metadata.Endpoints["http"],
// 		bytes.NewBuffer(jsonBody),
// 	)
// 	if err != nil {
// 		return map[string]any{"error": err.Error(), "output": nil}
// 	}
// 	req.Header.Set("Content-Type", "application/json")
// 	req.Header.Set("X-goog-api-key", viper.GetString("CODER.API_KEY"))

// 	// Execute request
// 	resp, err := http.DefaultClient.Do(req)
// 	if err != nil {
// 		return map[string]any{"error": err.Error(), "output": nil}
// 	}
// 	defer resp.Body.Close()

// 	// Handle non-200 responses
// 	if resp.StatusCode != http.StatusOK {
// 		bodyBytes, _ := io.ReadAll(resp.Body)
// 		return map[string]any{
// 			"error":  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)),
// 			"output": nil,
// 		}
// 	}

// 	// Parse valid response
// 	bodyBytes, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return map[string]any{"error": err.Error(), "output": nil}
// 	}

// 	var response repository.Response
// 	if err = json.Unmarshal(bodyBytes, &response); err != nil {
// 		return map[string]any{"error": err.Error(), "output": nil}
// 	}

// 	// Normalize into a standard output format
// 	newResponse := make([]map[string]any, 0)
// 	for _, candidate := range response.Candidates {
// 		for _, part := range candidate.Content.Parts {
// 			if part.Text != "" {
// 				newResponse = append(newResponse, map[string]any{
// 					"text": part.Text,
// 				})
// 			} else if part.FunctionCall.Name != "" {
// 				newResponse = append(newResponse, map[string]any{
// 					"function_name": part.FunctionCall.Name,
// 					"parameters":    part.FunctionCall.Args,
// 				})
// 			}
// 		}
// 	}

// 	return map[string]any{"error": nil, "output": newResponse}
// }

// func (c *CoderAgent) ToMap() map[string]any {
// 	fmt.Printf("Agents Created: %v\n", c.Metadata.ID)
// 	return map[string]any{
// 		"id":              c.Metadata.ID,
// 		"name":            c.Metadata.Name,
// 		"version":         c.Metadata.Version,
// 		"type":            c.Metadata.Type,
// 		"instructions":    c.Metadata.Instructions,
// 		"capabilities":    c.Capabilities,
// 		"max_concurrency": c.Metadata.MaxConcurrency,
// 		"timeout":         c.Metadata.Timeout.String(),
// 		"tags":            c.Metadata.Tags,
// 		"endpoints":       c.Metadata.Endpoints,
// 	}
// }

// func GetCoderMap() map[string]any {
// 	var cAgent CoderAgent
// 	agent := cAgent.NewAgent()

// 	// Marshal capabilities to JSON
// 	capsJSON, err := json.Marshal(agent.Capabilities)
// 	if err != nil {
// 		return map[string]any{
// 			"agent":        coderName,
// 			"metadata":     agent.Metadata,
// 			"capabilities": []map[string]any{}, // fallback: empty
// 		}
// 	}

// 	// Unmarshal JSON back into []map[string]any
// 	var caps []map[string]any
// 	if err := json.Unmarshal(capsJSON, &caps); err != nil {
// 		caps = []map[string]any{}
// 	}

// 	return map[string]any{
// 		"agent":        coderName,
// 		"metadata":     agent.Metadata,
// 		"capabilities": caps, // now it's []map[string]any
// 	}
// }
