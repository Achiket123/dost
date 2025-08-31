package planner
 
// // --------------------------- Capability Constants ---------------------------
// const (
// 	DecomposeTaskName        = "decompose_task"
// 	CreateWorkflowName       = "create_workflow"
// 	UpdatePlanName           = "update_plan"
// 	GetNextStepName          = "get_next_step"
// 	EvaluatePlanProgressName = "evaluate_plan_progress"
// 	RequestMissingInfoName   = "request_missing_info"
// )

// // CreateTask capability
// const CreateTaskName = "create_task"
// const CreateTaskDescription = `
// Create a new structured task from user input.
// This initializes a task object with a title, creator, and default status (pending).
// Important: "title" is required.
// `

// // BreakDownTask capability
// const BreakDownTaskName = "breakdown_task"
// const BreakDownTaskDescription = `
// Break down a high-level task into smaller subtasks or actionable steps.
// Requires "task_id" and a "description" of the task.
// `

// // UpdateTaskStatus capability
// const UpdateTaskStatusName = "update_task_status"
// const UpdateTaskStatusDescription = `
// Update the status of an existing task.
// Requires both "task_id" and the new "status" string.
// `

// // TrackProgress capability
// const TrackProgressName = "track_progress"
// const TrackProgressDescription = `
// Log progress for a given task.
// Requires "task_id" and "phase" to record progress.
// `

// // EvaluateTaskCompletion capability
// const EvaluateTaskCompletionName = "evaluate_task_completion"
// const EvaluateTaskCompletionDescription = `
// Evaluate whether a task is complete based on tracker data.
// Requires a "tracker" object (map of TaskTracker fields).
// `

// // --------------------------- Planner Capability Map ---------------------------

// var PlannerCapabilities = []repository.Function{
// 	{
// 		Name:        CreateTaskName,
// 		Description: CreateTaskDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"title": {
// 					Type:        "string",
// 					Description: "Title or name of the task (required).",
// 				},
// 				"created_by": {
// 					Type:        "string",
// 					Description: "Optional: Identifier of the task creator.",
// 				},
// 				"planning_complete": {
// 					Type:        "boolean",
// 					Description: "Optional: Whether the planning phase is complete.",
// 				},
// 				"execution_plan": {
// 					Type:        "object",
// 					Description: "Optional: Detailed execution plan with subtasks, overview, and metrics.",
// 				},
// 				"next_step": {
// 					Type:        "string",
// 					Description: "Optional: Description of what should happen next.",
// 				},
// 			},
// 			Required: []string{"title"},
// 		},
// 		Service: CreateTask,
// 	},
// 	{
// 		Name:        BreakDownTaskName,
// 		Description: BreakDownTaskDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"task_id": {
// 					Type:        "string",
// 					Description: "Unique identifier of the task to break down.",
// 				},
// 				"description": {
// 					Type:        "string",
// 					Description: "Description or context of the task to break down.",
// 				},
// 			},
// 			Required: []string{"task_id", "description"},
// 		},
// 		Service: BreakDownTask,
// 	},
// 	{
// 		Name:        UpdateTaskStatusName,
// 		Description: UpdateTaskStatusDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"task_id": {
// 					Type:        "string",
// 					Description: "Unique identifier of the task to update.",
// 				},
// 				"status": {
// 					Type:        "string",
// 					Description: "The new status for the task (e.g., pending, in_progress, completed).",
// 				},
// 			},
// 			Required: []string{"task_id", "status"},
// 		},
// 		Service: UpdateTaskStatus,
// 	},
// 	{
// 		Name:        TrackProgressName,
// 		Description: TrackProgressDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"task_id": {
// 					Type:        "string",
// 					Description: "Unique identifier of the task being tracked.",
// 				},
// 				"phase": {
// 					Type:        "string",
// 					Description: "Current phase of the task being logged.",
// 				},
// 			},
// 			Required: []string{"task_id", "phase"},
// 		},
// 		Service: TrackProgress,
// 	},
// 	{
// 		Name:        EvaluateTaskCompletionName,
// 		Description: EvaluateTaskCompletionDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"tracker": {
// 					Type:        repository.TypeObject,
// 					Description: "Tracker object containing fields for evaluation.",
// 				},
// 			},
// 			Required: []string{"tracker"},
// 		},
// 		Service: EvaluateTaskCompletion,
// 	},
// }

// // --------------------------- Helper Functions ---------------------------

// func GetPlannerCapabilities() []repository.Function {
// 	return PlannerCapabilities
// }

// func GetPlannerCapabilitiesArrayMap() []map[string]any {
// 	plannerMap := make([]map[string]any, 0)
// 	for _, v := range PlannerCapabilities {
// 		plannerMap = append(plannerMap, v.ToObject())
// 	}
// 	return plannerMap
// }

// func GetPlannerCapabilitiesMap() map[string]repository.Function {
// 	plannerMap := make(map[string]repository.Function)
// 	for _, v := range PlannerCapabilities {
// 		plannerMap[v.Name] = v
// 	}
// 	return plannerMap
// }
