package repository

import (
	"time"
)

// TaskResponse represents the structured response from a coder agent
type TaskResponse struct {
	TaskID   string         `json:"task_id"`
	AgentID  string         `json:"agent_id"`
	Status   string         `json:"status"` // "success" or "error"
	Output   interface{}    `json:"output"` // code or patch content
	Logs     []string       `json:"logs"`   // errors, retries, debug info
	Files    []FileOperation `json:"files"`  // files created/modified
	Duration time.Duration  `json:"duration"`
	Error    string         `json:"error,omitempty"`
}

// FileOperation represents a file operation performed by an agent
type FileOperation struct {
	Type     string `json:"type"`     // "create", "edit", "delete"
	Path     string `json:"path"`     // file path
	Content  string `json:"content"`  // file content (for create/edit)
	Success  bool   `json:"success"`  // whether operation succeeded
	Error    string `json:"error,omitempty"`
}

// EnhancedTask represents a task with all required fields for the multi-agent system
type EnhancedTask struct {
	TaskID        string            `json:"task_id"`
	Description   string            `json:"description"`
	RequiredFiles []string          `json:"required_files"` // files to be created or edited
	Dependencies  []string          `json:"dependencies"`   // task dependencies
	AgentType     string            `json:"agent_type"`     // preferred agent type
	Priority      int               `json:"priority"`       // 1-5, 5 being highest
	Context       map[string]any    `json:"context"`        // additional context
	Status        TaskStatus        `json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	AssignedAgent string            `json:"assigned_agent,omitempty"`
	Response      *TaskResponse     `json:"response,omitempty"`
}

// WorkflowResult represents the final result of a multi-agent workflow
type WorkflowResult struct {
	WorkflowID       string         `json:"workflow_id"`
	OriginalRequest  string         `json:"original_request"`
	Status           string         `json:"status"` // "completed", "failed", "partial"
	Tasks            []EnhancedTask `json:"tasks"`
	TaskResponses    []TaskResponse `json:"task_responses"`
	FilesCreated     []string       `json:"files_created"`
	FilesModified    []string       `json:"files_modified"`
	SuccessfulTasks  int            `json:"successful_tasks"`
	FailedTasks      int            `json:"failed_tasks"`
	TotalTasks       int            `json:"total_tasks"`
	StartTime        time.Time      `json:"start_time"`
	EndTime          time.Time      `json:"end_time"`
	TotalDuration    time.Duration  `json:"total_duration"`
	Errors           []string       `json:"errors"`
	Logs             []string       `json:"logs"`
}

// AgentExecutionContext provides isolated context for each agent
type AgentExecutionContext struct {
	AgentID       string         `json:"agent_id"`
	TaskID        string         `json:"task_id"`
	WorkflowID    string         `json:"workflow_id"`
	TaskContext   map[string]any `json:"task_context"`
	AgentType     string         `json:"agent_type"`
	CreatedAt     time.Time      `json:"created_at"`
	WorkingDir    string         `json:"working_dir"`
	AllowedFiles  []string       `json:"allowed_files"`
	Restrictions  map[string]any `json:"restrictions"`
	SharedContext map[string]any `json:"shared_context"` // Shared context for coordination between agents
}

// VerificationResult represents the result of task verification
type VerificationResult struct {
	TaskID            string            `json:"task_id"`
	Verified          bool              `json:"verified"`
	RequiredFileCheck map[string]bool   `json:"required_file_check"`
	MissingFiles      []string          `json:"missing_files"`
	ExtraFiles        []string          `json:"extra_files"`
	Issues            []string          `json:"issues"`
	Recommendations   []string          `json:"recommendations"`
	CompilationStatus string            `json:"compilation_status,omitempty"` // For C/C++ projects
	ExecutionStatus   string            `json:"execution_status,omitempty"`   // For executable projects
	VerifiedAt        time.Time         `json:"verified_at"`
}
