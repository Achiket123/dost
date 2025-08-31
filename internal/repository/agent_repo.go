package repository

import (
	"encoding/json"
	"time"
)

type AgentType string

const (
	AgentPlanner      AgentType = "planner"
	AgentCoder        AgentType = "coder"
	AgentCritic       AgentType = "critic"
	AgentExecutor     AgentType = "executor"
	AgentKnowledge    AgentType = "knowledge"
	AgentOrchestrator AgentType = "orchestrator"
)

// MessageRole represents the role in conversation
type MessageRole string

const (
	RoleUser     MessageRole = "user"
	RoleModel    MessageRole = "model"
	RoleSystem   MessageRole = "system"
	RoleFunction MessageRole = "function"
)

type AgentCapability struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	InputTypes  []string          `json:"input_types"` // "text", "image", "audio", "video", etc.
	OutputTypes []string          `json:"output_types"`
	Parameters  map[string]string `json:"parameters"` // Expected parameters and their types
}

type AgentMetadata struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Type         AgentType `json:"type"`
	Instructions string    `json:"instructions"`
	// Capabilities   []Function        `json:"capabilities"`
	Context        map[string]any    `json:"context"`
	LastActive     time.Time         `json:"last_active"`
	MaxConcurrency int               `json:"max_concurrency"`
	Timeout        time.Duration     `json:"timeout"`
	Status         string            `json:"status"`
	Tags           []string          `json:"tags"`
	Endpoints      map[string]string `json:"endpoints"` // HTTP, gRPC, etc.
}

type Agent struct {
	Metadata     AgentMetadata `json:"metadata"`
	Capabilities []Function    `json:"capabilities"`
}

func (a *Agent) ToMap() map[string]any {
	data, _ := json.Marshal(a)
	var result map[string]any
	json.Unmarshal(data, &result)
	return result
}

type ActiveAgent interface {
	RequestAgent(contents []map[string]any) map[string]any
}
