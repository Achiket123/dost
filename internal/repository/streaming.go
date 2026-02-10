// USED AI
package repository

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// StreamChunk represents a single chunk from the streaming API
type StreamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string         `json:"name"`
					Args map[string]any `json:"args"`
				} `json:"functionCall"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason,omitempty"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
}

// StreamResponse accumulates the complete response from streaming chunks
type StreamResponse struct {
	TextParts     []string
	FunctionCalls []map[string]any
	FinishReason  string
}

// NewStreamingHTTPClient creates an HTTP client optimized for SSE streaming.
// It forces HTTP/1.1 (better SSE compatibility), disables response buffering,
// and uses a long timeout suitable for streaming responses.
func NewStreamingHTTPClient() *http.Client {
	return &http.Client{
		// No timeout on the client level — streaming responses can take minutes.
		// Individual connection timeouts are handled by the Transport.
		Timeout: 0,
		Transport: &http.Transport{
			// Force HTTP/1.1 for proper SSE streaming support
			ForceAttemptHTTP2: false,
			TLSNextProto:      make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
			// Connection-level timeouts
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			// Disable compression so SSE events arrive immediately
			DisableCompression: true,
		},
	}
}

// ParseSSEStream reads Server-Sent Events from the Gemini API.
// The API returns SSE format when using ?alt=sse parameter.
// Each event line starts with "data: " followed by a JSON object.
// When displayRealtime is true, text chunks are printed to stdout immediately.
func ParseSSEStream(body io.ReadCloser, displayRealtime bool) (*StreamResponse, error) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	// Increase buffer size to handle large SSE lines (up to 1MB)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	response := &StreamResponse{
		TextParts:     []string{},
		FunctionCalls: []map[string]any{},
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines (SSE uses blank lines as event separators)
		if strings.TrimSpace(line) == "" {
			continue
		}

		// SSE format: "data: {json}"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		jsonData := strings.TrimPrefix(line, "data: ")

		// Parse the JSON chunk
		var chunk StreamChunk
		if err := json.Unmarshal([]byte(jsonData), &chunk); err != nil {
			// Skip malformed chunks but continue processing
			continue
		}

		// Process each candidate in the chunk
		for _, candidate := range chunk.Candidates {
			if candidate.FinishReason != "" {
				response.FinishReason = candidate.FinishReason
			}

			for _, part := range candidate.Content.Parts {
				// Handle text parts — stream to stdout immediately
				if part.Text != "" {
					response.TextParts = append(response.TextParts, part.Text)
					if displayRealtime {
						fmt.Print(part.Text)
						os.Stdout.Sync()
					}
				}

				// Handle function calls
				if part.FunctionCall != nil {
					response.FunctionCalls = append(response.FunctionCalls, map[string]any{
						"name": part.FunctionCall.Name,
						"args": part.FunctionCall.Args,
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	// Print newline after streamed text
	if displayRealtime && len(response.TextParts) > 0 {
		fmt.Println()
	}

	if len(response.TextParts) == 0 && len(response.FunctionCalls) == 0 {
		return nil, fmt.Errorf("no data received from stream")
	}

	return response, nil
}

func StreamText(text string) {
	words := strings.Fields(text)
	for i, word := range words {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(word)
		os.Stdout.Sync()
		time.Sleep(30 * time.Millisecond)
	}
	fmt.Println()
	os.Stdout.Sync()
}

// ConvertStreamResponseToOutput converts a StreamResponse to the standard output format
func ConvertStreamResponseToOutput(streamResp *StreamResponse) []map[string]any {
	output := []map[string]any{}

	for _, text := range streamResp.TextParts {
		output = append(output, map[string]any{
			"type": "text",
			"data": text,
		})
	}

	for _, fc := range streamResp.FunctionCalls {
		output = append(output, map[string]any{
			"type": "functionCall",
			"name": fc["name"],
			"args": fc["args"],
		})
	}

	return output
}

// BuildChatHistoryFromStream builds a chat history entry from a streaming response
func BuildChatHistoryFromStream(streamResp *StreamResponse, role string) map[string]any {
	parts := []map[string]any{}

	for _, text := range streamResp.TextParts {
		parts = append(parts, map[string]any{"text": text})
	}

	for _, fc := range streamResp.FunctionCalls {
		parts = append(parts, map[string]any{
			"functionCall": map[string]any{
				"name": fc["name"],
				"args": fc["args"],
			},
		})
	}

	if len(parts) == 0 {
		return nil
	}

	return map[string]any{
		"role":  role,
		"parts": parts,
	}
}
