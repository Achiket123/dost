package handler

import (
	"bufio"
	"bytes"
	"context"
	"dost/internal/config"
	"dost/internal/repository"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

type AI interface {
	ToObject() map[string]any
	Request() (string, error)
	Reset() (string, error)
}
type Response struct {
	Text          string         `json:"text"`
	Index         string         `json:"index"`
	Function_name string         `json:"function_name"`
	Parameters    map[string]any `json:"parameters"`
}

type GoogleAI struct {
	Instructions string         `json:"instructions"`
	Query        string         `json:"query"`
	Response     Response       `json:"response"`
	Config       *config.Config `json:"config"`
	Tools        map[string]any `json:"tools"`
}

func NewGoogleAI(config *config.Config, Query string) *GoogleAI {
	return &GoogleAI{
		Instructions: repository.Instructions,
		Query:        Query,
		Config:       config,
	}
}

func (g *GoogleAI) Reset(Query string) {
	g.Query = Query
	g.Response = Response{}
}

func (g *GoogleAI) ToObject() map[string]any {
	return map[string]any{
		// "instructions": g.Instructions,
		"query": g.Query,
		// "response": g.Response.Text,
	}
}
func (g *GoogleAI) Request() (*GoogleAI, error) {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Start()
	body := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{
						"text": fmt.Sprintf("%v", g.ToObject()), // human-readable text
					},
				},
			},
		},
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request object: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(
		context.Background(),
		"POST",
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent", // Replace with actual endpoint
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-goog-api-key", g.Config.AI.API_KEY)

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	// Optional: parse the response into the original object
	bodyBytes, err := io.ReadAll(resp.Body)
	cobra.CheckErr(err)
	var response map[string]any

	if err = json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	candidates, ok := response["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		return nil, errors.New("no candidates found")
	}
	content := candidates[0].(map[string]any)["content"].(map[string]any)
	parts := content["parts"].([]any)
	part := parts[0].(map[string]any)
	text := part["text"].(string)

	clean := strings.TrimSpace(text)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)
	if err = json.Unmarshal([]byte(clean), &g.Response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	s.Stop()
	return g, nil
}

type OpenAI struct {
	Instructions         string         `json:"instructions"`
	PreviousConversation string         `json:"previous_conversation"`
	Query                string         `json:"query"`
	Response             Response       `json:"response"`
	Config               *config.Config `json:"config"`
	Tools                map[string]any `json:"tools"`
}

func (o *OpenAI) ToObject() map[string]any {
	return map[string]any{
		"instructions":          o.Instructions,
		"previous_conversation": o.PreviousConversation,
		"query":                 o.Query,
		"response":              o.Response,
		"config":                o.Config,
		"tools":                 o.Tools,
	}
}

func (o *OpenAI) Request() (*OpenAI, error) {
	payload := o.ToObject()

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://api.openai.com/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.Config.AI.API_KEY)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Optional: capture and decode the response if needed
	var openaiResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// You can attach parsed response to `o` if needed
	// For now, just print it for debug
	pretty, _ := json.MarshalIndent(openaiResp, "", "  ")
	fmt.Println(string(pretty))

	return o, nil
}

// cleanupCache keeps only the last few entries to prevent token waste
func CleanupCache(cachePath string) {
	file, err := os.Open(cachePath)
	if err != nil {
		return
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Keep only last 3 lines to minimize cache
	startIndex := 0
	if len(lines) > 3 {
		startIndex = len(lines) - 3
	}

	// Write back only the last few entries
	newFile, err := os.Create(cachePath)
	if err != nil {
		return
	}
	defer newFile.Close()

	for i := startIndex; i < len(lines); i++ {
		newFile.WriteString(lines[i] + "\n")
	}
}

func ShouldAddToCache(cachePath string, newData map[string]any) bool {
	file, err := os.Open(cachePath)
	if err != nil {
		return true // File doesn't exist, safe to add
	}
	defer file.Close()

	newJSON, err := json.Marshal(newData)
	if err != nil {
		return true
	}
	newJSONStr := strings.TrimSpace(string(newJSON))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == newJSONStr {
			return false // Duplicate found
		}
	}
	return true
}

// Enhanced stuck state handling - returns false if can't recover
func HandleAdvancedStuckState(tracker *repository.TaskTracker, GoogleAI **GoogleAI) bool {
	tracker.StuckCounter++

	if len(tracker.DependencyErrors) > 1 {
		fmt.Printf("🔧 Forcing dependency fix phase\n")
		tracker.CurrentPhase = "FORCED_DEPENDENCY_FIX"
		tracker.RequiresFixing = true
		return true
	}

	if len(tracker.FailedCommands) > 2 && len(tracker.FilesCreated) > 0 {
		fmt.Printf("🔨 Forcing build fix phase\n")
		tracker.CurrentPhase = "FORCED_BUILD_FIX"
		tracker.RequiresFixing = true
		return true
	}

	if tracker.LastFunctionCall == GET_PROJECT_STRUCTURE && tracker.ProjectStructureRetrieved {
		fmt.Printf("🚫 Breaking project structure loop\n")
		tracker.CurrentPhase = "FORCED_IMPLEMENTATION"
		return true
	}

	// If we've tried multiple recovery attempts, suggest termination
	if tracker.StuckCounter > 5 {
		fmt.Printf("🛑 Multiple recovery attempts failed\n")
		return false
	}

	// Reset some counters to give it another chance
	tracker.ConsecutiveReadCalls = 0
	tracker.ConsecutiveSameCommands = 0
	return true
}
