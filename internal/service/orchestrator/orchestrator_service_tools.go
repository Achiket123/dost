package orchestrator

// import (
// 	"crypto/md5"
// 	"dost/internal/repository"
// 	"dost/internal/service/coder"
// 	"dost/internal/service/planner"
// 	"encoding/json"
// 	"fmt"
// 	"os"
// 	"path/filepath"
// 	"sort"
// 	"strings"
// 	"sync"
// 	"time"
// )

// // Enhanced in-memory state with context management
// var (
// 	RegisteredAgents     = make(map[string]*(repository.AgentMetadata)) // agent_id -> metadata
// 	RegisteredLiveAgents = make(map[string]any)
// 	globalContext        = make(map[string]any)            // shared orchestrator context
// 	agentContexts        = make(map[string]map[string]any) // agent_id -> agent context
// 	contextHistory       = make([]ContextSnapshot, 0)      // historical context snapshots
// 	workflowStates       = make(map[string]*WorkflowState) // workflow_id -> state
// 	mutex                sync.RWMutex
// 	contextFilePath      string
// )

// type ContextSnapshot struct {
// 	Timestamp      time.Time      `json:"timestamp"`
// 	GlobalContext  map[string]any `json:"global_context"`
// 	AgentContexts  map[string]any `json:"agent_contexts"`
// 	ActiveAgents   []string       `json:"active_agents"`
// 	WorkflowStates map[string]any `json:"workflow_states"`
// }

// type WorkflowState struct {
// 	ID        string         `json:"id"`
// 	Status    string         `json:"status"`
// 	Steps     []WorkflowStep `json:"steps"`
// 	Context   map[string]any `json:"context"`
// 	CreatedAt time.Time      `json:"created_at"`
// 	UpdatedAt time.Time      `json:"updated_at"`
// }

// type WorkflowStep struct {
// 	ID        string         `json:"id"`
// 	AgentID   string         `json:"agent_id"`
// 	Status    string         `json:"status"`
// 	Task      map[string]any `json:"task"`
// 	Result    map[string]any `json:"result"`
// 	Context   map[string]any `json:"context"`
// 	StartTime time.Time      `json:"start_time"`
// 	EndTime   *time.Time     `json:"end_time,omitempty"`
// }

// // Initialize orchestrator context management
// func InitializeOrchestratorContext() error {
// 	workingDir, err := os.Getwd()
// 	if err != nil {
// 		return fmt.Errorf("error getting working directory: %v", err)
// 	}

// 	dostDir := filepath.Join(workingDir, ".dost")
// 	if err := os.MkdirAll(dostDir, 0755); err != nil {
// 		return fmt.Errorf("error creating .dost directory: %v", err)
// 	}

// 	contextFilePath = filepath.Join(dostDir, "orchestrator_context.json")

// 	// Load existing context if available
// 	return loadOrchestratorContext()
// }

// // ------------------------- Enhanced Agent Registration -------------------------
// func RegisterAgent(data map[string]any) map[string]any {

// 	metadata, ok := data["agent_names"]
// 	if !ok {
// 		return map[string]any{"error": "missing or invalid agent_metadata"}
// 	}

// 	agentNames, ok := metadata.([]string)
// 	if !ok || len(agentNames) == 0 {
// 		return map[string]any{"error": "agent_names is required and must be a non-empty array"}
// 	}
// 	for _, Name := range agentNames {
// 		switch Name {
// 		case string(repository.AgentPlanner):
// 			var pAgent planner.PlannerAgent
// 			agent := pAgent.NewAgent()
// 			RegisteredAgents[agent.Metadata.ID] = &agent.Metadata
// 			RegisteredLiveAgents[agent.Metadata.ID] = &agent
// 		case string(repository.AgentCoder):
// 			var cAgent coder.CoderAgent
// 			agent := cAgent.NewAgent()
// 			RegisteredAgents[agent.Metadata.ID] = &agent.Metadata
// 			RegisteredLiveAgents[agent.Metadata.ID] = &agent
// 		case string(repository.AgentKnowledge):
// 		case string(repository.AgentCritic):
// 		case string(repository.AgentExecutor):
// 			// TODO: [IMPLEMENT REMAINING AGENTS]
// 		default:

// 			// case string(repository.AgentCritic):
// 			// 	agents.NewCriticAgent()
// 			// case string(repository.AgentKnowledge):
// 			// 	agents.NewKnowledgeAgent()
// 		}
// 	}

// 	// Save state + snapshot
// 	saveOrchestratorContext()
// 	createContextSnapshot("agent_registered")
// 	newAgents := make([]string, 0, len(agentNames))
// 	for _, agent := range RegisteredAgents {
// 		newAgents = append(newAgents, agent.ID)
// 	}
// 	return map[string]any{
// 		"status": "success",
// 		"agent":  newAgents,
// 	}
// }

// func DeregisterAgent(data map[string]any) map[string]any {
// 	id, ok := data["agent_id"].(string)
// 	if !ok || id == "" {
// 		return map[string]any{"error": "agent_id is required"}
// 	}

// 	mutex.Lock()
// 	defer mutex.Unlock()

// 	delete(RegisteredAgents, id)
// 	delete(agentContexts, id)

// 	saveOrchestratorContext()
// 	createContextSnapshot("agent_deregistered")

// 	return map[string]any{
// 		"status":           "success",
// 		"removed_agent_id": id,
// 	}
// }

// // ------------------------- Enhanced Context Management -------------------------

// func UpdateAgentContext(data map[string]any) map[string]any {
// 	agentID, ok := data["agent_id"].(string)
// 	if !ok || agentID == "" {
// 		return map[string]any{"error": "agent_id is required"}
// 	}

// 	context, ok := data["context"].(map[string]any)
// 	if !ok {
// 		return map[string]any{"error": "context is required"}
// 	}

// 	mutex.Lock()
// 	defer mutex.Unlock()

// 	// Update agent context
// 	if agentContexts[agentID] == nil {
// 		agentContexts[agentID] = make(map[string]any)
// 	}

// 	for k, v := range context {
// 		agentContexts[agentID][k] = v
// 	}

// 	// Update agent metadata context
// 	if agent, exists := RegisteredAgents[agentID]; exists {
// 		agent.Context = agentContexts[agentID]
// 		agent.LastActive = time.Now()
// 	}

// 	saveOrchestratorContext()

// 	return map[string]any{
// 		"status":   "updated",
// 		"agent_id": agentID,
// 		"context":  agentContexts[agentID],
// 	}
// }

// func GetAgentContext(data map[string]any) map[string]any {
// 	agentID, ok := data["agent_id"].(string)
// 	if !ok || agentID == "" {
// 		return map[string]any{"error": "agent_id is required"}
// 	}

// 	mutex.RLock()
// 	defer mutex.RUnlock()

// 	context, exists := agentContexts[agentID]
// 	if !exists {
// 		return map[string]any{"error": fmt.Sprintf("agent %s not found", agentID)}
// 	}

// 	return map[string]any{
// 		"status":   "found",
// 		"agent_id": agentID,
// 		"context":  context,
// 	}
// }

// func UpdateGlobalContext(data map[string]any) map[string]any {
// 	ctx, ok := data["context"].(map[string]any)
// 	if !ok {
// 		return map[string]any{"error": "context is required"}
// 	}

// 	mutex.Lock()
// 	defer mutex.Unlock()

// 	for k, v := range ctx {
// 		globalContext[k] = v
// 	}

// 	saveOrchestratorContext()
// 	createContextSnapshot("global_context_updated")

// 	return map[string]any{
// 		"status":  "updated",
// 		"context": globalContext,
// 	}
// }

// func GetRelevantContext(data map[string]any) map[string]any {
// 	task, ok := data["task"].(map[string]any)
// 	if !ok {
// 		return map[string]any{"error": "task is required"}
// 	}

// 	agentID := getStringFromMap(data, "agent_id", "")

// 	mutex.RLock()
// 	defer mutex.RUnlock()

// 	result := map[string]any{
// 		"status":         "context_found",
// 		"task":           task,
// 		"global_context": globalContext,
// 	}

// 	// Add agent-specific context if agent_id provided
// 	if agentID != "" {
// 		if agentContext, exists := agentContexts[agentID]; exists {
// 			result["agent_context"] = agentContext
// 		}
// 	}

// 	// Add related contexts based on task type or keywords
// 	relatedContexts := findRelatedContexts(task)
// 	if len(relatedContexts) > 0 {
// 		result["related_contexts"] = relatedContexts
// 	}

// 	return result
// }

// func MergeContexts(data map[string]any) map[string]any {
// 	contexts, ok := data["contexts"].([]map[string]any)
// 	if !ok {
// 		return map[string]any{"error": "contexts must be array of objects"}
// 	}

// 	merged := make(map[string]any)

// 	// Merge global context first
// 	mutex.RLock()
// 	for k, v := range globalContext {
// 		merged[k] = v
// 	}
// 	mutex.RUnlock()

// 	// Merge provided contexts
// 	for _, ctx := range contexts {
// 		for k, v := range ctx {
// 			merged[k] = v
// 		}
// 	}

// 	return map[string]any{
// 		"status":  "merged",
// 		"context": merged,
// 	}
// }

// // ------------------------- Context History & Snapshots -------------------------

// func createContextSnapshot(reason string) {
// 	snapshot := ContextSnapshot{
// 		Timestamp:      time.Now(),
// 		GlobalContext:  copyMap(globalContext),
// 		AgentContexts:  make(map[string]any),
// 		ActiveAgents:   make([]string, 0),
// 		WorkflowStates: make(map[string]any),
// 	}

// 	// Copy agent contexts
// 	for agentID, ctx := range agentContexts {
// 		snapshot.AgentContexts[agentID] = copyMap(ctx)
// 	}

// 	// Get active agents
// 	for agentID, agent := range RegisteredAgents {
// 		if agent.Status == "active" {
// 			snapshot.ActiveAgents = append(snapshot.ActiveAgents, agentID)
// 		}
// 	}

// 	// Copy workflow states
// 	for workflowID, state := range workflowStates {
// 		snapshot.WorkflowStates[workflowID] = map[string]any{
// 			"id":         state.ID,
// 			"status":     state.Status,
// 			"steps":      len(state.Steps),
// 			"created_at": state.CreatedAt,
// 			"updated_at": state.UpdatedAt,
// 		}
// 	}

// 	contextHistory = append(contextHistory, snapshot)

// 	// Keep only last 50 snapshots
// 	if len(contextHistory) > 50 {
// 		contextHistory = contextHistory[len(contextHistory)-50:]
// 	}
// }

// func GetContextHistory(data map[string]any) map[string]any {
// 	limit := 10 // default
// 	if l, ok := data["limit"].(float64); ok {
// 		limit = int(l)
// 	}

// 	mutex.RLock()
// 	defer mutex.RUnlock()

// 	start := 0
// 	if len(contextHistory) > limit {
// 		start = len(contextHistory) - limit
// 	}

// 	return map[string]any{
// 		"status":  "found",
// 		"history": contextHistory[start:],
// 		"total":   len(contextHistory),
// 	}
// }

// // ------------------------- Enhanced Workflow Management -------------------------

// func StartWorkflow(data map[string]any) map[string]any {
// 	workflow, ok := data["workflow"].(map[string]any)
// 	if !ok {
// 		return map[string]any{"error": "workflow is required"}
// 	}

// 	workflowID := getStringFromMap(workflow, "id", fmt.Sprintf("workflow_%d", time.Now().Unix()))

// 	mutex.Lock()
// 	defer mutex.Unlock()

// 	state := &WorkflowState{
// 		ID:        workflowID,
// 		Status:    "started",
// 		Steps:     make([]WorkflowStep, 0),
// 		Context:   copyMap(globalContext),
// 		CreatedAt: time.Now(),
// 		UpdatedAt: time.Now(),
// 	}

// 	// Add workflow-specific context
// 	if workflowCtx, exists := workflow["context"].(map[string]any); exists {
// 		for k, v := range workflowCtx {
// 			state.Context[k] = v
// 		}
// 	}

// 	workflowStates[workflowID] = state

// 	saveOrchestratorContext()
// 	createContextSnapshot("workflow_started")

// 	return map[string]any{
// 		"status":      "workflow_started",
// 		"workflow_id": workflowID,
// 		"workflow":    state,
// 	}
// }

// func UpdateWorkflowContext(data map[string]any) map[string]any {
// 	workflowID, ok := data["workflow_id"].(string)
// 	if !ok || workflowID == "" {
// 		return map[string]any{"error": "workflow_id is required"}
// 	}

// 	context, ok := data["context"].(map[string]any)
// 	if !ok {
// 		return map[string]any{"error": "context is required"}
// 	}

// 	mutex.Lock()
// 	defer mutex.Unlock()

// 	state, exists := workflowStates[workflowID]
// 	if !exists {
// 		return map[string]any{"error": fmt.Sprintf("workflow %s not found", workflowID)}
// 	}

// 	for k, v := range context {
// 		state.Context[k] = v
// 	}
// 	state.UpdatedAt = time.Now()

// 	saveOrchestratorContext()

// 	return map[string]any{
// 		"status":      "updated",
// 		"workflow_id": workflowID,
// 		"context":     state.Context,
// 	}
// }

// // ------------------------- Enhanced Task Routing -------------------------

// func RouteTask(data map[string]any) map[string]any {
// 	task, ok := data["task"]
// 	if !ok {
// 		return map[string]any{"error": "task is required"}
// 	}

// 	mutex.RLock()
// 	defer mutex.RUnlock()

// 	// Enhanced routing logic based on agent capabilities and context
// 	bestAgent, ok := data["agent_id"]
// 	if !ok {
// 		return map[string]any{"error": "agent_id is required"}
// 	}

// 	// Include relevant context with the routed task
// 	context, ok := data["context"]

// 	if !ok {
// 		return map[string]any{"error": "context is required"}
// 	}

// 	RegisteredAgents[bestAgent.(string)].LastActive = time.Now()
// 	var output map[string]any
// 	switch RegisteredAgents[bestAgent.(string)].Type {
// 	case repository.AgentPlanner:

// 		agentAcquired := RegisteredLiveAgents[bestAgent.(string)].(repository.ActiveAgent)
// 		contents := []map[string]any{
// 			{
// 				"parts": []map[string]any{
// 					{"text": task.(map[string]any)["description"]},
// 				},
// 			},
// 		}
// 		output = agentAcquired.RequestAgent(contents)
// 		if output["error"] != nil {
// 			return map[string]any{
// 				"status":  "error",
// 				"message": output["error"],
// 			}
// 		}
// 	}

// 	return map[string]any{
// 		"status":   "routed",
// 		"agent_id": bestAgent.(string),
// 		"task":     task.(map[string]any),
// 		"context":  context.(map[string]any),
// 		"output":   output["output"],
// 	}
// }

// // ------------------------- Utility Functions -------------------------

// func findRelatedContexts(task map[string]any) map[string]any {
// 	related := make(map[string]any)

// 	taskType := getStringFromMap(task, "type", "")
// 	keywords := getStringSliceFromMap(task, "keywords")

// 	// Find contexts with similar keywords or types
// 	for agentID, context := range agentContexts {
// 		if hasRelevantContext(context, taskType, keywords) {
// 			related[agentID] = context
// 		}
// 	}

// 	return related
// }

// func hasRelevantContext(context map[string]any, taskType string, keywords []string) bool {
// 	contextStr := fmt.Sprintf("%v", context)

// 	// Check if task type appears in context
// 	if taskType != "" && contains(contextStr, taskType) {
// 		return true
// 	}

// 	// Check for keyword matches
// 	for _, keyword := range keywords {
// 		if contains(contextStr, keyword) {
// 			return true
// 		}
// 	}

// 	return false
// }

// func getRelevantContextForTask(task map[string]any, agentID string) map[string]any {
// 	context := make(map[string]any)

// 	// Add global context
// 	for k, v := range globalContext {
// 		context[k] = v
// 	}

// 	// Add agent-specific context
// 	if agentContext, exists := agentContexts[agentID]; exists {
// 		for k, v := range agentContext {
// 			context["agent_"+k] = v
// 		}
// 	}

// 	return context
// }

// // ------------------------- Persistence Functions -------------------------

// func saveOrchestratorContext() error {
// 	if contextFilePath == "" {
// 		return fmt.Errorf("context file path not initialized")
// 	}

// 	contextData := map[string]any{
// 		"global_context":    globalContext,
// 		"agent_contexts":    agentContexts,
// 		"registered_agents": RegisteredAgents,
// 		"workflow_states":   workflowStates,
// 		"context_history":   contextHistory,
// 		"last_updated":      time.Now(),
// 	}

// 	data, err := json.MarshalIndent(contextData, "", "  ")
// 	if err != nil {
// 		return fmt.Errorf("error marshaling context data: %v", err)
// 	}

// 	return os.WriteFile(contextFilePath, data, 0644)
// }

// func loadOrchestratorContext() error {
// 	if contextFilePath == "" {
// 		return fmt.Errorf("context file path not initialized")
// 	}

// 	if _, err := os.Stat(contextFilePath); os.IsNotExist(err) {
// 		return nil // File doesn't exist yet, that's okay
// 	}

// 	data, err := os.ReadFile(contextFilePath)
// 	if err != nil {
// 		return fmt.Errorf("error reading context file: %v", err)
// 	}

// 	if len(data) == 0 {
// 		return nil
// 	}

// 	var contextData map[string]any
// 	if err := json.Unmarshal(data, &contextData); err != nil {
// 		return fmt.Errorf("error unmarshaling context data: %v", err)
// 	}

// 	// Restore contexts
// 	if gc, ok := contextData["global_context"].(map[string]any); ok {
// 		globalContext = gc
// 	}

// 	if ac, ok := contextData["agent_contexts"].(map[string]any); ok {
// 		agentContexts = make(map[string]map[string]any)
// 		for k, v := range ac {
// 			if ctx, ok := v.(map[string]any); ok {
// 				agentContexts[k] = ctx
// 			}
// 		}
// 	}

// 	// Restore agents (would need custom unmarshaling for complex types)
// 	// This is simplified - you might want to implement custom JSON marshaling

// 	return nil
// }

// // Helper functions
// func getStringFromMap(m map[string]any, key, defaultVal string) string {
// 	if val, ok := m[key].(string); ok {
// 		return val
// 	}
// 	return defaultVal
// }

// func getStringSliceFromMap(m map[string]any, key string) []string {
// 	if val, ok := m[key].([]string); ok {
// 		return val
// 	}
// 	if val, ok := m[key].([]any); ok {
// 		result := make([]string, 0, len(val))
// 		for _, v := range val {
// 			if s, ok := v.(string); ok {
// 				result = append(result, s)
// 			}
// 		}
// 		return result
// 	}
// 	return []string{}
// }

// func copyMap(original map[string]any) map[string]any {
// 	copy := make(map[string]any)
// 	for k, v := range original {
// 		copy[k] = v
// 	}
// 	return copy
// }

// func contains(str, substr string) bool {
// 	return len(str) >= len(substr) && (str == substr ||
// 		(len(str) > len(substr) && (str[:len(substr)] == substr ||
// 			str[len(str)-len(substr):] == substr ||
// 			len(str) > len(substr)*2)))
// }

// // --------------------------- Cache Configuration ---------------------------

// type CacheConfig struct {
// 	MaxResponses    int           `json:"max_responses"`     // Maximum number of cached responses
// 	MaxAge          time.Duration `json:"max_age"`           // Maximum age before expiration
// 	MaxSizeBytes    int64         `json:"max_size_bytes"`    // Maximum total cache size in bytes
// 	EvictionPolicy  string        `json:"eviction_policy"`   // "lru", "time", "size"
// 	PersistToDisk   bool          `json:"persist_to_disk"`   // Whether to save cache to disk
// 	ContextKeyLimit int           `json:"context_key_limit"` // Max context keys to extract per response
// }

// // Default cache configuration
// var defaultCacheConfig = CacheConfig{
// 	MaxResponses:    500,
// 	MaxAge:          24 * time.Hour,
// 	MaxSizeBytes:    50 * 1024 * 1024, // 50MB
// 	EvictionPolicy:  "lru",
// 	PersistToDisk:   true,
// 	ContextKeyLimit: 20,
// }

// // --------------------------- Cache Structures ---------------------------

// type CachedResponse struct {
// 	ID                string         `json:"id"`                  // Unique response ID
// 	AgentID           string         `json:"agent_id"`            // Agent that produced the response
// 	TaskID            string         `json:"task_id"`             // Task identifier
// 	TaskHash          string         `json:"task_hash"`           // Hash of task content for deduplication
// 	Timestamp         time.Time      `json:"timestamp"`           // When response was cached
// 	LastAccessed      time.Time      `json:"last_accessed"`       // LRU tracking
// 	AccessCount       int            `json:"access_count"`        // Usage frequency
// 	RawResponse       map[string]any `json:"raw_response"`        // Complete original response
// 	RelevantContext   map[string]any `json:"relevant_context"`    // Extracted relevant context
// 	ResponseSizeBytes int64          `json:"response_size_bytes"` // Size tracking
// 	ExpiresAt         time.Time      `json:"expires_at"`          // Expiration time
// 	Tags              []string       `json:"tags"`                // Searchable tags
// 	Success           bool           `json:"success"`             // Whether response was successful
// }

// type ResponseCache struct {
// 	responses      map[string]*CachedResponse // response_id -> cached response
// 	agentIndex     map[string][]string        // agent_id -> response_ids
// 	taskIndex      map[string][]string        // task_id -> response_ids
// 	hashIndex      map[string]string          // task_hash -> response_id
// 	timeIndex      []string                   // response_ids sorted by timestamp
// 	accessIndex    []string                   // response_ids sorted by last_accessed
// 	config         CacheConfig
// 	totalSizeBytes int64
// 	cacheFilePath  string
// 	mutex          sync.RWMutex
// }

// // Global response cache instance
// var globalResponseCache *ResponseCache

// // --------------------------- Cache Initialization ---------------------------

// func InitializeResponseCache(config *CacheConfig) error {
// 	if config == nil {
// 		config = &defaultCacheConfig
// 	}

// 	workingDir, err := os.Getwd()
// 	if err != nil {
// 		return fmt.Errorf("error getting working directory: %v", err)
// 	}

// 	dostDir := filepath.Join(workingDir, ".dost")
// 	if err := os.MkdirAll(dostDir, 0755); err != nil {
// 		return fmt.Errorf("error creating .dost directory: %v", err)
// 	}

// 	cacheFilePath := filepath.Join(dostDir, "response_cache.json")

// 	globalResponseCache = &ResponseCache{
// 		responses:     make(map[string]*CachedResponse),
// 		agentIndex:    make(map[string][]string),
// 		taskIndex:     make(map[string][]string),
// 		hashIndex:     make(map[string]string),
// 		timeIndex:     make([]string, 0),
// 		accessIndex:   make([]string, 0),
// 		config:        *config,
// 		cacheFilePath: cacheFilePath,
// 	}

// 	return globalResponseCache.loadFromDisk()
// }

// // --------------------------- Core Cache Operations ---------------------------

// func CacheAgentResponse(agentID, taskID string, task map[string]any, response map[string]any) string {
// 	if globalResponseCache == nil {
// 		InitializeResponseCache(nil)
// 	}

// 	taskHash := generateTaskHash(task)

// 	// Check for existing response with same task hash
// 	if existingID, exists := globalResponseCache.hashIndex[taskHash]; exists {
// 		globalResponseCache.mutex.Lock()
// 		if cachedResp, found := globalResponseCache.responses[existingID]; found && !isExpired(cachedResp) {
// 			// Update access tracking
// 			cachedResp.LastAccessed = time.Now()
// 			cachedResp.AccessCount++
// 			globalResponseCache.mutex.Unlock()
// 			return existingID // Return existing cache ID
// 		}
// 		globalResponseCache.mutex.Unlock()
// 	}

// 	responseID := fmt.Sprintf("%s_%s_%d", agentID, taskID, time.Now().UnixNano())

// 	rawResponseBytes, _ := json.Marshal(response)
// 	responseSize := int64(len(rawResponseBytes))

// 	relevantContext := extractRelevantContext(response, globalResponseCache.config.ContextKeyLimit)

// 	cachedResponse := &CachedResponse{
// 		ID:                responseID,
// 		AgentID:           agentID,
// 		TaskID:            taskID,
// 		TaskHash:          taskHash,
// 		Timestamp:         time.Now(),
// 		LastAccessed:      time.Now(),
// 		AccessCount:       1,
// 		RawResponse:       deepCopyMap(response),
// 		RelevantContext:   relevantContext,
// 		ResponseSizeBytes: responseSize,
// 		ExpiresAt:         time.Now().Add(globalResponseCache.config.MaxAge),
// 		Tags:              generateResponseTags(task, response),
// 		Success:           response["error"] == nil,
// 	}

// 	globalResponseCache.mutex.Lock()
// 	defer globalResponseCache.mutex.Unlock()

// 	// Store response
// 	globalResponseCache.responses[responseID] = cachedResponse
// 	globalResponseCache.totalSizeBytes += responseSize

// 	// Update indexes
// 	globalResponseCache.agentIndex[agentID] = append(globalResponseCache.agentIndex[agentID], responseID)
// 	globalResponseCache.taskIndex[taskID] = append(globalResponseCache.taskIndex[taskID], responseID)
// 	globalResponseCache.hashIndex[taskHash] = responseID

// 	// Insert in sorted order for time and access indexes
// 	globalResponseCache.insertIntoTimeIndex(responseID)
// 	globalResponseCache.insertIntoAccessIndex(responseID)

// 	// Trigger cleanup if needed
// 	globalResponseCache.enforceEvictionPolicies()

// 	if globalResponseCache.config.PersistToDisk {
// 		globalResponseCache.saveToDisk()
// 	}

// 	return responseID
// }

// func GetCachedResponse(responseID string, includeRaw bool) map[string]any {
// 	if globalResponseCache == nil {
// 		return map[string]any{"error": "cache not initialized"}
// 	}

// 	globalResponseCache.mutex.RLock()
// 	cachedResp, exists := globalResponseCache.responses[responseID]
// 	globalResponseCache.mutex.RUnlock()

// 	if !exists {
// 		return map[string]any{"error": "response not found in cache"}
// 	}

// 	if isExpired(cachedResp) {
// 		return map[string]any{"error": "cached response expired"}
// 	}

// 	// Update access tracking
// 	globalResponseCache.mutex.Lock()
// 	cachedResp.LastAccessed = time.Now()
// 	cachedResp.AccessCount++
// 	globalResponseCache.mutex.Unlock()

// 	result := map[string]any{
// 		"status":           "cache_hit",
// 		"response_id":      responseID,
// 		"agent_id":         cachedResp.AgentID,
// 		"task_id":          cachedResp.TaskID,
// 		"timestamp":        cachedResp.Timestamp,
// 		"relevant_context": cachedResp.RelevantContext,
// 		"access_count":     cachedResp.AccessCount,
// 		"success":          cachedResp.Success,
// 	}

// 	if includeRaw {
// 		result["raw_response"] = cachedResp.RawResponse
// 	}

// 	return result
// }

// func GetCachedResponsesByAgent(agentID string, limit int) map[string]any {
// 	if globalResponseCache == nil {
// 		return map[string]any{"error": "cache not initialized"}
// 	}

// 	globalResponseCache.mutex.RLock()
// 	responseIDs, exists := globalResponseCache.agentIndex[agentID]
// 	globalResponseCache.mutex.RUnlock()

// 	if !exists || len(responseIDs) == 0 {
// 		return map[string]any{
// 			"status":    "no_responses",
// 			"agent_id":  agentID,
// 			"responses": []map[string]any{},
// 		}
// 	}

// 	// Sort by timestamp (most recent first) and apply limit
// 	sortedResponses := globalResponseCache.sortResponsesByTime(responseIDs, true)
// 	if limit > 0 && len(sortedResponses) > limit {
// 		sortedResponses = sortedResponses[:limit]
// 	}

// 	responses := make([]map[string]any, 0, len(sortedResponses))
// 	for _, respID := range sortedResponses {
// 		if cached := GetCachedResponse(respID, false); cached["error"] == nil {
// 			responses = append(responses, cached)
// 		}
// 	}

// 	return map[string]any{
// 		"status":    "found",
// 		"agent_id":  agentID,
// 		"responses": responses,
// 		"total":     len(responseIDs),
// 	}
// }

// func FindSimilarCachedResponses(task map[string]any, agentID string, similarity float64) map[string]any {
// 	if globalResponseCache == nil {
// 		return map[string]any{"error": "cache not initialized"}
// 	}

// 	taskHash := generateTaskHash(task)

// 	globalResponseCache.mutex.RLock()
// 	defer globalResponseCache.mutex.RUnlock()

// 	// Check for exact match first
// 	if existingID, exists := globalResponseCache.hashIndex[taskHash]; exists {
// 		if cachedResp, found := globalResponseCache.responses[existingID]; found && !isExpired(cachedResp) {
// 			return map[string]any{
// 				"status":      "exact_match",
// 				"response_id": existingID,
// 				"similarity":  1.0,
// 				"context":     cachedResp.RelevantContext,
// 			}
// 		}
// 	}

// 	// Find similar responses
// 	candidates := make([]struct {
// 		responseID string
// 		similarity float64
// 	}, 0)

// 	searchTerms := extractSearchTerms(task)

// 	for responseID, cachedResp := range globalResponseCache.responses {
// 		if isExpired(cachedResp) {
// 			continue
// 		}

// 		// Filter by agent if specified
// 		if agentID != "" && cachedResp.AgentID != agentID {
// 			continue
// 		}

// 		sim := calculateSimilarity(searchTerms, cachedResp.Tags, cachedResp.RelevantContext)
// 		if sim >= similarity {
// 			candidates = append(candidates, struct {
// 				responseID string
// 				similarity float64
// 			}{responseID, sim})
// 		}
// 	}

// 	// Sort by similarity (highest first)
// 	sort.Slice(candidates, func(i, j int) bool {
// 		return candidates[i].similarity > candidates[j].similarity
// 	})

// 	if len(candidates) == 0 {
// 		return map[string]any{
// 			"status": "no_similar_responses",
// 			"query":  task,
// 		}
// 	}

// 	// Return top matches
// 	matches := make([]map[string]any, 0, min(len(candidates), 5))
// 	for i, candidate := range candidates {
// 		if i >= 5 {
// 			break
// 		}

// 		if cachedResp, exists := globalResponseCache.responses[candidate.responseID]; exists {
// 			matches = append(matches, map[string]any{
// 				"response_id": candidate.responseID,
// 				"agent_id":    cachedResp.AgentID,
// 				"similarity":  candidate.similarity,
// 				"context":     cachedResp.RelevantContext,
// 				"timestamp":   cachedResp.Timestamp,
// 			})
// 		}
// 	}

// 	return map[string]any{
// 		"status":  "similar_found",
// 		"matches": matches,
// 		"query":   task,
// 	}
// }

// // --------------------------- Cache Management Functions ---------------------------

// func ClearExpiredResponses() map[string]any {
// 	if globalResponseCache == nil {
// 		return map[string]any{"error": "cache not initialized"}
// 	}

// 	globalResponseCache.mutex.Lock()
// 	defer globalResponseCache.mutex.Unlock()

// 	expiredCount := 0
// 	now := time.Now()

// 	for responseID, cachedResp := range globalResponseCache.responses {
// 		if cachedResp.ExpiresAt.Before(now) {
// 			globalResponseCache.removeFromIndexes(responseID)
// 			globalResponseCache.totalSizeBytes -= cachedResp.ResponseSizeBytes
// 			delete(globalResponseCache.responses, responseID)
// 			expiredCount++
// 		}
// 	}

// 	if globalResponseCache.config.PersistToDisk {
// 		globalResponseCache.saveToDisk()
// 	}

// 	return map[string]any{
// 		"status":        "cleaned",
// 		"expired_count": expiredCount,
// 		"remaining":     len(globalResponseCache.responses),
// 		"total_size_mb": float64(globalResponseCache.totalSizeBytes) / (1024 * 1024),
// 	}
// }

// func GetCacheStatistics() map[string]any {
// 	if globalResponseCache == nil {
// 		return map[string]any{"error": "cache not initialized"}
// 	}

// 	globalResponseCache.mutex.RLock()
// 	defer globalResponseCache.mutex.RUnlock()

// 	agentStats := make(map[string]int)
// 	successCount := 0

// 	for _, cachedResp := range globalResponseCache.responses {
// 		agentStats[cachedResp.AgentID]++
// 		if cachedResp.Success {
// 			successCount++
// 		}
// 	}

// 	return map[string]any{
// 		"total_responses":    len(globalResponseCache.responses),
// 		"total_size_bytes":   globalResponseCache.totalSizeBytes,
// 		"total_size_mb":      float64(globalResponseCache.totalSizeBytes) / (1024 * 1024),
// 		"success_rate":       float64(successCount) / float64(len(globalResponseCache.responses)),
// 		"agent_distribution": agentStats,
// 		"config":             globalResponseCache.config,
// 		"oldest_response":    globalResponseCache.getOldestResponseTime(),
// 		"newest_response":    globalResponseCache.getNewestResponseTime(),
// 	}
// }

// // --------------------------- Integration with Existing Context System ---------------------------

// // Enhanced GetRelevantContext that checks cache first
// func GetRelevantContextEnhanced(data map[string]any) map[string]any {
// 	task, ok := data["task"].(map[string]any)
// 	if !ok {
// 		return map[string]any{"error": "task is required"}
// 	}

// 	agentID := getStringFromMap(data, "agent_id", "")

// 	// Check cache for similar responses first
// 	if globalResponseCache != nil {
// 		similarResponses := FindSimilarCachedResponses(task, agentID, 0.8)
// 		if similarResponses["status"] == "exact_match" || similarResponses["status"] == "similar_found" {
// 			// Merge cached context with current context
// 			result := GetRelevantContext(data) // Call original function

// 			if similarResponses["status"] == "exact_match" {
// 				if context, exists := similarResponses["context"].(map[string]any); exists {
// 					if globalCtx, ok := result["global_context"].(map[string]any); ok {
// 						for k, v := range context {
// 							globalCtx["cached_"+k] = v
// 						}
// 					}
// 					result["cache_hit"] = true
// 					result["cache_source"] = "exact_match"
// 				}
// 			} else if matches, exists := similarResponses["matches"].([]map[string]any); exists && len(matches) > 0 {
// 				// Use the most similar match
// 				bestMatch := matches[0]
// 				if context, exists := bestMatch["context"].(map[string]any); exists {
// 					if globalCtx, ok := result["global_context"].(map[string]any); ok {
// 						for k, v := range context {
// 							globalCtx["similar_cached_"+k] = v
// 						}
// 					}
// 					result["cache_hit"] = true
// 					result["cache_source"] = "similar_match"
// 					result["similarity"] = bestMatch["similarity"]
// 				}
// 			}

// 			return result
// 		}
// 	}

// 	// No cache hit, return original context
// 	return GetRelevantContext(data)
// }

// // Enhanced RouteTask that caches responses
// func RouteTaskWithCache(data map[string]any) map[string]any {
// 	task, ok := data["task"]
// 	if !ok {
// 		return map[string]any{"error": "task is required"}
// 	}

// 	agentID, ok := data["agent_id"].(string)
// 	if !ok {
// 		return map[string]any{"error": "agent_id is required"}
// 	}

// 	taskMap := task.(map[string]any)
// 	taskID := getStringFromMap(taskMap, "id", fmt.Sprintf("task_%d", time.Now().UnixNano()))

// 	// Check cache for exact match first
// 	if globalResponseCache != nil {
// 		similarResponses := FindSimilarCachedResponses(taskMap, agentID, 1.0)
// 		if similarResponses["status"] == "exact_match" {
// 			cachedResponseID := similarResponses["response_id"].(string)
// 			cachedData := GetCachedResponse(cachedResponseID, true)

// 			if cachedData["error"] == nil {
// 				result := cachedData["raw_response"].(map[string]any)
// 				result["cache_hit"] = true
// 				result["cached_response_id"] = cachedResponseID
// 				return result
// 			}
// 		}
// 	}

// 	// No cache hit, execute the task
// 	result := RouteTask(data)

// 	// Cache the response
// 	if globalResponseCache != nil && result["error"] == nil {
// 		responseID := CacheAgentResponse(agentID, taskID, taskMap, result)
// 		result["cached_response_id"] = responseID
// 		result["cache_hit"] = false
// 	}

// 	return result
// }

// // --------------------------- Helper Functions ---------------------------

// func (rc *ResponseCache) enforceEvictionPolicies() {
// 	// Size-based eviction
// 	if rc.totalSizeBytes > rc.config.MaxSizeBytes {
// 		rc.evictBySize()
// 	}

// 	// Count-based eviction
// 	if len(rc.responses) > rc.config.MaxResponses {
// 		rc.evictByPolicy()
// 	}

// 	// Clean expired responses
// 	rc.cleanExpired()
// }

// func (rc *ResponseCache) evictByPolicy() {
// 	toRemove := len(rc.responses) - rc.config.MaxResponses

// 	var idsToRemove []string

// 	switch rc.config.EvictionPolicy {
// 	case "lru":
// 		// Remove least recently accessed
// 		sort.Slice(rc.accessIndex, func(i, j int) bool {
// 			resp1 := rc.responses[rc.accessIndex[i]]
// 			resp2 := rc.responses[rc.accessIndex[j]]
// 			return resp1.LastAccessed.Before(resp2.LastAccessed)
// 		})
// 		idsToRemove = rc.accessIndex[:toRemove]

// 	case "time":
// 		// Remove oldest
// 		sort.Slice(rc.timeIndex, func(i, j int) bool {
// 			resp1 := rc.responses[rc.timeIndex[i]]
// 			resp2 := rc.responses[rc.timeIndex[j]]
// 			return resp1.Timestamp.Before(resp2.Timestamp)
// 		})
// 		idsToRemove = rc.timeIndex[:toRemove]

// 	case "size":
// 		// Remove largest responses first
// 		type sizeEntry struct {
// 			id   string
// 			size int64
// 		}

// 		sizeList := make([]sizeEntry, 0, len(rc.responses))
// 		for id, resp := range rc.responses {
// 			sizeList = append(sizeList, sizeEntry{id, resp.ResponseSizeBytes})
// 		}

// 		sort.Slice(sizeList, func(i, j int) bool {
// 			return sizeList[i].size > sizeList[j].size
// 		})

// 		for i := 0; i < toRemove && i < len(sizeList); i++ {
// 			idsToRemove = append(idsToRemove, sizeList[i].id)
// 		}
// 	}

// 	for _, id := range idsToRemove {
// 		rc.removeResponse(id)
// 	}
// }

// func (rc *ResponseCache) evictBySize() {
// 	targetSize := rc.config.MaxSizeBytes * 8 / 10 // Reduce to 80% of max

// 	// Sort by access frequency and time (keep frequently accessed, recent responses)
// 	type responseScore struct {
// 		id    string
// 		score float64
// 	}

// 	scores := make([]responseScore, 0, len(rc.responses))
// 	now := time.Now()

// 	for id, resp := range rc.responses {
// 		// Score based on access count and recency
// 		ageHours := now.Sub(resp.LastAccessed).Hours()
// 		accessScore := float64(resp.AccessCount)
// 		timeScore := 1.0 / (1.0 + ageHours/24.0) // Decay over days

// 		score := accessScore * timeScore
// 		scores = append(scores, responseScore{id, score})
// 	}

// 	// Sort by score (lowest first - these will be removed)
// 	sort.Slice(scores, func(i, j int) bool {
// 		return scores[i].score < scores[j].score
// 	})

// 	// Remove responses until under target size
// 	for _, entry := range scores {
// 		if rc.totalSizeBytes <= targetSize {
// 			break
// 		}
// 		rc.removeResponse(entry.id)
// 	}
// }

// func (rc *ResponseCache) cleanExpired() {
// 	now := time.Now()
// 	expiredIDs := make([]string, 0)

// 	for id, resp := range rc.responses {
// 		if resp.ExpiresAt.Before(now) {
// 			expiredIDs = append(expiredIDs, id)
// 		}
// 	}

// 	for _, id := range expiredIDs {
// 		rc.removeResponse(id)
// 	}
// }

// func (rc *ResponseCache) removeResponse(responseID string) {
// 	if resp, exists := rc.responses[responseID]; exists {
// 		rc.totalSizeBytes -= resp.ResponseSizeBytes
// 		delete(rc.responses, responseID)
// 		rc.removeFromIndexes(responseID)
// 	}
// }

// func (rc *ResponseCache) removeFromIndexes(responseID string) {
// 	// Remove from agent index
// 	for agentID, responseIDs := range rc.agentIndex {
// 		for i, id := range responseIDs {
// 			if id == responseID {
// 				rc.agentIndex[agentID] = append(responseIDs[:i], responseIDs[i+1:]...)
// 				break
// 			}
// 		}
// 	}

// 	// Remove from task index
// 	for taskID, responseIDs := range rc.taskIndex {
// 		for i, id := range responseIDs {
// 			if id == responseID {
// 				rc.taskIndex[taskID] = append(responseIDs[:i], responseIDs[i+1:]...)
// 				break
// 			}
// 		}
// 	}

// 	// Remove from hash index
// 	for hash, id := range rc.hashIndex {
// 		if id == responseID {
// 			delete(rc.hashIndex, hash)
// 			break
// 		}
// 	}

// 	// Remove from time and access indexes
// 	rc.removeFromSlice(&rc.timeIndex, responseID)
// 	rc.removeFromSlice(&rc.accessIndex, responseID)
// }

// func (rc *ResponseCache) insertIntoTimeIndex(responseID string) {
// 	resp := rc.responses[responseID]
// 	insertPos := sort.Search(len(rc.timeIndex), func(i int) bool {
// 		other := rc.responses[rc.timeIndex[i]]
// 		return other.Timestamp.After(resp.Timestamp)
// 	})

// 	rc.timeIndex = append(rc.timeIndex, "")
// 	copy(rc.timeIndex[insertPos+1:], rc.timeIndex[insertPos:])
// 	rc.timeIndex[insertPos] = responseID
// }

// func (rc *ResponseCache) insertIntoAccessIndex(responseID string) {
// 	resp := rc.responses[responseID]
// 	insertPos := sort.Search(len(rc.accessIndex), func(i int) bool {
// 		other := rc.responses[rc.accessIndex[i]]
// 		return other.LastAccessed.After(resp.LastAccessed)
// 	})

// 	rc.accessIndex = append(rc.accessIndex, "")
// 	copy(rc.accessIndex[insertPos+1:], rc.accessIndex[insertPos:])
// 	rc.accessIndex[insertPos] = responseID
// }

// func (rc *ResponseCache) removeFromSlice(slice *[]string, item string) {
// 	for i, v := range *slice {
// 		if v == item {
// 			*slice = append((*slice)[:i], (*slice)[i+1:]...)
// 			break
// 		}
// 	}
// }

// func (rc *ResponseCache) sortResponsesByTime(responseIDs []string, descending bool) []string {
// 	sorted := make([]string, len(responseIDs))
// 	copy(sorted, responseIDs)

// 	sort.Slice(sorted, func(i, j int) bool {
// 		resp1 := rc.responses[sorted[i]]
// 		resp2 := rc.responses[sorted[j]]
// 		if descending {
// 			return resp1.Timestamp.After(resp2.Timestamp)
// 		}
// 		return resp1.Timestamp.Before(resp2.Timestamp)
// 	})

// 	return sorted
// }

// func (rc *ResponseCache) getOldestResponseTime() *time.Time {
// 	if len(rc.responses) == 0 {
// 		return nil
// 	}

// 	var oldest *time.Time
// 	for _, resp := range rc.responses {
// 		if oldest == nil || resp.Timestamp.Before(*oldest) {
// 			t := resp.Timestamp
// 			oldest = &t
// 		}
// 	}
// 	return oldest
// }

// func (rc *ResponseCache) getNewestResponseTime() *time.Time {
// 	if len(rc.responses) == 0 {
// 		return nil
// 	}

// 	var newest *time.Time
// 	for _, resp := range rc.responses {
// 		if newest == nil || resp.Timestamp.After(*newest) {
// 			t := resp.Timestamp
// 			newest = &t
// 		}
// 	}
// 	return newest
// }

// // --------------------------- Persistence Functions ---------------------------

// func (rc *ResponseCache) saveToDisk() error {
// 	if rc.cacheFilePath == "" {
// 		return fmt.Errorf("cache file path not initialized")
// 	}

// 	cacheData := map[string]any{
// 		"responses":        rc.responses,
// 		"agent_index":      rc.agentIndex,
// 		"task_index":       rc.taskIndex,
// 		"hash_index":       rc.hashIndex,
// 		"time_index":       rc.timeIndex,
// 		"access_index":     rc.accessIndex,
// 		"config":           rc.config,
// 		"total_size_bytes": rc.totalSizeBytes,
// 		"last_saved":       time.Now(),
// 	}

// 	data, err := json.MarshalIndent(cacheData, "", "  ")
// 	if err != nil {
// 		return fmt.Errorf("error marshaling cache data: %v", err)
// 	}

// 	return os.WriteFile(rc.cacheFilePath, data, 0644)
// }

// func (rc *ResponseCache) loadFromDisk() error {
// 	if rc.cacheFilePath == "" {
// 		return fmt.Errorf("cache file path not initialized")
// 	}

// 	if _, err := os.Stat(rc.cacheFilePath); os.IsNotExist(err) {
// 		return nil // File doesn't exist yet
// 	}

// 	data, err := os.ReadFile(rc.cacheFilePath)
// 	if err != nil {
// 		return fmt.Errorf("error reading cache file: %v", err)
// 	}

// 	if len(data) == 0 {
// 		return nil
// 	}

// 	var cacheData map[string]any
// 	if err := json.Unmarshal(data, &cacheData); err != nil {
// 		return fmt.Errorf("error unmarshaling cache data: %v", err)
// 	}

// 	// Restore responses
// 	if responses, ok := cacheData["responses"].(map[string]any); ok {
// 		for id, respData := range responses {
// 			if respBytes, err := json.Marshal(respData); err == nil {
// 				var cachedResp CachedResponse
// 				if err := json.Unmarshal(respBytes, &cachedResp); err == nil {
// 					rc.responses[id] = &cachedResp
// 				}
// 			}
// 		}
// 	}

// 	// Restore indexes
// 	if agentIndex, ok := cacheData["agent_index"].(map[string]any); ok {
// 		for agentID, responseIDsInterface := range agentIndex {
// 			if responseIDs, ok := responseIDsInterface.([]string); ok {
// 				rc.agentIndex[agentID] = responseIDs
// 			}
// 		}
// 	}

// 	if taskIndex, ok := cacheData["task_index"].(map[string]any); ok {
// 		for taskID, responseIDsInterface := range taskIndex {
// 			if responseIDs, ok := responseIDsInterface.([]string); ok {
// 				rc.taskIndex[taskID] = responseIDs
// 			}
// 		}
// 	}

// 	if hashIndex, ok := cacheData["hash_index"].(map[string]any); ok {
// 		for hash, responseID := range hashIndex {
// 			if id, ok := responseID.(string); ok {
// 				rc.hashIndex[hash] = id
// 			}
// 		}
// 	}

// 	if timeIndex, ok := cacheData["time_index"].([]string); ok {
// 		rc.timeIndex = timeIndex
// 	}

// 	if accessIndex, ok := cacheData["access_index"].([]string); ok {
// 		rc.accessIndex = accessIndex
// 	}

// 	if totalSize, ok := cacheData["total_size_bytes"].(float64); ok {
// 		rc.totalSizeBytes = int64(totalSize)
// 	}

// 	// Clean expired entries after loading
// 	rc.cleanExpired()

// 	return nil
// }

// // --------------------------- Utility Functions ---------------------------

// func generateTaskHash(task map[string]any) string {
// 	// Create a normalized hash of the task for deduplication
// 	normalizedTask := make(map[string]any)

// 	// Extract key fields for hashing
// 	keyFields := []string{"description", "type", "priority", "requirements", "context"}
// 	for _, field := range keyFields {
// 		if value, exists := task[field]; exists {
// 			normalizedTask[field] = value
// 		}
// 	}

// 	taskBytes, _ := json.Marshal(normalizedTask)
// 	hash := md5.Sum(taskBytes)
// 	return fmt.Sprintf("%x", hash)
// }

// func extractRelevantContext(response map[string]any, keyLimit int) map[string]any {
// 	context := make(map[string]any)
// 	keyCount := 0

// 	// Priority order for context extraction
// 	priorityKeys := []string{"status", "result", "output", "data", "content", "message"}

// 	// Extract priority keys first
// 	for _, key := range priorityKeys {
// 		if keyCount >= keyLimit {
// 			break
// 		}
// 		if value, exists := response[key]; exists {
// 			context[key] = simplifyValue(value)
// 			keyCount++
// 		}
// 	}

// 	// Extract remaining keys up to limit
// 	for key, value := range response {
// 		if keyCount >= keyLimit {
// 			break
// 		}

// 		// Skip if already processed
// 		found := false
// 		for _, priorityKey := range priorityKeys {
// 			if key == priorityKey {
// 				found = true
// 				break
// 			}
// 		}
// 		if found {
// 			continue
// 		}

// 		// Skip error details and metadata for context
// 		if key == "error" || key == "metadata" || key == "debug" {
// 			continue
// 		}

// 		context[key] = simplifyValue(value)
// 		keyCount++
// 	}

// 	return context
// }

// func simplifyValue(value any) any {
// 	switch v := value.(type) {
// 	case string:
// 		if len(v) > 500 {
// 			return v[:500] + "...[truncated]"
// 		}
// 		return v
// 	case map[string]any:
// 		if len(v) > 10 {
// 			simplified := make(map[string]any)
// 			count := 0
// 			for k, val := range v {
// 				if count >= 10 {
// 					simplified["..."] = "additional fields truncated"
// 					break
// 				}
// 				simplified[k] = simplifyValue(val)
// 				count++
// 			}
// 			return simplified
// 		}
// 		return v
// 	case []any:
// 		if len(v) > 5 {
// 			simplified := make([]any, 5)
// 			for i := 0; i < 5; i++ {
// 				simplified[i] = simplifyValue(v[i])
// 			}
// 			return append(simplified, "...[additional items truncated]")
// 		}
// 		return v
// 	default:
// 		return v
// 	}
// }

// func generateResponseTags(task map[string]any, response map[string]any) []string {
// 	tags := make([]string, 0)

// 	// Extract tags from task
// 	if taskType, exists := task["type"].(string); exists {
// 		tags = append(tags, "task_"+taskType)
// 	}

// 	if priority, exists := task["priority"].(string); exists {
// 		tags = append(tags, "priority_"+priority)
// 	}

// 	// Extract tags from response
// 	if status, exists := response["status"].(string); exists {
// 		tags = append(tags, "status_"+status)
// 	}

// 	// Extract keywords from description
// 	if desc, exists := task["description"].(string); exists {
// 		keywords := extractKeywords(desc)
// 		for _, keyword := range keywords {
// 			tags = append(tags, "keyword_"+keyword)
// 		}
// 	}

// 	return tags
// }

// func extractKeywords(text string) []string {
// 	// Simple keyword extraction
// 	words := strings.Fields(strings.ToLower(text))
// 	keywords := make([]string, 0)

// 	// Filter out common words and keep meaningful terms
// 	stopWords := map[string]bool{
// 		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
// 		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
// 		"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
// 	}

// 	for _, word := range words {
// 		word = strings.Trim(word, ".,!?;:")
// 		if len(word) > 3 && !stopWords[word] {
// 			keywords = append(keywords, word)
// 		}
// 	}

// 	// Return first 10 keywords
// 	if len(keywords) > 10 {
// 		keywords = keywords[:10]
// 	}

// 	return keywords
// }

// func extractSearchTerms(task map[string]any) []string {
// 	terms := make([]string, 0)

// 	// Extract from description
// 	if desc, exists := task["description"].(string); exists {
// 		terms = append(terms, extractKeywords(desc)...)
// 	}

// 	// Extract from type
// 	if taskType, exists := task["type"].(string); exists {
// 		terms = append(terms, taskType)
// 	}

// 	// Extract from other string fields
// 	stringFields := []string{"category", "domain", "module", "component"}
// 	for _, field := range stringFields {
// 		if value, exists := task[field].(string); exists {
// 			terms = append(terms, value)
// 		}
// 	}

// 	return terms
// }

// func calculateSimilarity(searchTerms, tags []string, context map[string]any) float64 {
// 	if len(searchTerms) == 0 {
// 		return 0.0
// 	}

// 	matches := 0

// 	// Check tag matches
// 	for _, term := range searchTerms {
// 		for _, tag := range tags {
// 			if strings.Contains(strings.ToLower(tag), strings.ToLower(term)) {
// 				matches++
// 				break
// 			}
// 		}
// 	}

// 	// Check context matches
// 	contextStr := strings.ToLower(fmt.Sprintf("%v", context))
// 	for _, term := range searchTerms {
// 		if strings.Contains(contextStr, strings.ToLower(term)) {
// 			matches++
// 		}
// 	}

// 	return float64(matches) / float64(len(searchTerms))
// }

// func isExpired(resp *CachedResponse) bool {
// 	return time.Now().After(resp.ExpiresAt)
// }

// func deepCopyMap(original map[string]any) map[string]any {
// 	copy := make(map[string]any)
// 	for k, v := range original {
// 		switch val := v.(type) {
// 		case map[string]any:
// 			copy[k] = deepCopyMap(val)
// 		case []any:
// 			copySlice := make([]any, len(val))
// 			for i, item := range val {
// 				if itemMap, ok := item.(map[string]any); ok {
// 					copySlice[i] = deepCopyMap(itemMap)
// 				} else {
// 					copySlice[i] = item
// 				}
// 			}
// 			copy[k] = copySlice
// 		default:
// 			copy[k] = v
// 		}
// 	}
// 	return copy
// }

// func min(a, b int) int {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }

// // --------------------------- Cache Management API Functions ---------------------------

// // These functions can be added to the orchestrator capabilities

// func GetCachedResponsesForAgent(data map[string]any) map[string]any {
// 	agentID, ok := data["agent_id"].(string)
// 	if !ok || agentID == "" {
// 		return map[string]any{"error": "agent_id is required"}
// 	}

// 	limit := 10
// 	if l, ok := data["limit"].(float64); ok {
// 		limit = int(l)
// 	}

// 	return GetCachedResponsesByAgent(agentID, limit)
// }

// func SearchCachedResponses(data map[string]any) map[string]any {
// 	query, ok := data["query"].(map[string]any)
// 	if !ok {
// 		return map[string]any{"error": "query is required"}
// 	}

// 	agentID := getStringFromMap(data, "agent_id", "")

// 	similarity := 0.7
// 	if s, ok := data["similarity"].(float64); ok {
// 		similarity = s
// 	}

// 	return FindSimilarCachedResponses(query, agentID, similarity)
// }

// func PurgeCacheByAgent(data map[string]any) map[string]any {
// 	agentID, ok := data["agent_id"].(string)
// 	if !ok || agentID == "" {
// 		return map[string]any{"error": "agent_id is required"}
// 	}

// 	if globalResponseCache == nil {
// 		return map[string]any{"error": "cache not initialized"}
// 	}

// 	globalResponseCache.mutex.Lock()
// 	defer globalResponseCache.mutex.Unlock()

// 	responseIDs, exists := globalResponseCache.agentIndex[agentID]
// 	if !exists {
// 		return map[string]any{
// 			"status":   "no_responses",
// 			"agent_id": agentID,
// 			"removed":  0,
// 		}
// 	}

// 	removedCount := 0
// 	for _, responseID := range responseIDs {
// 		if _, exists := globalResponseCache.responses[responseID]; exists {
// 			globalResponseCache.removeResponse(responseID)
// 			removedCount++
// 		}
// 	}

// 	delete(globalResponseCache.agentIndex, agentID)

// 	if globalResponseCache.config.PersistToDisk {
// 		globalResponseCache.saveToDisk()
// 	}

// 	return map[string]any{
// 		"status":   "purged",
// 		"agent_id": agentID,
// 		"removed":  removedCount,
// 	}
// }

// func OptimizeCache(data map[string]any) map[string]any {
// 	if globalResponseCache == nil {
// 		return map[string]any{"error": "cache not initialized"}
// 	}

// 	globalResponseCache.mutex.Lock()
// 	defer globalResponseCache.mutex.Unlock()

// 	initialCount := len(globalResponseCache.responses)
// 	initialSize := globalResponseCache.totalSizeBytes

// 	// Run all optimization strategies
// 	globalResponseCache.cleanExpired()
// 	globalResponseCache.enforceEvictionPolicies()

// 	// Rebuild indexes to ensure consistency
// 	globalResponseCache.rebuildIndexes()

// 	finalCount := len(globalResponseCache.responses)
// 	finalSize := globalResponseCache.totalSizeBytes

// 	if globalResponseCache.config.PersistToDisk {
// 		globalResponseCache.saveToDisk()
// 	}

// 	return map[string]any{
// 		"status":            "optimized",
// 		"initial_count":     initialCount,
// 		"final_count":       finalCount,
// 		"removed_count":     initialCount - finalCount,
// 		"initial_size_mb":   float64(initialSize) / (1024 * 1024),
// 		"final_size_mb":     float64(finalSize) / (1024 * 1024),
// 		"size_reduction_mb": float64(initialSize-finalSize) / (1024 * 1024),
// 	}
// }

// func (rc *ResponseCache) rebuildIndexes() {
// 	// Clear all indexes
// 	rc.agentIndex = make(map[string][]string)
// 	rc.taskIndex = make(map[string][]string)
// 	rc.hashIndex = make(map[string]string)
// 	rc.timeIndex = make([]string, 0, len(rc.responses))
// 	rc.accessIndex = make([]string, 0, len(rc.responses))

// 	// Rebuild from responses
// 	for responseID, resp := range rc.responses {
// 		// Agent index
// 		rc.agentIndex[resp.AgentID] = append(rc.agentIndex[resp.AgentID], responseID)

// 		// Task index
// 		rc.taskIndex[resp.TaskID] = append(rc.taskIndex[resp.TaskID], responseID)

// 		// Hash index (keep most recent for each hash)
// 		if existingID, exists := rc.hashIndex[resp.TaskHash]; exists {
// 			if existing := rc.responses[existingID]; existing.Timestamp.Before(resp.Timestamp) {
// 				rc.hashIndex[resp.TaskHash] = responseID
// 			}
// 		} else {
// 			rc.hashIndex[resp.TaskHash] = responseID
// 		}

// 		// Time and access indexes
// 		rc.timeIndex = append(rc.timeIndex, responseID)
// 		rc.accessIndex = append(rc.accessIndex, responseID)
// 	}

// 	// Sort indexes
// 	sort.Slice(rc.timeIndex, func(i, j int) bool {
// 		resp1 := rc.responses[rc.timeIndex[i]]
// 		resp2 := rc.responses[rc.timeIndex[j]]
// 		return resp1.Timestamp.Before(resp2.Timestamp)
// 	})

// 	sort.Slice(rc.accessIndex, func(i, j int) bool {
// 		resp1 := rc.responses[rc.accessIndex[i]]
// 		resp2 := rc.responses[rc.accessIndex[j]]
// 		return resp1.LastAccessed.Before(resp2.LastAccessed)
// 	})
// }

// // --------------------------- Integration Functions ---------------------------

// // Call this function after any agent request to cache the response
// func CacheAgentResponseIntegration(agentID string, task map[string]any, response map[string]any) {
// 	if globalResponseCache == nil {
// 		InitializeResponseCache(nil)
// 	}

// 	taskID := getStringFromMap(task, "id", fmt.Sprintf("task_%d", time.Now().UnixNano()))
// 	CacheAgentResponse(agentID, taskID, task, response)
// }

// // Enhanced version of existing function that includes caching
// func RouteTaskEnhanced(data map[string]any) map[string]any {
// 	// First check cache
// 	result := RouteTaskWithCache(data)

// 	// If cache miss, the response is already cached by RouteTaskWithCache
// 	// If cache hit, we already have the result

// 	return result
// }

// // Function to get the best cached context for a new task
// func GetBestCachedContext(task map[string]any, agentID string) map[string]any {
// 	if globalResponseCache == nil {
// 		return map[string]any{"status": "cache_not_initialized"}
// 	}

// 	// Look for similar successful responses
// 	similarResponses := FindSimilarCachedResponses(task, agentID, 0.6)

// 	if similarResponses["status"] == "exact_match" {
// 		return map[string]any{
// 			"status":  "exact_context_found",
// 			"context": similarResponses["context"],
// 			"source":  "exact_match",
// 		}
// 	}

// 	if similarResponses["status"] == "similar_found" {
// 		matches := similarResponses["matches"].([]map[string]any)
// 		if len(matches) > 0 {
// 			// Merge contexts from top 3 similar responses
// 			mergedContext := make(map[string]any)
// 			sourcesUsed := make([]string, 0)

// 			for i, match := range matches {
// 				if i >= 3 {
// 					break
// 				}

// 				if context, exists := match["context"].(map[string]any); exists {
// 					for k, v := range context {
// 						// Prefix with similarity score to avoid conflicts
// 						prefixedKey := fmt.Sprintf("sim%.1f_%s", match["similarity"], k)
// 						mergedContext[prefixedKey] = v
// 					}
// 					sourcesUsed = append(sourcesUsed, match["response_id"].(string))
// 				}
// 			}

// 			return map[string]any{
// 				"status":       "similar_context_found",
// 				"context":      mergedContext,
// 				"source":       "similar_matches",
// 				"sources_used": sourcesUsed,
// 				"match_count":  len(matches),
// 			}
// 		}
// 	}

// 	return map[string]any{
// 		"status": "no_cached_context",
// 		"query":  task,
// 	}
// }

// // --------------------------- Cache Maintenance Functions ---------------------------

// func PerformCacheMaintenance() map[string]any {
// 	if globalResponseCache == nil {
// 		return map[string]any{"error": "cache not initialized"}
// 	}

// 	results := make(map[string]any)

// 	// Clear expired responses
// 	expiredResult := ClearExpiredResponses()
// 	results["expired_cleanup"] = expiredResult

// 	// Optimize cache
// 	optimizeResult := OptimizeCache(map[string]any{})
// 	results["optimization"] = optimizeResult

// 	// Get final statistics
// 	stats := GetCacheStatistics()
// 	results["final_stats"] = stats

// 	return map[string]any{
// 		"status":  "maintenance_complete",
// 		"results": results,
// 	}
// }

// // Schedule periodic cache maintenance (call this from your main initialization)
// func StartCacheMaintenanceScheduler() {
// 	go func() {
// 		ticker := time.NewTicker(1 * time.Hour) // Run every hour
// 		defer ticker.Stop()

// 		for {
// 			select {
// 			case <-ticker.C:
// 				if globalResponseCache != nil {
// 					// Perform light maintenance
// 					ClearExpiredResponses()

// 					// Full optimization every 6 hours
// 					if time.Now().Hour()%6 == 0 {
// 						OptimizeCache(map[string]any{})
// 					}
// 				}
// 			}
// 		}
// 	}()
// }
