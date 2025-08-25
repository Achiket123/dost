package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	MAX_TEXT_CHARS_PER_PART = 2000
	MAX_HISTORY_ENTRIES     = 15
	MAX_ERROR_LINES         = 20
	MAX_FUNCTION_RESPONSE   = 1500
	MAX_TOTAL_CACHE_SIZE    = 100000
)

type ConversationPart struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *FunctionCallData `json:"functionCall,omitempty"`
	FunctionResponse map[string]any    `json:"functionResponse,omitempty"`
}

type FunctionCallData struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type ConversationContent struct {
	Role  string             `json:"role"`
	Parts []ConversationPart `json:"parts"`
}

type CacheData struct {
	SystemInstruction struct {
		Parts []ConversationPart `json:"parts"`
	} `json:"systemInstruction"`
	Contents []ConversationContent `json:"contents"`
	Tracker  interface{}           `json:"tracker,omitempty"`
	User     string                `json:"user,omitempty"`
}

var jsonCache *CacheData
var cacheFilePath string
var logsDir string

func InitializeCache() error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting working directory: %v", err)
	}

	dostDir := filepath.Join(workingDir, ".dost")
	if err := os.MkdirAll(dostDir, 0755); err != nil {
		return fmt.Errorf("error creating .dost directory: %v", err)
	}

	logsDir = filepath.Join(dostDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("error creating logs directory: %v", err)
	}

	cacheFilePath = filepath.Join(dostDir, "cache.json")

	jsonCache = &CacheData{
		Contents: make([]ConversationContent, 0),
	}

	loadJSONCache()

	return nil
}

func truncateSmartly(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}

	if isErrorLog(text) {
		return truncateErrorLog(text, maxChars)
	}

	if isCodeContent(text) {
		return truncateCode(text, maxChars)
	}

	if maxChars > 100 {
		keepStart := maxChars/2 - 25
		keepEnd := maxChars/2 - 25
		return text[:keepStart] + "\n...[truncated]...\n" + text[len(text)-keepEnd:]
	}

	return text[:maxChars] + "...[truncated]"
}

func isErrorLog(text string) bool {
	errorKeywords := []string{"error:", "Error:", "ERROR:", "failed:", "Failed:", "FAILED:"}
	for _, keyword := range errorKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func isCodeContent(text string) bool {
	codeIndicators := []string{"{", "}", "import ", "#include", "function ", "def ", "class "}
	for _, indicator := range codeIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}
	return false
}

func truncateErrorLog(text string, maxChars int) string {
	lines := strings.Split(text, "\n")

	var errorLines []string
	var warningLines []string
	var infoLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(strings.ToLower(line), "error") {
			errorLines = append(errorLines, line)
		} else if strings.Contains(strings.ToLower(line), "warning") {
			warningLines = append(warningLines, line)
		} else {
			infoLines = append(infoLines, line)
		}
	}

	var result strings.Builder
	currentSize := 0

	for _, line := range errorLines {
		if currentSize+len(line)+1 > maxChars {
			break
		}
		if result.Len() > 0 {
			result.WriteString("\n")
			currentSize++
		}
		result.WriteString(line)
		currentSize += len(line)
	}

	warningCount := 0
	for _, line := range warningLines {
		if currentSize+len(line)+1 > maxChars || warningCount >= 5 {
			break
		}
		if result.Len() > 0 {
			result.WriteString("\n")
			currentSize++
		}
		result.WriteString(line)
		currentSize += len(line)
		warningCount++
	}

	if len(errorLines)+len(warningLines)+len(infoLines) > warningCount+len(errorLines) {
		if currentSize+30 <= maxChars {
			result.WriteString("\n...[additional output truncated]")
		}
	}

	return result.String()
}

func truncateCode(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}

	breakPoints := []string{"}\n", "}\r\n", ";\n", ";\r\n"}

	for _, bp := range breakPoints {
		if pos := strings.LastIndex(text[:maxChars-20], bp); pos > maxChars/3 {
			return text[:pos+len(bp)] + "\n...[code truncated]"
		}
	}

	return text[:maxChars-20] + "\n...[code truncated]"
}

func storeExternalLog(content string, prefix string) (string, error) {
	timestamp := fmt.Sprintf("%d", os.Getpid())
	filename := fmt.Sprintf("%s_%s.log", prefix, timestamp)
	filepath := filepath.Join(logsDir, filename)

	err := os.WriteFile(filepath, []byte(content), 0644)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("[Large output saved to logs/%s]", filename), nil
}

func processFunctionResponse(response map[string]any) map[string]any {
	if response == nil {
		return response
	}

	result := make(map[string]any)
	totalSize := 0

	for key, value := range response {
		valueStr := fmt.Sprintf("%v", value)

		if len(valueStr) > MAX_FUNCTION_RESPONSE {

			if externalRef, err := storeExternalLog(valueStr, fmt.Sprintf("func_resp_%s", key)); err == nil {

				if key == "content" {

					result[key] = map[string]any{
						"output":     truncateSmartly(valueStr, MAX_FUNCTION_RESPONSE/3),
						"truncated":  true,
						"full_log":   externalRef,
						"size_chars": len(valueStr),
					}
				} else {

					result[key] = truncateSmartly(valueStr, MAX_FUNCTION_RESPONSE/2)
				}
				totalSize += MAX_FUNCTION_RESPONSE / 2
			} else {

				result[key] = truncateSmartly(valueStr, MAX_FUNCTION_RESPONSE/2)
				totalSize += len(result[key].(string))
			}
		} else {
			result[key] = value
			totalSize += len(valueStr)
		}

		if totalSize > MAX_FUNCTION_RESPONSE {
			result["[truncated]"] = "Additional fields omitted due to size"
			break
		}
	}

	return result
}

func processConversationPart(part *ConversationPart) {

	if part.Text != "" {
		originalLen := len(part.Text)

		if originalLen > MAX_TEXT_CHARS_PER_PART*3 {
			if externalRef, err := storeExternalLog(part.Text, "large_text"); err == nil {

				summary := truncateSmartly(part.Text, 500)
				part.Text = fmt.Sprintf("%s\n\n%s", summary, externalRef)
			} else {

				part.Text = truncateSmartly(part.Text, MAX_TEXT_CHARS_PER_PART)
			}
		} else {

			part.Text = truncateSmartly(part.Text, MAX_TEXT_CHARS_PER_PART)
		}
	}

	if part.FunctionResponse != nil {

		if response, exists := part.FunctionResponse["response"]; exists {
			if responseMap, ok := response.(map[string]any); ok {
				part.FunctionResponse["response"] = processFunctionResponse(responseMap)
			} else {

				responseStr := fmt.Sprintf("%v", response)
				if len(responseStr) > MAX_FUNCTION_RESPONSE {
					part.FunctionResponse["response"] = map[string]any{
						"content":   truncateSmartly(responseStr, MAX_FUNCTION_RESPONSE),
						"truncated": true,
					}
				} else {
					part.FunctionResponse["response"] = map[string]any{
						"content": responseStr,
					}
				}
			}
		}
	}
}

func estimateTokens(text string) int {
	return len(text) / 4
}

func getTotalCacheSize() int {
	total := 0
	for _, content := range jsonCache.Contents {
		for _, part := range content.Parts {
			total += len(part.Text)
			if part.FunctionResponse != nil {
				responseStr, _ := json.Marshal(part.FunctionResponse)
				total += len(responseStr)
			}
		}
	}
	return total
}

func limitConversationHistory() {

	if len(jsonCache.Contents) > MAX_HISTORY_ENTRIES {
		jsonCache.Contents = jsonCache.Contents[len(jsonCache.Contents)-MAX_HISTORY_ENTRIES:]
	}

	for getTotalCacheSize() > MAX_TOTAL_CACHE_SIZE && len(jsonCache.Contents) > 5 {

		if len(jsonCache.Contents) >= 2 {
			jsonCache.Contents = jsonCache.Contents[2:]
		} else {
			jsonCache.Contents = jsonCache.Contents[1:]
		}
	}

	fmt.Printf("🔧 Cache size after limiting: %d chars (~%d tokens), %d entries\n",
		getTotalCacheSize(), estimateTokens(fmt.Sprintf("%v", jsonCache.Contents)), len(jsonCache.Contents))
}

func GetJSONCache() map[string]any {
	if jsonCache == nil {
		InitializeCache()
	}

	result := make(map[string]any)

	result["systemInstruction"] = map[string]any{
		"parts": jsonCache.SystemInstruction.Parts,
	}

	result["contents"] = jsonCache.Contents

	if jsonCache.Tracker != nil {
		result["tracker"] = jsonCache.Tracker
	}
	if jsonCache.User != "" {
		result["user"] = jsonCache.User
	}

	return result
}

func PutJSONCache(data map[string]any) error {
	if jsonCache == nil {
		InitializeCache()
	}

	if tracker, exists := data["tracker"]; exists {
		jsonCache.Tracker = tracker
	}

	if user, exists := data["user"]; exists && user != nil {
		jsonCache.User = fmt.Sprintf("%v", user)
	}

	return saveJSONCache()
}

func AddUserMessage(message string) error {
	if jsonCache == nil {
		InitializeCache()
	}

	if message == "" {
		return nil
	}

	processedMessage := truncateSmartly(message, MAX_TEXT_CHARS_PER_PART)

	userContent := ConversationContent{
		Role: "user",
		Parts: []ConversationPart{
			{Text: processedMessage},
		},
	}
	jsonCache.Contents = append(jsonCache.Contents, userContent)

	limitConversationHistory()

	return saveJSONCache()
}

func AddUserMessageWithParts(parts []map[string]any) error {
	if jsonCache == nil {
		InitializeCache()
	}

	if len(parts) == 0 {
		return nil
	}

	conversationParts := make([]ConversationPart, 0, len(parts))

	for _, part := range parts {
		convPart := ConversationPart{}

		if text, exists := part["text"]; exists {
			if textStr, ok := text.(string); ok && textStr != "" {
				convPart.Text = textStr
			}
		}

		if functionCall, exists := part["functionCall"]; exists {
			if fcMap, ok := functionCall.(map[string]any); ok {
				if name, nameOk := fcMap["name"].(string); nameOk {
					args := make(map[string]interface{})
					if argsMap, argsOk := fcMap["arguments"].(map[string]any); argsOk {
						args = argsMap
					}
					convPart.FunctionCall = &FunctionCallData{
						Name: name,
						Args: args,
					}
				}
			}
		}

		if functionResponse, exists := part["functionResponse"]; exists {
			if frMap, ok := functionResponse.(map[string]any); ok {

				if response, responseExists := frMap["response"]; responseExists {

					switch responseVal := response.(type) {
					case map[string]any:

						convPart.FunctionResponse = frMap
					case string:

						frMap["response"] = map[string]any{
							"content": responseVal,
						}
						convPart.FunctionResponse = frMap
					default:

						responseStr := fmt.Sprintf("%v", responseVal)
						frMap["response"] = map[string]any{
							"content": responseStr,
						}
						convPart.FunctionResponse = frMap
					}
				} else {

					frMap["response"] = map[string]any{
						"content": "No response data",
					}
					convPart.FunctionResponse = frMap
				}
			}
		}

		processConversationPart(&convPart)

		if convPart.Text != "" || convPart.FunctionCall != nil || convPart.FunctionResponse != nil {
			conversationParts = append(conversationParts, convPart)
		}
	}

	if len(conversationParts) > 0 {
		userMessage := ConversationContent{
			Role:  "user",
			Parts: conversationParts,
		}

		jsonCache.Contents = append(jsonCache.Contents, userMessage)

		limitConversationHistory()
	}

	return saveJSONCache()
}

func AddModelResponse(response string, functionCalls []FunctionCallData) error {
	if jsonCache == nil {
		InitializeCache()
	}

	parts := make([]ConversationPart, 0)

	if response != "" && response != "[]" {
		processedResponse := truncateSmartly(response, MAX_TEXT_CHARS_PER_PART)
		parts = append(parts, ConversationPart{Text: processedResponse})
	}

	for _, funcCall := range functionCalls {
		if funcCall.Name != "" {
			parts = append(parts, ConversationPart{
				FunctionCall: &funcCall,
			})
		}
	}

	if len(parts) > 0 {
		modelContent := ConversationContent{
			Role:  "model",
			Parts: parts,
		}
		jsonCache.Contents = append(jsonCache.Contents, modelContent)

		limitConversationHistory()

		return saveJSONCache()
	}

	return nil
}

func SetSystemInstruction(instruction string) error {
	if jsonCache == nil {
		InitializeCache()
	}

	processedInstruction := truncateSmartly(instruction, MAX_TEXT_CHARS_PER_PART*2)

	jsonCache.SystemInstruction.Parts = []ConversationPart{
		{Text: processedInstruction},
	}

	return saveJSONCache()
}

func ClearCache() error {
	if jsonCache == nil {
		InitializeCache()
	}

	jsonCache.Contents = make([]ConversationContent, 0)
	jsonCache.Tracker = nil
	jsonCache.User = ""

	return saveJSONCache()
}

func GetConversationHistory() map[string]any {
	if jsonCache == nil {
		InitializeCache()
	}

	validContents := make([]ConversationContent, 0, len(jsonCache.Contents))

	for _, content := range jsonCache.Contents {

		if (content.Role == "user" || content.Role == "model") && len(content.Parts) > 0 {

			hasContent := false
			validParts := make([]ConversationPart, 0, len(content.Parts))

			for _, part := range content.Parts {
				if part.Text != "" || part.FunctionCall != nil || part.FunctionResponse != nil {

					if part.FunctionResponse != nil {
						if response, exists := part.FunctionResponse["response"]; exists {

							if _, ok := response.(map[string]any); !ok {

								responseStr := fmt.Sprintf("%v", response)
								part.FunctionResponse["response"] = map[string]any{
									"content": truncateSmartly(responseStr, MAX_FUNCTION_RESPONSE),
								}
							}
						}
					}

					validParts = append(validParts, part)
					hasContent = true
				}
			}

			if hasContent {
				content.Parts = validParts
				validContents = append(validContents, content)
			}
		}
	}

	if len(validContents) > 0 && validContents[len(validContents)-1].Role == "model" {

		guidanceMessage := ConversationContent{
			Role: "user",
			Parts: []ConversationPart{
				{Text: "Continue with the next step. If you think the task is complete, call exit_process(). Otherwise, continue with tool calls. Don't reply with filler text."},
			},
		}
		validContents = append(validContents, guidanceMessage)
	}

	totalSize := getTotalCacheSize()
	estimatedTokens := estimateTokens(fmt.Sprintf("%v", validContents))

	fmt.Printf("📊 Sending to API: %d chars (~%d tokens), %d messages\n",
		totalSize, estimatedTokens, len(validContents))

	if estimatedTokens > 50000 {
		fmt.Printf("⚠️  WARNING: Large token count detected! Consider further reducing history.\n")
	}

	return map[string]any{
		"systemInstruction": map[string]any{
			"parts": jsonCache.SystemInstruction.Parts,
		},
		"contents": validContents,
	}
}

func saveJSONCache() error {
	if cacheFilePath == "" {
		return fmt.Errorf("cache file path not initialized")
	}

	data, err := json.MarshalIndent(jsonCache, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling cache data: %v", err)
	}

	err = os.WriteFile(cacheFilePath, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing cache file: %v", err)
	}

	return nil
}

func loadJSONCache() error {
	if cacheFilePath == "" {
		return fmt.Errorf("cache file path not initialized")
	}

	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {

		return nil
	}

	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return fmt.Errorf("error reading cache file: %v", err)
	}

	if len(data) == 0 {

		return nil
	}

	err = json.Unmarshal(data, jsonCache)
	if err != nil {
		return fmt.Errorf("error unmarshaling cache data: %v", err)
	}

	return nil
}

func GetCacheFilePath() string {
	return cacheFilePath
}

func ShouldAddGuidanceMessage() bool {
	if jsonCache == nil || len(jsonCache.Contents) == 0 {
		return false
	}

	lastMessage := jsonCache.Contents[len(jsonCache.Contents)-1]
	return lastMessage.Role == "model"
}

func GetConversationSummary() string {
	if jsonCache == nil {
		return "Cache not initialized"
	}

	totalSize := getTotalCacheSize()
	estimatedTokens := estimateTokens(fmt.Sprintf("%v", jsonCache.Contents))

	summary := fmt.Sprintf("Conversation: %d messages, %d chars (~%d tokens): ",
		len(jsonCache.Contents), totalSize, estimatedTokens)

	for i, content := range jsonCache.Contents {
		summary += fmt.Sprintf("[%d: %s] ", i, content.Role)
	}

	return summary
}

func CleanupOldLogs() error {
	if logsDir == "" {
		return nil
	}

	files, err := os.ReadDir(logsDir)
	if err != nil {
		return err
	}

	if len(files) > 50 {

		for i := 0; i < len(files)-50; i++ {
			os.Remove(filepath.Join(logsDir, files[i].Name()))
		}
	}

	return nil
}
