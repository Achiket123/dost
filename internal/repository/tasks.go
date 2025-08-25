package repository

import (
	"fmt"
	"strings"
	"time"
)

// Enhanced TaskTracker with better error tracking
type TaskTracker struct {
	OriginalTask               string
	CurrentPhase               string
	CompletedSteps             []string
	PendingSteps               []string
	FilesCreated               []string
	FilesModified              []string
	CommandsRun                []string
	FailedCommands             []string
	LastAIResponse             string
	Iteration                  int
	TaskCompleted              bool
	ConsecutiveReadCalls       int
	ConsecutiveSameCommands    int
	LastActionType             string
	StuckCounter               int
	ErrorsFound                bool
	BuildAttempted             bool
	LastFunctionCall           string
	LastError                  string
	ErrorCount                 int
	LastSuccessfulCommand      string
	DependencyErrors           []string
	RequiresFixing             bool
	ProjectStructureRetrieved  bool
	EmptyResponseCount         int
	ConversationResetCount     int
	LastFunctionResponse       map[string]any
	FunctionCallsThisIteration int
	ConsecutiveFailures        int
	LastSuccessTime            time.Time
	StartTime                  time.Time
	MaxExecutionTime           time.Duration // Maximum execution time
	IdleTime                   time.Duration // Time since last meaningful action
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
	taskLower := strings.ToLower(tracker.OriginalTask)
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

	// Pause if we're clearly stuck
	if tracker.StuckCounter > 3 {
		return true
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
