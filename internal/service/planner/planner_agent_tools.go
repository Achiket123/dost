package planner

// ------------------------- Task Creat -------------------------

// CreateTask initializes a new structured task from a user request
/*
{
  "planningComplete": true/false,
  "executionPlan": {
    "overview": "Brief description of the overall approach",
    "subtasks": [
      {
        "id": "task_1",
        "description": "Detailed task description",
        "requiredAgent": "Coder/Executor/Knowledge/Critic",
        "priority": "high/medium/low",
        "dependencies": ["task_id"],
        "estimatedComplexity": "simple/moderate/complex",
        "tools": ["tool1", "tool2"],
        "acceptance_criteria": "What defines completion"
      }
    ],
    "riskFactors": ["potential issues or challenges"],
    "successMetrics": "How to measure success"
  },
  "nextStep": "What should happen next"
}
*/
// func CreateTask(data map[string]any) map[string]any {
// 	title, _ := data["title"].(string)
// 	createdBy, _ := data["created_by"].(string)

// 	// Extract optional planning data
// 	planningComplete, _ := data["planning_complete"].(bool)
// 	nextStep, _ := data["next_step"].(string)

// 	var executionPlan map[string]any
// 	var overview, successMetrics string
// 	var subtasks, riskFactors []any

// 	// Handle execution plan if provided
// 	if execPlan, ok := data["execution_plan"].(map[string]any); ok {
// 		executionPlan = execPlan
// 		overview, _ = execPlan["overview"].(string)
// 		subtasks, _ = execPlan["subtasks"].([]any)
// 		riskFactors, _ = execPlan["risk_factors"].([]any)
// 		successMetrics, _ = execPlan["success_metrics"].(string)
// 	}

// 	if title == "" {
// 		return map[string]any{"error": "title is required"}
// 	}

// 	// Create the base task
// 	taskParams := map[string]any{
// 		"title":     title,
// 		"createdBy": createdBy,
// 		"status":    repository.StatusPending,
// 	}

// 	// Add planning data if available
// 	if planningComplete {
// 		taskParams["planningComplete"] = planningComplete
// 	}
// 	if executionPlan != nil {
// 		taskParams["executionPlan"] = executionPlan
// 	}
// 	if nextStep != "" {
// 		taskParams["nextStep"] = nextStep
// 	}

// 	task := repository.NewTask(taskParams)

// 	// Check if task creation failed
// 	if task["error"] != nil {
// 		return map[string]any{
// 			"error": task["error"],
// 		}
// 	}

// 	// Build response with planning information
// 	response := map[string]any{
// 		"status": "task_created",
// 		"task":   task["output"],
// 	}

// 	// Add planning summary if available
// 	if planningComplete && executionPlan != nil {
// 		planningInfo := map[string]any{
// 			"planning_complete": planningComplete,
// 			"overview":          overview,
// 			"subtask_count":     len(subtasks),
// 			"risk_count":        len(riskFactors),
// 			"success_metrics":   successMetrics,
// 		}

// 		if nextStep != "" {
// 			planningInfo["next_step"] = nextStep
// 		}

// 		response["planning_info"] = planningInfo
// 	}

// 	return response
// }

// // ------------------------- Task Breakdown -------------------------

// // BreakDownTask decomposes a high-level task into subtasks/steps
// func BreakDownTask(data map[string]any) map[string]any {
// 	taskID, _ := data["task_id"].(string)
// 	description, _ := data["description"].(string)

// 	if taskID == "" {
// 		return map[string]any{"error": "task_id is required"}
// 	}

// 	// For now: mock breakdown into 3 phases
// 	steps := []string{
// 		fmt.Sprintf("Analyze requirements for: %s", description),
// 		fmt.Sprintf("Design a plan for: %s", description),
// 		fmt.Sprintf("Execute and validate: %s", description),
// 	}

// 	return map[string]any{
// 		"status":  "task_broken_down",
// 		"task_id": taskID,
// 		"steps":   steps,
// 	}
// }

// // ------------------------- Task Update -------------------------

// // UpdateTaskStatus modifies the status of a task
// func UpdateTaskStatus(data map[string]any) map[string]any {
// 	taskID, _ := data["task_id"].(string)
// 	status, _ := data["status"].(string)

// 	if taskID == "" || status == "" {
// 		return map[string]any{"error": "task_id and status are required"}
// 	}

// 	return map[string]any{
// 		"status":     "task_status_updated",
// 		"task_id":    taskID,
// 		"new_status": status,
// 	}
// }

// // ------------------------- Task Tracking -------------------------

// // TrackProgress logs progress for a task
// func TrackProgress(data map[string]any) map[string]any {
// 	taskID, _ := data["task_id"].(string)
// 	phase, _ := data["phase"].(string)

// 	if taskID == "" || phase == "" {
// 		return map[string]any{"error": "task_id and phase are required"}
// 	}

// 	return map[string]any{
// 		"status":   "progress_logged",
// 		"task_id":  taskID,
// 		"phase":    phase,
// 		"loggedAt": time.Now(),
// 	}
// }

// // ------------------------- Task Evaluation -------------------------

// // EvaluateTaskCompletion checks if a task is complete
// func EvaluateTaskCompletion(data map[string]any) map[string]any {
// 	trackerData, ok := data["tracker"].(map[string]any)
// 	if !ok {
// 		return map[string]any{"error": "tracker is required"}
// 	}

// 	tracker := repository.TaskTracker{}
// 	if err := mapToStruct(trackerData, &tracker); err != nil {
// 		return map[string]any{"error": "invalid tracker data"}
// 	}

// 	isComplete := repository.IsTaskLikelyComplete(&tracker)
// 	return map[string]any{
// 		"status":      "evaluation_done",
// 		"task_id":     tracker.TaskID,
// 		"is_complete": isComplete,
// 	}
// }

// // ------------------------- Helper -------------------------

// // Simple helper to convert map -> struct
// func mapToStruct(data map[string]any, tracker *repository.TaskTracker) error {
// 	// Quick JSON marshal/unmarshal hack
// 	j, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}
// 	return json.Unmarshal(j, tracker)
// }
