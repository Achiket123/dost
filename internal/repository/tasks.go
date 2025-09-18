package repository

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
	StatusStuck      TaskStatus = "stuck"
)

type TaskTracker struct {
	// Core Task Info
	TaskID    string     `json:"taskId"`
	Title     string     `json:"title"`
	Status    TaskStatus `json:"status"`
	Priority  int        `json:"priority"` // 1-5, 5 being highest
	CreatedBy string     `json:"createdBy"`

	// Progress Tracking
	CurrentPhase   string   `json:"currentPhase"`
	CompletedSteps []string `json:"completedSteps"`
	PendingSteps   []string `json:"pendingSteps"`

	// File Operations
	FilesCreated  []string `json:"filesCreated"`
	FilesModified []string `json:"filesModified"`

	// Command Execution
	CommandsRun           []string `json:"commandsRun"`
	FailedCommands        []string `json:"failedCommands"`
	LastSuccessfulCommand string   `json:"lastSuccessfulCommand"`

	// AI Interaction
	LastAIResponse       string         `json:"lastAiResponse"`
	LastFunctionCall     string         `json:"lastFunctionCall"`
	LastFunctionResponse map[string]any `json:"lastFunctionResponse"`

	// Iteration & Loop Prevention
	Iteration                  int `json:"iteration"`
	ConsecutiveReadCalls       int `json:"consecutiveReadCalls"`
	ConsecutiveSameCommands    int `json:"consecutiveSameCommands"`
	FunctionCallsThisIteration int `json:"functionCallsThisIteration"`
	EmptyResponseCount         int `json:"emptyResponseCount"`

	// Error Handling
	LastError           string   `json:"lastError"`
	ErrorCount          int      `json:"errorCount"`
	DependencyErrors    []string `json:"dependencyErrors"`
	ConsecutiveFailures int      `json:"consecutiveFailures"`

	// State Flags
	TaskCompleted             bool `json:"taskCompleted"`
	ErrorsFound               bool `json:"errorsFound"`
	BuildAttempted            bool `json:"buildAttempted"`
	RequiresFixing            bool `json:"requiresFixing"`
	ProjectStructureRetrieved bool `json:"projectStructureRetrieved"`

	// Advanced Tracking
	LastActionType         string `json:"lastActionType"`
	ConversationResetCount int    `json:"conversationResetCount"`

	// Timing
	StartTime        time.Time      `json:"startTime"`
	LastSuccessTime  time.Time      `json:"lastSuccessTime"`
	MaxExecutionTime time.Duration  `json:"maxExecutionTime"`
	IdleTime         time.Duration  `json:"idleTime"`
	PlanningComplete bool           `json:"planningComplete"`
	ExecutionPlan    map[string]any `json:"executionPlan"`
	NextStep         string         `json:"nextStep"`
}

func (T *TaskTracker) ToObject() map[string]any {

	// Convert struct to JSON
	jsonData, err := json.Marshal(T)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return nil
	}

	// Unmarshal JSON into a map[string]any
	var taskMap map[string]any
	err = json.Unmarshal(jsonData, &taskMap)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return nil
	}
	return taskMap
}
func NewTask(params map[string]any) map[string]any {
	title, _ := params["title"].(string)
	createdBy, _ := params["createdBy"].(string)
	status, _ := params["status"].(TaskStatus)

	// Extract planning data if provided
	planningComplete, _ := params["planningComplete"].(bool)
	executionPlan, _ := params["executionPlan"].(map[string]any)
	nextStep, _ := params["nextStep"].(string)

	tracker := TaskTracker{
		TaskID:               generateTaskID(),
		Title:                title,
		Status:               status,
		Priority:             3, // Default medium priority
		CreatedBy:            createdBy,
		CurrentPhase:         "initialization",
		CompletedSteps:       make([]string, 0),
		PendingSteps:         make([]string, 0),
		FilesCreated:         make([]string, 0),
		FilesModified:        make([]string, 0),
		CommandsRun:          make([]string, 0),
		FailedCommands:       make([]string, 0),
		DependencyErrors:     make([]string, 0),
		StartTime:            time.Now(),
		LastSuccessTime:      time.Now(),
		MaxExecutionTime:     30 * time.Minute,
		LastFunctionResponse: make(map[string]any),
		PlanningComplete:     planningComplete,
		ExecutionPlan:        executionPlan,
		NextStep:             nextStep,
	}

	return map[string]any{
		"error":  nil,
		"output": tracker.ToObject(),
	}
}

// Helper function to generate unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}
func ShouldTerminate(tracker *TaskTracker) bool {
	// Time-based termination
	if time.Since(tracker.StartTime) > tracker.MaxExecutionTime {
		fmt.Printf("⏰ Maximum execution time (%v) reached\n", tracker.MaxExecutionTime)
		return true
	}

	// Task explicitly completed
	if tracker.TaskCompleted {
		return true
	}

	// Too many empty responses
	if tracker.EmptyResponseCount > 10 {
		fmt.Printf("🔇 Too many empty AI responses (%d)\n", tracker.EmptyResponseCount)
		return true
	}

	// Too many conversation resets
	if tracker.ConversationResetCount > 5 {
		fmt.Printf("🔄 Too many conversation resets (%d)\n", tracker.ConversationResetCount)
		return true
	}

	// Excessive errors without progress
	if tracker.ErrorCount > 20 && len(tracker.FilesCreated) == 0 && len(tracker.CommandsRun) == 0 {
		fmt.Printf("💥 Too many errors (%d) with no tangible progress\n", tracker.ErrorCount)
		return true
	}

	// Long idle time without meaningful actions
	if tracker.IdleTime > 3*time.Minute {
		fmt.Printf("😴 No meaningful progress for %v\n", tracker.IdleTime)
		return true
	}

	// Successful completion indicators
	if IsTaskLikelyComplete(tracker) {
		fmt.Printf("🎯 Task appears to be complete based on heuristics\n")
		tracker.TaskCompleted = true
		return true
	}

	return false
}

// Check if task is likely complete based on heuristics
func IsTaskLikelyComplete(tracker *TaskTracker) bool {
	// Basic success patterns
	if len(tracker.FilesCreated) > 0 && len(tracker.CommandsRun) > 0 && tracker.ErrorCount < 3 {
		return true
	}

	// Hello world type tasks
	taskLower := strings.ToLower(tracker.Title)
	if strings.Contains(taskLower, "hello") && strings.Contains(taskLower, "world") {
		return len(tracker.FilesCreated) > 0 || len(tracker.FilesModified) > 0
	}

	// Simple file creation tasks
	if strings.Contains(taskLower, "create") && strings.Contains(taskLower, "file") {
		return len(tracker.FilesCreated) > 0
	}

	// Build/compile tasks that succeeded
	if (strings.Contains(taskLower, "build") || strings.Contains(taskLower, "compile")) && tracker.BuildAttempted {
		return len(tracker.FailedCommands) == 0 || tracker.LastSuccessfulCommand != ""
	}

	return false
}

// Check if we should pause for user input
func ShouldPauseForUserInput(tracker *TaskTracker) bool {
	// Don't pause too early
	if tracker.Iteration < 5 {
		return false
	}

	// Pause if too many dependency errors
	if len(tracker.DependencyErrors) > 3 {
		return true
	}

	// Pause after many iterations without clear progress
	if tracker.Iteration > 15 && len(tracker.FilesCreated) == 0 && len(tracker.CommandsRun) == 0 {
		return true
	}

	return false
}

func EvaluateTaskCompletionWithErrorAnalysis(tracker *TaskTracker, returns map[string]any, originalTask string) bool {
	if tracker.TaskCompleted {
		return true
	}

	if tracker.ErrorCount > 10 && len(tracker.FilesCreated) > 1 {
		fmt.Printf("⚠️ Too many errors, but files were created. Marking as complete.\n")
		return true
	}

	// Simple task completion for basic tasks like "hello world"
	taskLower := strings.ToLower(originalTask)
	if strings.Contains(taskLower, "hello") && strings.Contains(taskLower, "world") {
		if len(tracker.FilesCreated) > 0 || len(tracker.FilesModified) > 0 {
			fmt.Printf("✅ Hello world task complete: file created/modified\n")
			return true
		}
	}

	if len(tracker.FilesCreated) >= 1 && len(tracker.CommandsRun) > 0 {
		fmt.Printf("🗂️ Task appears complete: files created and commands executed\n")
		return true
	}

	if tracker.Iteration > 20 && len(tracker.FilesCreated) > 0 && len(tracker.CompletedSteps) > 3 {
		fmt.Printf("🔥 Auto-completing task due to iteration limit with progress\n")
		return true
	}

	// Check if AI explicitly marked as complete
	outputs, ok := returns["output"].([]map[string]any)
	if ok {
		for _, output := range outputs {
			if text, ok := output["text"].(string); ok {
				textUpper := strings.ToUpper(text)
				if strings.Contains(textUpper, "TASK_COMPLETE") ||
					strings.Contains(textUpper, "TASK COMPLETED") ||
					strings.Contains(textUpper, "SUCCESSFULLY COMPLETED") {
					return true
				}
			}
		}
	}

	return false
}
