package orchestrator

import (
	"bufio"
	"crypto/md5"
	"dost/internal/repository"
	"dost/internal/service/coder"
	"dost/internal/service/planner"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	RegisteredAgents        = make(map[string]*repository.AgentMetadata)
	RegisteredLiveAgents    = make(map[string]any)
	globalContext           = make(map[string]any)
	agentContexts           = make(map[string]map[string]any)
	contextHistory          = make([]ContextSnapshot, 0)
	workflowStates          = make(map[string]*WorkflowState)
	taskExecutions          = make(map[string]*TaskExecution)
	mutex                   sync.RWMutex
	contextFilePath         string
	orchestratorInitialized bool
)

type ContextSnapshot struct {
	Timestamp      time.Time      `json:"timestamp"`
	GlobalContext  map[string]any `json:"global_context"`
	AgentContexts  map[string]any `json:"agent_contexts"`
	ActiveAgents   []string       `json:"active_agents"`
	WorkflowStates map[string]any `json:"workflow_states"`
	Reason         string         `json:"reason"`
}

type WorkflowState struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"`
	Steps     []WorkflowStep `json:"steps"`
	Context   map[string]any `json:"context"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type WorkflowStep struct {
	ID        string         `json:"id"`
	AgentID   string         `json:"agent_id"`
	Status    string         `json:"status"`
	Task      map[string]any `json:"task"`
	Result    map[string]any `json:"result"`
	Context   map[string]any `json:"context"`
	StartTime time.Time      `json:"start_time"`
	EndTime   *time.Time     `json:"end_time,omitempty"`
}

type SubTask struct {
	ID           string         `json:"id"`
	Description  string         `json:"description"`
	Type         string         `json:"type"`
	AgentType    string         `json:"agent_type"`
	Priority     int            `json:"priority"`
	Dependencies []string       `json:"dependencies"`
	Context      map[string]any `json:"context"`
	Status       string         `json:"status"`
}

type TaskExecution struct {
	ID             string           `json:"id"`
	Status         string           `json:"status"`
	SubTasks       []SubTask        `json:"sub_tasks"`
	Results        []map[string]any `json:"results"`
	StartTime      time.Time        `json:"start_time"`
	EndTime        *time.Time       `json:"end_time,omitempty"`
	Progress       float64          `json:"progress"`
	TotalTasks     int              `json:"total_tasks"`
	CompletedTasks int              `json:"completed_tasks"`
	FailedTasks    int              `json:"failed_tasks"`
	PendingTasks   int              `json:"pending_tasks"`
	Errors         []string         `json:"errors"`
}

type CacheConfig struct {
	MaxResponses    int           `json:"max_responses"`
	MaxAge          time.Duration `json:"max_age"`
	MaxSizeBytes    int64         `json:"max_size_bytes"`
	EvictionPolicy  string        `json:"eviction_policy"`
	PersistToDisk   bool          `json:"persist_to_disk"`
	ContextKeyLimit int           `json:"context_key_limit"`
}

var defaultCacheConfig = CacheConfig{
	MaxResponses:    500,
	MaxAge:          24 * time.Hour,
	MaxSizeBytes:    50 * 1024 * 1024,
	EvictionPolicy:  "lru",
	PersistToDisk:   true,
	ContextKeyLimit: 20,
}

type CachedResponse struct {
	ID                string         `json:"id"`
	AgentID           string         `json:"agent_id"`
	TaskID            string         `json:"task_id"`
	TaskHash          string         `json:"task_hash"`
	Timestamp         time.Time      `json:"timestamp"`
	LastAccessed      time.Time      `json:"last_accessed"`
	AccessCount       int            `json:"access_count"`
	RawResponse       map[string]any `json:"raw_response"`
	RelevantContext   map[string]any `json:"relevant_context"`
	ResponseSizeBytes int64          `json:"response_size_bytes"`
	ExpiresAt         time.Time      `json:"expires_at"`
	Tags              []string       `json:"tags"`
	Success           bool           `json:"success"`
}

type ResponseCache struct {
	responses      map[string]*CachedResponse
	agentIndex     map[string][]string
	taskIndex      map[string][]string
	hashIndex      map[string]string
	timeIndex      []string
	accessIndex    []string
	config         CacheConfig
	totalSizeBytes int64
	cacheFilePath  string
	mutex          sync.RWMutex
}

var globalResponseCache *ResponseCache

func InitializeEnhancedOrchestrator() error {
	if orchestratorInitialized {
		return nil
	}

	if err := InitializeOrchestratorContext(); err != nil {
		return fmt.Errorf("failed to initialize context: %v", err)
	}

	if err := InitializeResponseCache(nil); err != nil {
		return fmt.Errorf("failed to initialize cache: %v", err)
	}

	orchestratorInitialized = true
	return nil
}

func ProcessUserQuery(query string) map[string]any {
	mutex.Lock()
	globalContext["last_query"] = query
	globalContext["query_timestamp"] = time.Now()
	mutex.Unlock()

	requiredAgentType := determineAgentTypeFromQuery(query)

	agentNamesToRegister := []any{string(repository.AgentPlanner), string(repository.AgentCoder)}

	for _, agentName := range agentNamesToRegister {
		if !isAgentTypeRegistered(agentName.(string)) {
			registerResult := RegisterAgentsEnhanced(map[string]any{
				"agent_names": []any{agentName},
			})
			if registerResult["error"] != nil {
				return registerResult
			}
		}
	}

	task := map[string]any{
		"id":          fmt.Sprintf("query_%d", time.Now().UnixNano()),
		"description": query,
		"type":        "user_query",
		"priority":    "normal",
		"context":     globalContext,
	}

	executionID := fmt.Sprintf("exec_%d", time.Now().UnixNano())

	execution := &TaskExecution{
		ID:             executionID,
		Status:         "initiated",
		SubTasks:       []SubTask{},
		Results:        make([]map[string]any, 0),
		StartTime:      time.Now(),
		Progress:       0.0,
		TotalTasks:     0,
		CompletedTasks: 0,
		FailedTasks:    0,
		PendingTasks:   1,
		Errors:         make([]string, 0),
	}

	mutex.Lock()
	taskExecutions[executionID] = execution
	mutex.Unlock()

	go monitorAndExecuteTasks(executionID, task, requiredAgentType)

	return map[string]any{
		"status":       "task_initiated",
		"message":      "Task initiated and running in the background.",
		"execution_id": executionID,
	}
}

func monitorAndExecuteTasks(executionID string, initialTask map[string]any, requiredAgentType string) {
	mutex.RLock()
	execution := taskExecutions[executionID]
	mutex.RUnlock()

	// Handle simple, non-task queries directly without involving a planner
	if requiredAgentType == "general_responder" {
		result := map[string]any{
			"status": "completed",
			"output": "DOST is ready to assist!",
		}
		updateExecutionState(execution, result, "simple_completion")
		return
	}

	// Step 1: Subdivide the task
	subdivisionResult := SubdivideTask(map[string]any{"task": initialTask, "max_subtasks": 5})

	if subdivisionResult["error"] != nil {
		updateExecutionState(execution, subdivisionResult, "subdivision_failed")
		return
	}

	subTasks, ok := subdivisionResult["sub_tasks"].([]SubTask)
	if !ok {
		updateExecutionState(execution, map[string]any{"error": "invalid subtasks format"}, "subdivision_failed")
		return
	}

	mutex.Lock()
	execution.SubTasks = subTasks
	execution.TotalTasks = len(subTasks)
	execution.PendingTasks = len(subTasks)
	execution.Status = "in_progress"
	mutex.Unlock()
	saveOrchestratorContext()

	// Step 2: Execute tasks in a loop until all are completed
	for execution.CompletedTasks+execution.FailedTasks < execution.TotalTasks {
		var wg sync.WaitGroup
		tasksToExecute := make(chan SubTask, len(subTasks))
		resultsChan := make(chan map[string]any, len(subTasks))

		mutex.RLock()
		for _, subTask := range subTasks {
			if subTask.Status == "pending" {
				// Check dependencies
				dependenciesMet := true
				for _, depID := range subTask.Dependencies {
					depCompleted := false
					for _, completedTask := range execution.SubTasks {
						if completedTask.ID == depID && completedTask.Status == "completed" {
							depCompleted = true
							break
						}
					}
					if !depCompleted {
						dependenciesMet = false
						break
					}
				}

				if dependenciesMet {
					tasksToExecute <- subTask
				}
			}
		}
		mutex.RUnlock()
		close(tasksToExecute)

		if len(tasksToExecute) == 0 && execution.PendingTasks > 0 {
			// All pending tasks are blocked, handle this as a potential deadlock or wait state
			fmt.Printf("Orchestrator is blocked, waiting for tasks to complete...\n")
			time.Sleep(5 * time.Second)
			continue // Continue the outer loop to re-check status
		}

		// Launch worker goroutines
		numWorkers := min(len(tasksToExecute), 5) // Limit concurrency
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range tasksToExecute {
					taskResult := executeTask(task)
					resultsChan <- taskResult
				}
			}()
		}

		go func() {
			wg.Wait()
			close(resultsChan)
		}()

		// Process results as they come in
		for result := range resultsChan {
			processTaskResult(execution, result)
		}
		saveOrchestratorContext()
	}

	finalStatus := "completed"
	if execution.FailedTasks > 0 {
		finalStatus = "partial_success"
	}

	finalResult := map[string]any{
		"status":       finalStatus,
		"execution_id": execution.ID,
		"results":      execution.Results,
		"errors":       execution.Errors,
	}
	updateExecutionState(execution, finalResult, "final_completion")

}

func executeTask(subTask SubTask) map[string]any {
	fmt.Printf("Executing subtask: %s (Agent: %s)\n", subTask.Description, subTask.AgentType)
	result := routeToAgent(subTask.AgentType, map[string]any{
		"id":          subTask.ID,
		"description": subTask.Description,
		"type":        subTask.Type,
		"context":     subTask.Context,
	})
	return result
}

func processTaskResult(execution *TaskExecution, result map[string]any) {
	mutex.Lock()
	defer mutex.Unlock()

	taskID := getStringFromMap(result, "task_id", "")
	var failedSubTask SubTask
	found := false
	for i, subTask := range execution.SubTasks {
		if subTask.ID == taskID {
			execution.SubTasks[i].Status = "completed"
			failedSubTask = subTask
			found = true
			break
		}
	}

	execution.Results = append(execution.Results, result)
	if result["error"] != nil {
		execution.FailedTasks++
		execution.Errors = append(execution.Errors, fmt.Sprintf("Task %s failed: %v", taskID, result["error"]))
		// Self-correction: Send the error to the planner for a new plan
		if found {
			replanTask := map[string]any{
				"description": fmt.Sprintf("Subtask '%s' failed with error: %v. Please create a new plan to fix it.", failedSubTask.Description, result["error"]),
				"type":        "re_plan",
			}
			go monitorAndExecuteTasks(execution.ID, replanTask, "planner")
		}
	} else {
		execution.CompletedTasks++
	}
	execution.Progress = float64(execution.CompletedTasks) / float64(execution.TotalTasks)
	execution.PendingTasks = execution.TotalTasks - execution.CompletedTasks - execution.FailedTasks
}

func updateExecutionState(execution *TaskExecution, result map[string]any, reason string) {
	mutex.Lock()
	defer mutex.Unlock()

	endTime := time.Now()
	execution.EndTime = &endTime
	execution.Status = getStringFromMap(result, "status", "failed")

	if result["error"] != nil {
		execution.Errors = append(execution.Errors, getStringFromMap(result, "error", "unknown error"))
	}

	createContextSnapshot(reason)
	saveOrchestratorContext()
}

func GetEnhancedOrchestratorCapabilitiesMap() map[string]repository.Function {
	return map[string]repository.Function{
		"register_agents_enhanced": {
			Name:        "register_agents_enhanced",
			Description: "Register multiple agents for task execution and management.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"agent_names": {
						Type:        "array",
						Description: "An array of agent names to register, e.g., ['planner', 'coder'].",
						Items: &repository.Properties{
							Type: "string",
							Enum: []string{"planner", "coder"},
						},
					},
				},
				Required: []string{"agent_names"},
			},
			Service: RegisterAgentsEnhanced,
		},
		"subdivide_task": {
			Name:        "subdivide_task",
			Description: "Break down a complex or general task into smaller, manageable subtasks based on keywords and complexity analysis.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"task": {
						Type:        "object",
						Description: "The task to subdivide, including its description and type.",
						Properties: map[string]*repository.Properties{
							"id":          {Type: "string", Description: "Unique identifier for the task."},
							"description": {Type: "string", Description: "A description of the task."},
							"type":        {Type: "string", Description: "The type of task, e.g., 'user_query'."},
							"priority":    {Type: "string", Description: "The priority of the task."},
							"context":     {Type: "object", Description: "Additional context for the task."},
						},
						Required: []string{"description"},
					},
					"max_subtasks": {
						Type:        "number",
						Description: "The maximum number of subtasks to generate.",
					},
				},
				Required: []string{"task"},
			},
			Service: SubdivideTask,
		},
		"execute_task_with_subdivision": {
			Name:        "execute_task_with_subdivision",
			Description: "Executes a task by first subdividing it and then orchestrating the execution of the subtasks by the appropriate agents.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"task": {
						Type:        "object",
						Description: "The original high-level task to be executed.",
					},
					"subdivision_result": {
						Type:        "object",
						Description: "The result of a previous subdivision call, containing a list of subtasks.",
					},
				},
				Required: []string{"task"},
			},
			Service: ExecuteTaskWithSubdivision,
		},
		"parallel_execute_tasks": {
			Name:        "parallel_execute_tasks",
			Description: "Executes a list of independent tasks concurrently to improve performance. Useful for tasks that do not have dependencies on each other.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"tasks": {
						Type:        "array",
						Description: "An array of task objects to be executed in parallel.",
						Items: &repository.Properties{
							Type: "object",
							Properties: map[string]*repository.Properties{
								"id":          {Type: "string", Description: "Unique identifier for the task."},
								"description": {Type: "string", Description: "Description of the task."},
							},
							Required: []string{"description"},
						},
					},
					"max_concurrency": {
						Type:        "number",
						Description: "The maximum number of tasks to execute at the same time.",
					},
				},
				Required: []string{"tasks"},
			},
			Service: ParallelExecuteTasks,
		},
		"route_task_with_cache": {
			Name:        "route_task_with_cache",
			Description: "Routes a task to a specific agent, first checking the cache for a similar completed task to avoid redundant work.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"task": {
						Type:        "object",
						Description: "The task to route.",
						Properties: map[string]*repository.Properties{
							"id":          {Type: "string", Description: "Unique identifier for the task."},
							"description": {Type: "string", Description: "Description of the task."},
						},
						Required: []string{"description"},
					},
					"agent_id": {
						Type:        "string",
						Description: "The ID of the target agent.",
					},
				},
				Required: []string{"task", "agent_id"},
			},
			Service: RouteTaskWithCache,
		},
		"get_relevant_context_enhanced": {
			Name:        "get_relevant_context_enhanced",
			Description: "Retrieves the global and agent-specific context for a task, and also checks the cache for relevant past interactions to provide a richer context.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"task": {
						Type:        "object",
						Description: "The task for which to retrieve context.",
					},
					"agent_id": {
						Type:        "string",
						Description: "The ID of the agent requesting the context.",
					},
				},
				Required: []string{"task", "agent_id"},
			},
			Service: GetRelevantContextEnhanced,
		},
		"find_similar_cached_responses": {
			Name:        "find_similar_cached_responses",
			Description: "Searches the cache for responses to tasks that are semantically similar to the provided task, based on a configurable similarity threshold.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"task": {
						Type:        "object",
						Description: "The task to search for similar responses.",
					},
					"agent_id": {
						Type:        "string",
						Description: "The ID of the agent to filter responses by.",
					},
					"similarity": {
						Type:        "number",
						Description: "A threshold (0.0 to 1.0) for semantic similarity.",
					},
				},
				Required: []string{"task"},
			},
			Service: FindSimilarCachedResponsesWrapper,
		},
		"get_cache_statistics": {
			Name:        "get_cache_statistics",
			Description: "Provides detailed statistics about the response cache, including size, hit rate, and agent distribution.",
			Parameters: repository.Parameters{
				Type: "object",
			},
			Service: GetCacheStatisticsWrapper,
		},
		"clear_expired_responses": {
			Name:        "clear_expired_responses",
			Description: "Removes all responses from the cache that have exceeded their configured time-to-live.",
			Parameters: repository.Parameters{
				Type: "object",
			},
			Service: ClearExpiredResponsesWrapper,
		},
		"get_cached_responses_for_agent": {
			Name:        "get_cached_responses_for_agent",
			Description: "Retrieves a list of cached responses for a specific agent, ordered by recency.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"agent_id": {
						Type:        "string",
						Description: "The ID of the agent to retrieve responses for.",
					},
					"limit": {
						Type:        "number",
						Description: "The maximum number of responses to return.",
					},
					"include_raw": {
						Type:        "boolean",
						Description: "Whether to include the full raw response data.",
					},
				},
				Required: []string{"agent_id"},
			},
			Service: GetCachedResponsesForAgent,
		},
		"search_cached_responses": {
			Name:        "search_cached_responses",
			Description: "Performs a full-text search on cached responses based on a query, returning relevant matches.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"query": {
						Type:        "object",
						Description: "The search query, e.g., {'description': 'find bug'}.",
					},
					"agent_id": {
						Type:        "string",
						Description: "Optional agent ID to filter search results.",
					},
					"similarity": {
						Type:        "number",
						Description: "The minimum similarity threshold for a match.",
					},
				},
				Required: []string{"query"},
			},
			Service: SearchCachedResponses,
		},
		"purge_cache_by_agent": {
			Name:        "purge_cache_by_agent",
			Description: "Permanently removes all cached responses associated with a specific agent.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"agent_id": {
						Type:        "string",
						Description: "The ID of the agent whose cache entries will be removed.",
					},
				},
				Required: []string{"agent_id"},
			},
			Service: PurgeCacheByAgent,
		},
		"optimize_cache": {
			Name:        "optimize_cache",
			Description: "Runs cache maintenance and optimization routines, including eviction and index rebuilding.",
			Parameters: repository.Parameters{
				Type: "object",
			},
			Service: OptimizeCache,
		},
		"update_agent_context": {
			Name:        "update_agent_context",
			Description: "Updates the long-term, persistent context for a specific agent.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"agent_id": {
						Type:        "string",
						Description: "The ID of the agent whose context to update.",
					},
					"context": {
						Type:        "object",
						Description: "The new context key-value pairs to add or update.",
					},
				},
				Required: []string{"agent_id", "context"},
			},
			Service: UpdateAgentContext,
		},
		"get_agent_context": {
			Name:        "get_agent_context",
			Description: "Retrieves the current long-term context for a specific agent.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"agent_id": {
						Type:        "string",
						Description: "The ID of the agent whose context to retrieve.",
					},
				},
				Required: []string{"agent_id"},
			},
			Service: GetAgentContext,
		},
		"create_workflow": {
			Name:        "create_workflow",
			Description: "Creates a new, structured workflow with a series of steps and their dependencies.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"name": {
						Type:        "string",
						Description: "The name of the workflow.",
					},
					"description": {
						Type:        "string",
						Description: "A description of the workflow's purpose.",
					},
					"steps": {
						Type:        "array",
						Description: "An array of workflow step objects, each with a task and agent ID.",
						Items: &repository.Properties{
							Type: "object",
							Properties: map[string]*repository.Properties{
								"agent_id":     {Type: "string", Description: "The ID of the agent for this step."},
								"task":         {Type: "object", Description: "The task to be executed in this step."},
								"dependencies": {Type: "array", Description: "IDs of steps that must complete before this step starts."},
							},
							Required: []string{"agent_id", "task"},
						},
					},
					"context": {
						Type:        "object",
						Description: "Initial context for the workflow.",
					},
				},
				Required: []string{"name", "steps"},
			},
			Service: CreateWorkflow,
		},
		"execute_workflow": {
			Name:        "execute_workflow",
			Description: "Initiates the execution of a created workflow, running its steps in the specified order.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"workflow_id": {
						Type:        "string",
						Description: "The ID of the workflow to execute.",
					},
				},
				Required: []string{"workflow_id"},
			},
			Service: ExecuteWorkflow,
		},
		"get_task_execution_status": {
			Name:        "get_task_execution_status",
			Description: "Retrieves the current status and progress of a task execution, including subtasks and errors.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"execution_id": {
						Type:        "string",
						Description: "The ID of the task execution to check.",
					},
				},
				Required: []string{"execution_id"},
			},
			Service: GetTaskExecutionStatus,
		},
		"request_user_input": {
			Name:        "request_user_input",
			Description: "Prompts the user for a single line of input from the terminal and returns the response.",
			Parameters: repository.Parameters{
				Type: "object",
				Properties: map[string]*repository.Properties{
					"prompt": {
						Type:        "string",
						Description: "The message to display to the user when requesting input.",
					},
				},
				Required: []string{"prompt"},
			},
			Service: RequestUserInput,
		},
	}
}

func RequestUserInput(data map[string]any) map[string]any {
	prompt, ok := data["prompt"].(string)
	if !ok || prompt == "" {
		return map[string]any{"error": "prompt message is required"}
	}

	fmt.Printf("\n%s\n> ", prompt)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to read user input: %v", err)}
	}

	return map[string]any{
		"status": "completed",
		"output": strings.TrimSpace(input),
	}
}

func RegisterAgentsEnhanced(data map[string]any) map[string]any {
	agentNames, ok := data["agent_names"].([]any)
	if !ok {
		return map[string]any{"error": "agent_names is required"}
	}

	registeredAgents := make([]string, 0)
	var errors []string

	for _, nameInterface := range agentNames {
		agentName, ok := nameInterface.(string)
		if !ok {
			errors = append(errors, "invalid agent name type")
			continue
		}

		agentType := repository.AgentType(agentName)

		isRegistered := false
		mutex.RLock()
		for _, metadata := range RegisteredAgents {
			if metadata.Type == agentType && metadata.Status == "active" {
				isRegistered = true
				break
			}
		}
		mutex.RUnlock()

		if isRegistered {
			registeredAgents = append(registeredAgents, "Agent of type "+agentName+" already active")
			continue
		}

		var agent any
		var metadata *repository.AgentMetadata

		switch agentType {
		case repository.AgentPlanner:
			plannerAgent := planner.PlannerAgent{}
			pAgent := plannerAgent.NewAgent()
			agent = pAgent
			metadata = &pAgent.Metadata

		case repository.AgentCoder:
			coderAgent := coder.CoderAgent{}
			cAgent := coderAgent.NewAgent()
			agent = cAgent
			metadata = &cAgent.Metadata

		default:
			errors = append(errors, fmt.Sprintf("unknown agent type: %s", agentName))
			continue
		}

		mutex.Lock()
		RegisteredAgents[metadata.ID] = metadata
		RegisteredLiveAgents[metadata.ID] = agent
		agentContexts[metadata.ID] = make(map[string]any)
		mutex.Unlock()

		registeredAgents = append(registeredAgents, metadata.ID)
	}

	status := "completed"
	if len(errors) > 0 {
		if len(registeredAgents) == 0 {
			status = "failed"
		} else {
			status = "partial_success"
		}
	}

	result := map[string]any{
		"status":            status,
		"registered_agents": registeredAgents,
	}

	if len(errors) > 0 {
		result["errors"] = errors
	}

	createContextSnapshot("agents_registered")
	saveOrchestratorContext()

	return result
}

func SubdivideTask(data map[string]any) map[string]any {
	task, ok := data["task"].(map[string]any)
	if !ok {
		return map[string]any{"error": "task is required"}
	}

	maxSubtasks := 5
	if max, ok := data["max_subtasks"].(float64); ok {
		maxSubtasks = int(max)
	}

	description := getStringFromMap(task, "description", "")
	taskType := getStringFromMap(task, "type", "general")

	subTasks := make([]SubTask, 0)
	lowerDescription := strings.ToLower(description)

	hasAppOrGame := strings.Contains(lowerDescription, "make") && (strings.Contains(lowerDescription, "app") || strings.Contains(lowerDescription, "game") || strings.Contains(lowerDescription, "cli tool"))
	hasCodeKeywords := strings.Contains(lowerDescription, "code") || strings.Contains(lowerDescription, "implement") || strings.Contains(lowerDescription, "script") || strings.Contains(lowerDescription, "program") || strings.Contains(lowerDescription, "python") || strings.Contains(lowerDescription, "go") || strings.Contains(lowerDescription, "javascript") || strings.Contains(lowerDescription, "bash")

	taskID := getStringFromMap(task, "id", fmt.Sprintf("task_%d", time.Now().UnixNano()))

	if hasAppOrGame || hasCodeKeywords {
		// Step 1: Planning and Decomposition (for the planner agent)
		planSubtaskID := fmt.Sprintf("%s_plan", taskID)
		subTasks = append(subTasks, SubTask{
			ID:           planSubtaskID,
			Description:  fmt.Sprintf("Plan and design the implementation for: %s", description),
			Type:         "planning",
			AgentType:    string(repository.AgentPlanner),
			Priority:     1,
			Dependencies: []string{},
			Context:      copyMap(task),
			Status:       "pending",
		})

		// Step 2: Code Implementation (for the coder agent), dependent on the planning step
		codeSubtaskID := fmt.Sprintf("%s_code", taskID)
		subTasks = append(subTasks, SubTask{
			ID:           codeSubtaskID,
			Description:  fmt.Sprintf("Write and implement the code based on the plan for: %s", description),
			Type:         "coding",
			AgentType:    string(repository.AgentCoder),
			Priority:     2,
			Dependencies: []string{planSubtaskID},
			Context:      copyMap(task),
			Status:       "pending",
		})
	} else {
		// Fallback to a single planner task for simple queries
		subTasks = append(subTasks, SubTask{
			ID:           fmt.Sprintf("%s_single", taskID),
			Description:  description,
			Type:         taskType,
			AgentType:    string(repository.AgentPlanner),
			Priority:     1,
			Dependencies: []string{},
			Context:      copyMap(task),
			Status:       "pending",
		})
	}

	if len(subTasks) > maxSubtasks {
		subTasks = subTasks[:maxSubtasks]
	}

	return map[string]any{
		"status":               "subdivided",
		"original_task":        task,
		"sub_tasks":            subTasks,
		"subdivision_strategy": "multi_agent_sequential",
	}
}

func ExecuteTaskWithSubdivision(data map[string]any) map[string]any {
	task, ok := data["task"].(map[string]any)
	if !ok {
		return map[string]any{"error": "task is required"}
	}

	subdivisionResult, ok := data["subdivision_result"].(map[string]any)
	if !ok {
		subdivisionResult = SubdivideTask(map[string]any{"task": task, "max_subtasks": 5})
		if subdivisionResult["error"] != nil {
			return subdivisionResult
		}
	}

	subTasks, ok := subdivisionResult["sub_tasks"].([]SubTask)
	if !ok {
		return map[string]any{"error": "failed to subdivide task"}
	}

	executionID := fmt.Sprintf("exec_%d", time.Now().UnixNano())

	execution := &TaskExecution{
		ID:             executionID,
		Status:         "in_progress",
		SubTasks:       subTasks,
		Results:        make([]map[string]any, 0),
		StartTime:      time.Now(),
		Progress:       0.0,
		TotalTasks:     len(subTasks),
		CompletedTasks: 0,
		FailedTasks:    0,
		PendingTasks:   len(subTasks),
		Errors:         make([]string, 0),
	}

	mutex.Lock()
	taskExecutions[executionID] = execution
	mutex.Unlock()

	go monitorAndExecuteTasks(executionID, task, "unknown")

	return map[string]any{
		"status":       "task_initiated",
		"execution_id": executionID,
	}
}

func ParallelExecuteTasks(data map[string]any) map[string]any {
	tasks, ok := data["tasks"].([]any)
	if !ok {
		return map[string]any{"error": "tasks array is required"}
	}

	maxConcurrency := 3
	if max, ok := data["max_concurrency"].(float64); ok {
		maxConcurrency = int(max)
	}

	executionID := fmt.Sprintf("parallel_exec_%d", time.Now().UnixNano())

	taskList := make([]map[string]any, 0)
	for _, taskInterface := range tasks {
		if task, ok := taskInterface.(map[string]any); ok {
			taskList = append(taskList, task)
		}
	}

	if len(taskList) == 0 {
		return map[string]any{"error": "no valid tasks provided"}
	}

	taskChan := make(chan map[string]any, len(taskList))
	resultChan := make(chan map[string]any, len(taskList))

	for _, task := range taskList {
		taskChan <- task
	}
	close(taskChan)

	var wg sync.WaitGroup
	for i := 0; i < maxConcurrency && i < len(taskList); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				agentType := determineAgentType(task)
				result := routeToAgent(agentType, task)
				result["task_id"] = getStringFromMap(task, "id", fmt.Sprintf("task_%d", time.Now().UnixNano()))
				resultChan <- result
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	results := make([]map[string]any, 0)
	errors := make([]any, 0)
	outputs := make([]any, 0)
	successCount := 0

	for result := range resultChan {
		results = append(results, result)

		if result["error"] != nil {
			errors = append(errors, result["error"])
		} else {
			successCount++
			if output := result["output"]; output != nil {
				outputs = append(outputs, output)
			}
		}
	}

	finalResult := map[string]any{
		"success_count": successCount,
		"total_count":   len(taskList),
		"outputs":       outputs,
		"errors":        errors,
	}

	return map[string]any{
		"status":             "parallel_execution_completed",
		"execution_id":       executionID,
		"final_result":       finalResult,
		"individual_results": results,
	}
}

func CreateWorkflow(data map[string]any) map[string]any {
	_, ok := data["name"].(string)
	if !ok {
		return map[string]any{"error": "workflow name is required"}
	}

	stepsInterface, ok := data["steps"].([]any)
	if !ok {
		return map[string]any{"error": "workflow steps are required"}
	}

	workflowID := fmt.Sprintf("workflow_%d", time.Now().UnixNano())

	steps := make([]WorkflowStep, 0)
	for i, stepInterface := range stepsInterface {
		if stepData, ok := stepInterface.(map[string]any); ok {
			step := WorkflowStep{
				ID:        fmt.Sprintf("%s_step_%d", workflowID, i),
				AgentID:   getStringFromMap(stepData, "agent_id", string(repository.AgentPlanner)),
				Status:    "pending",
				Task:      stepData,
				Context:   make(map[string]any),
				StartTime: time.Time{},
			}
			steps = append(steps, step)
		}
	}

	workflow := &WorkflowState{
		ID:        workflowID,
		Status:    "created",
		Steps:     steps,
		Context:   copyMap(getMapFromMap(data, "context")),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mutex.Lock()
	workflowStates[workflowID] = workflow
	mutex.Unlock()

	saveOrchestratorContext()

	return map[string]any{
		"status":      "created",
		"workflow_id": workflowID,
		"step_count":  len(steps),
		"workflow":    workflow,
	}
}

func ExecuteWorkflow(data map[string]any) map[string]any {
	workflowID, ok := data["workflow_id"].(string)
	if !ok {
		return map[string]any{"error": "workflow_id is required"}
	}

	mutex.RLock()
	workflow, exists := workflowStates[workflowID]
	mutex.RUnlock()

	if !exists {
		return map[string]any{"error": "workflow not found"}
	}

	executionID := fmt.Sprintf("workflow_exec_%d", time.Now().UnixNano())

	mutex.Lock()
	workflow.Status = "executing"
	workflow.UpdatedAt = time.Now()
	mutex.Unlock()

	results := make([]map[string]any, 0)
	completedSteps := 0
	failedSteps := 0

	for i, step := range workflow.Steps {
		fmt.Printf("Executing workflow step %d: %s\n", i+1, getStringFromMap(step.Task, "description", ""))

		mutex.Lock()
		workflow.Steps[i].Status = "executing"
		workflow.Steps[i].StartTime = time.Now()
		mutex.Unlock()

		result := routeToAgent(step.AgentID, step.Task)

		endTime := time.Now()
		mutex.Lock()
		workflow.Steps[i].Result = result
		workflow.Steps[i].EndTime = &endTime

		if result["error"] != nil {
			workflow.Steps[i].Status = "failed"
			failedSteps++
		} else {
			workflow.Steps[i].Status = "completed"
			completedSteps++
		}

		workflow.UpdatedAt = time.Now()
		mutex.Unlock()

		results = append(results, result)
	}

	mutex.Lock()
	if failedSteps == 0 {
		workflow.Status = "completed"
	} else if completedSteps > 0 {
		workflow.Status = "partial_success"
	} else {
		workflow.Status = "failed"
	}
	workflow.UpdatedAt = time.Now()
	mutex.Unlock()

	saveOrchestratorContext()

	return map[string]any{
		"status":       "workflow.Status",
		"workflow_id":  workflowID,
		"execution_id": executionID,
		"progress": map[string]any{
			"completed": completedSteps,
			"failed":    failedSteps,
			"total":     len(workflow.Steps),
		},
		"results": results,
	}
}

func GetTaskExecutionStatus(data map[string]any) map[string]any {
	executionID, ok := data["execution_id"].(string)
	if !ok {
		return map[string]any{"error": "execution_id is required"}
	}

	mutex.RLock()
	execution, exists := taskExecutions[executionID]
	mutex.RUnlock()

	if !exists {
		return map[string]any{"error": "execution not found"}
	}

	status := map[string]any{
		"execution_id": executionID,
		"status":       execution.Status,
		"progress":     execution.Progress,
		"total_tasks":  execution.TotalTasks,
		"completed":    execution.CompletedTasks,
		"failed":       execution.FailedTasks,
		"pending":      execution.PendingTasks,
		"start_time":   execution.StartTime.Format(time.RFC3339),
	}

	if execution.EndTime != nil {
		status["end_time"] = execution.EndTime.Format(time.RFC3339)
		duration := execution.EndTime.Sub(execution.StartTime)
		status["duration"] = duration.String()
	}

	if len(execution.Errors) > 0 {
		status["errors"] = execution.Errors
	}

	if execution.Status == "in_progress" {
		if execution.Progress > 0 {
			elapsed := time.Since(execution.StartTime)
			estimated := time.Duration(float64(elapsed) / execution.Progress)
			estimatedCompletion := execution.StartTime.Add(estimated)
			status["estimated_completion"] = estimatedCompletion.Format(time.RFC3339)
		}
	}

	return status
}

func FindSimilarCachedResponsesWrapper(data map[string]any) map[string]any {
	task, ok := data["task"].(map[string]any)
	if !ok {
		return map[string]any{"error": "task is required"}
	}

	agentID := getStringFromMap(data, "agent_id", "")
	similarity := 0.8
	if s, ok := data["similarity"].(float64); ok {
		similarity = s
	}

	return FindSimilarCachedResponses(task, agentID, similarity)
}

func GetCacheStatisticsWrapper(data map[string]any) map[string]any {
	return GetCacheStatistics()
}

func ClearExpiredResponsesWrapper(data map[string]any) map[string]any {
	return ClearExpiredResponses()
}

func InitializeOrchestratorContext() error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting working directory: %v", err)
	}

	dostDir := filepath.Join(workingDir, ".dost")
	if err := os.MkdirAll(dostDir, 0755); err != nil {
		return fmt.Errorf("error creating .dost directory: %v", err)
	}

	contextFilePath = filepath.Join(dostDir, "orchestrator_context.json")
	return loadOrchestratorContext()
}

func InitializeResponseCache(config *CacheConfig) error {
	if config == nil {
		config = &defaultCacheConfig
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting working directory: %v", err)
	}

	dostDir := filepath.Join(workingDir, ".dost")
	if err := os.MkdirAll(dostDir, 0755); err != nil {
		return fmt.Errorf("error creating .dost directory: %v", err)
	}

	cacheFilePath := filepath.Join(dostDir, "response_cache.json")

	globalResponseCache = &ResponseCache{
		responses:     make(map[string]*CachedResponse),
		agentIndex:    make(map[string][]string),
		taskIndex:     make(map[string][]string),
		hashIndex:     make(map[string]string),
		timeIndex:     make([]string, 0),
		accessIndex:   make([]string, 0),
		config:        *config,
		cacheFilePath: cacheFilePath,
	}

	return globalResponseCache.loadFromDisk()
}

func CacheAgentResponse(agentID, taskID string, task map[string]any, response map[string]any) string {
	if globalResponseCache == nil {
		InitializeResponseCache(nil)
	}

	taskHash := generateTaskHash(task)

	globalResponseCache.mutex.RLock()
	if existingID, exists := globalResponseCache.hashIndex[taskHash]; exists {
		if cachedResp, found := globalResponseCache.responses[existingID]; found && !isExpired(cachedResp) {
			cachedResp.LastAccessed = time.Now()
			cachedResp.AccessCount++
			globalResponseCache.mutex.RUnlock()
			return existingID
		}
	}
	globalResponseCache.mutex.RUnlock()

	responseID := fmt.Sprintf("%s_%s_%d", agentID, taskID, time.Now().UnixNano())

	rawResponseBytes, _ := json.Marshal(response)
	responseSize := int64(len(rawResponseBytes))

	relevantContext := extractRelevantContext(response, globalResponseCache.config.ContextKeyLimit)

	cachedResponse := &CachedResponse{
		ID:                responseID,
		AgentID:           agentID,
		TaskID:            taskID,
		TaskHash:          taskHash,
		Timestamp:         time.Now(),
		LastAccessed:      time.Now(),
		AccessCount:       1,
		RawResponse:       deepCopyMap(response),
		RelevantContext:   relevantContext,
		ResponseSizeBytes: responseSize,
		ExpiresAt:         time.Now().Add(globalResponseCache.config.MaxAge),
		Tags:              generateResponseTags(task, response),
		Success:           response["error"] == nil,
	}

	globalResponseCache.mutex.Lock()
	defer globalResponseCache.mutex.Unlock()

	globalResponseCache.responses[responseID] = cachedResponse
	globalResponseCache.totalSizeBytes += responseSize

	globalResponseCache.agentIndex[agentID] = append(globalResponseCache.agentIndex[agentID], responseID)
	globalResponseCache.taskIndex[taskID] = append(globalResponseCache.taskIndex[taskID], responseID)
	globalResponseCache.hashIndex[taskHash] = responseID

	globalResponseCache.insertIntoTimeIndex(responseID)
	globalResponseCache.insertIntoAccessIndex(responseID)

	globalResponseCache.enforceEvictionPolicies()

	if globalResponseCache.config.PersistToDisk {
		globalResponseCache.saveToDisk()
	}

	return responseID
}

func GetCachedResponse(responseID string, includeRaw bool) map[string]any {
	if globalResponseCache == nil {
		return map[string]any{"error": "cache not initialized"}
	}

	globalResponseCache.mutex.RLock()
	cachedResp, exists := globalResponseCache.responses[responseID]
	globalResponseCache.mutex.RUnlock()

	if !exists {
		return map[string]any{"error": "response not found in cache"}
	}

	if isExpired(cachedResp) {
		return map[string]any{"error": "cached response expired"}
	}

	globalResponseCache.mutex.Lock()
	cachedResp.LastAccessed = time.Now()
	cachedResp.AccessCount++
	globalResponseCache.mutex.Unlock()

	result := map[string]any{
		"status":           "cache_hit",
		"response_id":      responseID,
		"agent_id":         cachedResp.AgentID,
		"task_id":          cachedResp.TaskID,
		"timestamp":        cachedResp.Timestamp,
		"relevant_context": cachedResp.RelevantContext,
		"access_count":     cachedResp.AccessCount,
		"success":          cachedResp.Success,
	}

	if includeRaw {
		result["raw_response"] = cachedResp.RawResponse
	}

	return result
}

func RouteTaskWithCache(data map[string]any) map[string]any {
	task, ok := data["task"]
	if !ok {
		return map[string]any{"error": "task is required"}
	}

	agentID, ok := data["agent_id"].(string)
	if !ok {
		return map[string]any{"error": "agent_id is required"}
	}

	taskMap := task.(map[string]any)
	taskID := getStringFromMap(taskMap, "id", fmt.Sprintf("task_%d", time.Now().UnixNano()))

	if globalResponseCache != nil {
		similarResponses := FindSimilarCachedResponses(taskMap, agentID, 1.0)
		if similarResponses["status"] == "exact_match" {
			cachedResponseID := similarResponses["response_id"].(string)
			cachedData := GetCachedResponse(cachedResponseID, true)

			if cachedData["error"] == nil {
				result := cachedData["raw_response"].(map[string]any)
				result["cache_hit"] = true
				result["cached_response_id"] = cachedResponseID
				return result
			}
		}
	}

	result := executeTaskWithAgent(agentID, taskMap, map[string]any{})

	if globalResponseCache != nil && result["error"] == nil {
		responseID := CacheAgentResponse(agentID, taskID, taskMap, result)
		result["cached_response_id"] = responseID
		result["cache_hit"] = false
	}

	return result
}

func GetRelevantContextEnhanced(data map[string]any) map[string]any {
	task, ok := data["task"].(map[string]any)
	if !ok {
		return map[string]any{"error": "task is required"}
	}

	agentID := getStringFromMap(data, "agent_id", "")

	mutex.RLock()
	result := map[string]any{
		"status":         "context_found",
		"task":           task,
		"global_context": copyMap(globalContext),
	}

	if agentID != "" {
		if agentContext, exists := agentContexts[agentID]; exists {
			result["agent_context"] = copyMap(agentContext)
		}
	}
	mutex.RUnlock()

	if globalResponseCache != nil {
		similarResponses := FindSimilarCachedResponses(task, agentID, 0.8)
		if similarResponses["status"] == "exact_match" || similarResponses["status"] == "similar_found" {
			if similarResponses["status"] == "exact_match" {
				if context, exists := similarResponses["context"].(map[string]any); exists {
					if globalCtx, ok := result["global_context"].(map[string]any); ok {
						for k, v := range context {
							globalCtx["cached_"+k] = v
						}
					}
					result["cache_hit"] = true
					result["cache_source"] = "exact_match"
				}
			} else if matches, exists := similarResponses["matches"].([]map[string]any); exists && len(matches) > 0 {
				bestMatch := matches[0]
				if context, exists := bestMatch["context"].(map[string]any); exists {
					if globalCtx, ok := result["global_context"].(map[string]any); ok {
						for k, v := range context {
							globalCtx["similar_cached_"+k] = v
						}
					}
					result["cache_hit"] = true
					result["cache_source"] = "similar_match"
					result["similarity"] = bestMatch["similarity"]
				}
			}
		}
	}

	relatedContexts := findRelatedContexts(task)
	if len(relatedContexts) > 0 {
		result["related_contexts"] = relatedContexts
	}

	return result
}

func FindSimilarCachedResponses(task map[string]any, agentID string, similarity float64) map[string]any {
	if globalResponseCache == nil {
		return map[string]any{"error": "cache not initialized"}
	}

	taskHash := generateTaskHash(task)

	globalResponseCache.mutex.RLock()
	defer globalResponseCache.mutex.RUnlock()

	if existingID, exists := globalResponseCache.hashIndex[taskHash]; exists {
		if cachedResp, found := globalResponseCache.responses[existingID]; found && !isExpired(cachedResp) {
			return map[string]any{
				"status":      "exact_match",
				"response_id": existingID,
				"similarity":  1.0,
				"context":     cachedResp.RelevantContext,
			}
		}
	}

	candidates := make([]struct {
		responseID string
		similarity float64
	}, 0)

	searchTerms := extractSearchTerms(task)

	for responseID, cachedResp := range globalResponseCache.responses {
		if isExpired(cachedResp) {
			continue
		}

		if agentID != "" && cachedResp.AgentID != agentID {
			continue
		}

		sim := calculateSimilarity(searchTerms, cachedResp.Tags, cachedResp.RelevantContext)
		if sim >= similarity {
			candidates = append(candidates, struct {
				responseID string
				similarity float64
			}{responseID, sim})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].similarity > candidates[j].similarity
	})

	if len(candidates) == 0 {
		return map[string]any{
			"status": "no_similar_responses",
			"query":  task,
		}
	}

	matches := make([]map[string]any, 0, min(len(candidates), 5))
	for i, candidate := range candidates {
		if i >= 5 {
			break
		}

		if cachedResp, exists := globalResponseCache.responses[candidate.responseID]; exists {
			matches = append(matches, map[string]any{
				"response_id": cachedResp.ID,
				"agent_id":    cachedResp.AgentID,
				"similarity":  candidate.similarity,
				"context":     cachedResp.RelevantContext,
				"timestamp":   cachedResp.Timestamp,
			})
		}
	}

	return map[string]any{
		"status":  "similar_found",
		"matches": matches,
		"query":   task,
	}
}

func ClearExpiredResponses() map[string]any {
	if globalResponseCache == nil {
		return map[string]any{"error": "cache not initialized"}
	}

	globalResponseCache.mutex.Lock()
	defer globalResponseCache.mutex.Unlock()

	expiredCount := 0
	now := time.Now()

	for responseID, cachedResp := range globalResponseCache.responses {
		if cachedResp.ExpiresAt.Before(now) {
			globalResponseCache.removeFromIndexes(responseID)
			globalResponseCache.totalSizeBytes -= cachedResp.ResponseSizeBytes
			delete(globalResponseCache.responses, responseID)
			expiredCount++
		}
	}

	if globalResponseCache.config.PersistToDisk {
		globalResponseCache.saveToDisk()
	}

	return map[string]any{
		"status":        "cleaned",
		"expired_count": expiredCount,
		"remaining":     len(globalResponseCache.responses),
		"total_size_mb": float64(globalResponseCache.totalSizeBytes) / (1024 * 1024),
	}
}

func GetCacheStatistics() map[string]any {
	if globalResponseCache == nil {
		return map[string]any{"error": "cache not initialized"}
	}

	globalResponseCache.mutex.RLock()
	defer globalResponseCache.mutex.RUnlock()

	agentStats := make(map[string]int)
	successCount := 0

	for _, cachedResp := range globalResponseCache.responses {
		agentStats[cachedResp.AgentID]++
		if cachedResp.Success {
			successCount++
		}
	}

	successRate := 0.0
	if len(globalResponseCache.responses) > 0 {
		successRate = float64(successCount) / float64(len(globalResponseCache.responses))
	}

	return map[string]any{
		"total_responses":    len(globalResponseCache.responses),
		"total_size_bytes":   globalResponseCache.totalSizeBytes,
		"total_size_mb":      float64(globalResponseCache.totalSizeBytes) / (1024 * 1024),
		"success_rate":       successRate,
		"agent_distribution": agentStats,
		"config":             globalResponseCache.config,
		"oldest_response":    globalResponseCache.getOldestResponseTime(),
		"newest_response":    globalResponseCache.getNewestResponseTime(),
	}
}

func routeToAgent(agentType string, task map[string]any) map[string]any {
	mutex.RLock()
	var selectedAgentID string
	for agentID, metadata := range RegisteredAgents {
		if string(metadata.Type) == agentType && metadata.Status == "active" {
			selectedAgentID = agentID
			break
		}
	}
	mutex.RUnlock()

	if selectedAgentID == "" {
		return map[string]any{
			"error": fmt.Sprintf("no active agent found for type: %s", agentType),
		}
	}

	contextResult := GetRelevantContextEnhanced(map[string]any{
		"task":     task,
		"agent_id": selectedAgentID,
	})

	return executeTaskWithAgent(selectedAgentID, task, contextResult)
}

func determineAgentType(task map[string]any) string {
	description := strings.ToLower(getStringFromMap(task, "description", ""))
	taskType := strings.ToLower(getStringFromMap(task, "type", ""))

	codeKeywords := []string{"code", "implement", "develop", "build", "function", "class", "module", "debug", "fix", "program"}
	planKeywords := []string{"plan", "design", "analyze", "strategy", "organize", "structure", "approach", "concept", "replan", "fix", "error"}

	for _, keyword := range codeKeywords {
		if strings.Contains(description, keyword) || strings.Contains(taskType, keyword) {
			return string(repository.AgentCoder)
		}
	}

	for _, keyword := range planKeywords {
		if strings.Contains(description, keyword) || strings.Contains(taskType, keyword) {
			return string(repository.AgentPlanner)
		}
	}

	return string(repository.AgentPlanner)
}

func getSubdivisionStrategy(hasPlanningWork, hasCodeWork, hasMultipleSteps bool) string {
	strategies := make([]string, 0)

	if hasPlanningWork {
		strategies = append(strategies, "planning_first")
	}
	if hasCodeWork {
		strategies = append(strategies, "code_implementation")
	}
	if hasMultipleSteps {
		strategies = append(strategies, "sequential_steps")
	}

	if len(strategies) == 0 {
		return "single_agent"
	}

	return strings.Join(strategies, "+")
}

func getMapFromMap(m map[string]any, key string) map[string]any {
	if val, ok := m[key].(map[string]any); ok {
		return val
	}
	return make(map[string]any)
}

func (rc *ResponseCache) enforceEvictionPolicies() {
	if rc.totalSizeBytes > rc.config.MaxSizeBytes {
		rc.evictBySize()
	}

	if len(rc.responses) > rc.config.MaxResponses {
		rc.evictByPolicy()
	}

	rc.cleanExpired()
}

func (rc *ResponseCache) evictByPolicy() {
	toRemove := len(rc.responses) - rc.config.MaxResponses
	if toRemove <= 0 {
		return
	}

	var idsToRemove []string

	switch rc.config.EvictionPolicy {
	case "lru":
		sort.Slice(rc.accessIndex, func(i, j int) bool {
			resp1 := rc.responses[rc.accessIndex[i]]
			resp2 := rc.responses[rc.accessIndex[j]]
			return resp1.LastAccessed.Before(resp2.LastAccessed)
		})
		idsToRemove = rc.accessIndex[:toRemove]

	case "time":
		sort.Slice(rc.timeIndex, func(i, j int) bool {
			resp1 := rc.responses[rc.timeIndex[i]]
			resp2 := rc.responses[rc.timeIndex[j]]
			return resp1.Timestamp.Before(resp2.Timestamp)
		})
		idsToRemove = rc.timeIndex[:toRemove]

	case "size":
		type sizeEntry struct {
			id   string
			size int64
		}

		sizeList := make([]sizeEntry, 0, len(rc.responses))
		for id, resp := range rc.responses {
			sizeList = append(sizeList, sizeEntry{id, resp.ResponseSizeBytes})
		}

		sort.Slice(sizeList, func(i, j int) bool {
			return sizeList[i].size > sizeList[j].size
		})

		for i := 0; i < toRemove && i < len(sizeList); i++ {
			idsToRemove = append(idsToRemove, sizeList[i].id)
		}
	}

	for _, id := range idsToRemove {
		rc.removeResponse(id)
	}
}

func (rc *ResponseCache) evictBySize() {
	targetSize := rc.config.MaxSizeBytes * 8 / 10

	type responseScore struct {
		id    string
		score float64
	}

	scores := make([]responseScore, 0, len(rc.responses))
	now := time.Now()

	for id, resp := range rc.responses {
		ageHours := now.Sub(resp.LastAccessed).Hours()
		accessScore := float64(resp.AccessCount)
		timeScore := 1.0 / (1.0 + ageHours/24.0)
		score := accessScore * timeScore
		scores = append(scores, responseScore{id, score})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})

	for _, entry := range scores {
		if rc.totalSizeBytes <= targetSize {
			break
		}
		rc.removeResponse(entry.id)
	}
}

func (rc *ResponseCache) cleanExpired() {
	now := time.Now()
	expiredIDs := make([]string, 0)

	for id, resp := range rc.responses {
		if resp.ExpiresAt.Before(now) {
			expiredIDs = append(expiredIDs, id)
		}
	}

	for _, id := range expiredIDs {
		rc.removeResponse(id)
	}
}

func (rc *ResponseCache) removeResponse(responseID string) {
	if resp, exists := rc.responses[responseID]; exists {
		rc.totalSizeBytes -= resp.ResponseSizeBytes
		delete(rc.responses, responseID)
		rc.removeFromIndexes(responseID)
	}
}

func (rc *ResponseCache) removeFromIndexes(responseID string) {
	for agentID, responseIDs := range rc.agentIndex {
		for i, id := range responseIDs {
			if id == responseID {
				rc.agentIndex[agentID] = append(responseIDs[:i], responseIDs[i+1:]...)
				break
			}
		}
	}

	for taskID, responseIDs := range rc.taskIndex {
		for i, id := range responseIDs {
			if id == responseID {
				rc.taskIndex[taskID] = append(responseIDs[:i], responseIDs[i+1:]...)
				break
			}
		}
	}

	for hash, id := range rc.hashIndex {
		if id == responseID {
			delete(rc.hashIndex, hash)
			break
		}
	}

	rc.removeFromSlice(&rc.timeIndex, responseID)
	rc.removeFromSlice(&rc.accessIndex, responseID)
}

func (rc *ResponseCache) insertIntoTimeIndex(responseID string) {
	resp := rc.responses[responseID]
	insertPos := sort.Search(len(rc.timeIndex), func(i int) bool {
		other := rc.responses[rc.timeIndex[i]]
		return other.Timestamp.After(resp.Timestamp)
	})

	rc.timeIndex = append(rc.timeIndex, "")
	copy(rc.timeIndex[insertPos+1:], rc.timeIndex[insertPos:])
	rc.timeIndex[insertPos] = responseID
}

func (rc *ResponseCache) insertIntoAccessIndex(responseID string) {
	resp := rc.responses[responseID]
	insertPos := sort.Search(len(rc.accessIndex), func(i int) bool {
		other := rc.responses[rc.accessIndex[i]]
		return other.LastAccessed.After(resp.LastAccessed)
	})

	rc.accessIndex = append(rc.accessIndex, "")
	copy(rc.accessIndex[insertPos+1:], rc.accessIndex[insertPos:])
	rc.accessIndex[insertPos] = responseID
}

func (rc *ResponseCache) removeFromSlice(slice *[]string, item string) {
	for i, v := range *slice {
		if v == item {
			*slice = append((*slice)[:i], (*slice)[i+1:]...)
			break
		}
	}
}

func (rc *ResponseCache) getOldestResponseTime() *time.Time {
	if len(rc.responses) == 0 {
		return nil
	}

	var oldest *time.Time
	for _, resp := range rc.responses {
		if oldest == nil || resp.Timestamp.Before(*oldest) {
			t := resp.Timestamp
			oldest = &t
		}
	}
	return oldest
}

func (rc *ResponseCache) getNewestResponseTime() *time.Time {
	if len(rc.responses) == 0 {
		return nil
	}

	var newest *time.Time
	for _, resp := range rc.responses {
		if newest == nil || resp.Timestamp.After(*newest) {
			t := resp.Timestamp
			newest = &t
		}
	}
	return newest
}

func (rc *ResponseCache) saveToDisk() error {
	if rc.cacheFilePath == "" {
		return fmt.Errorf("cache file path not initialized")
	}

	cacheData := map[string]any{
		"responses":        rc.responses,
		"agent_index":      rc.agentIndex,
		"task_index":       rc.taskIndex,
		"hash_index":       rc.hashIndex,
		"time_index":       rc.timeIndex,
		"access_index":     rc.accessIndex,
		"config":           rc.config,
		"total_size_bytes": rc.totalSizeBytes,
		"last_saved":       time.Now(),
	}

	data, err := json.MarshalIndent(cacheData, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling cache data: %v", err)
	}

	return os.WriteFile(rc.cacheFilePath, data, 0644)
}

func (rc *ResponseCache) loadFromDisk() error {
	if rc.cacheFilePath == "" {
		return fmt.Errorf("cache file path not initialized")
	}

	if _, err := os.Stat(rc.cacheFilePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(rc.cacheFilePath)
	if err != nil {
		return fmt.Errorf("error reading cache file: %v", err)
	}

	if len(data) == 0 {
		return nil
	}

	var cacheData map[string]any
	if err := json.Unmarshal(data, &cacheData); err != nil {
		return fmt.Errorf("error unmarshaling cache data: %v", err)
	}

	if responses, ok := cacheData["responses"].(map[string]any); ok {
		for id, respData := range responses {
			if respBytes, err := json.Marshal(respData); err == nil {
				var cachedResp CachedResponse
				if err := json.Unmarshal(respBytes, &cachedResp); err == nil {
					rc.responses[id] = &cachedResp
				}
			}
		}
	}

	if agentIndex, ok := cacheData["agent_index"].(map[string]any); ok {
		for agentID, responseIDsInterface := range agentIndex {
			if responseIDsArray, ok := responseIDsInterface.([]any); ok {
				responseIDs := make([]string, 0, len(responseIDsArray))
				for _, idInterface := range responseIDsArray {
					if id, ok := idInterface.(string); ok {
						responseIDs = append(responseIDs, id)
					}
				}
				rc.agentIndex[agentID] = responseIDs
			}
		}
	}

	if taskIndex, ok := cacheData["task_index"].(map[string]any); ok {
		for taskID, responseIDsInterface := range taskIndex {
			if responseIDsArray, ok := responseIDsInterface.([]any); ok {
				responseIDs := make([]string, 0, len(responseIDsArray))
				for _, idInterface := range responseIDsArray {
					if id, ok := idInterface.(string); ok {
						responseIDs = append(responseIDs, id)
					}
				}
				rc.taskIndex[taskID] = responseIDs
			}
		}
	}

	if hashIndex, ok := cacheData["hash_index"].(map[string]any); ok {
		for hash, responseID := range hashIndex {
			if id, ok := responseID.(string); ok {
				rc.hashIndex[hash] = id
			}
		}
	}

	if timeIndex, ok := cacheData["time_index"].([]any); ok {
		rc.timeIndex = make([]string, 0, len(timeIndex))
		for _, idInterface := range timeIndex {
			if id, ok := idInterface.(string); ok {
				rc.timeIndex = append(rc.timeIndex, id)
			}
		}
	}

	if accessIndex, ok := cacheData["access_index"].([]any); ok {
		rc.accessIndex = make([]string, 0, len(accessIndex))
		for _, idInterface := range accessIndex {
			if id, ok := idInterface.(string); ok {
				rc.accessIndex = append(rc.accessIndex, id)
			}
		}
	}

	if totalSize, ok := cacheData["total_size_bytes"].(float64); ok {
		rc.totalSizeBytes = int64(totalSize)
	}

	rc.cleanExpired()
	return nil
}

func saveOrchestratorContext() error {
	if contextFilePath == "" {
		return fmt.Errorf("context file path not initialized")
	}

	contextData := map[string]any{
		"global_context":    globalContext,
		"agent_contexts":    agentContexts,
		"registered_agents": RegisteredAgents,
		"workflow_states":   workflowStates,
		"context_history":   contextHistory,
		"last_updated":      time.Now(),
	}

	data, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling context data: %v", err)
	}

	return os.WriteFile(contextFilePath, data, 0644)
}

func loadOrchestratorContext() error {
	if contextFilePath == "" {
		return fmt.Errorf("context file path not initialized")
	}

	if _, err := os.Stat(contextFilePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(contextFilePath)
	if err != nil {
		return fmt.Errorf("error reading context file: %v", err)
	}

	if len(data) == 0 {
		return nil
	}

	var contextData map[string]any
	if err := json.Unmarshal(data, &contextData); err != nil {
		return fmt.Errorf("error unmarshaling context data: %v", err)
	}

	if gc, ok := contextData["global_context"].(map[string]any); ok {
		globalContext = gc
	}

	if ac, ok := contextData["agent_contexts"].(map[string]any); ok {
		agentContexts = make(map[string]map[string]any)
		for k, v := range ac {
			if ctx, ok := v.(map[string]any); ok {
				agentContexts[k] = ctx
			}
		}
	}

	if regAgents, ok := contextData["registered_agents"].(map[string]any); ok {
		for k, v := range regAgents {
			if metadata, ok := v.(map[string]any); ok {
				agentMetadata := &repository.AgentMetadata{
					ID:             metadata["id"].(string),
					Name:           metadata["name"].(string),
					Version:        metadata["version"].(string),
					Type:           repository.AgentType(metadata["type"].(string)),
					Instructions:   metadata["instructions"].(string),
					MaxConcurrency: int(metadata["max_concurrency"].(float64)),
					Timeout:        time.Duration(metadata["timeout"].(float64)),
					Tags:           getStringSliceFromMap(metadata, "tags"),
					Endpoints:      getMapStringFromMap(metadata, "endpoints"),
					Status:         metadata["status"].(string),
					LastActive:     parseTime(metadata["last_active"]),
				}
				if ctx, ok := metadata["context"].(map[string]any); ok {
					agentMetadata.Context = ctx
				}
				RegisteredAgents[k] = agentMetadata
			}
		}
	}

	if ch, ok := contextData["context_history"].([]any); ok {
		contextHistory = make([]ContextSnapshot, 0, len(ch))
		for _, v := range ch {
			if snapshotMap, ok := v.(map[string]any); ok {
				snapshot := ContextSnapshot{
					Timestamp:      parseTime(snapshotMap["timestamp"]),
					GlobalContext:  getMapFromMap(snapshotMap, "global_context"),
					AgentContexts:  getMapFromMap(snapshotMap, "agent_contexts"),
					ActiveAgents:   getStringSliceFromMap(snapshotMap, "active_agents"),
					WorkflowStates: getMapFromMap(snapshotMap, "workflow_states"),
					Reason:         getStringFromMap(snapshotMap, "reason", ""),
				}
				contextHistory = append(contextHistory, snapshot)
			}
		}
	}

	return nil
}

func getMapStringFromMap(m map[string]any, key string) map[string]string {
	if val, ok := m[key].(map[string]string); ok {
		return val
	}
	if val, ok := m[key].(map[string]any); ok {
		result := make(map[string]string)
		for k, v := range val {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
		return result
	}
	return make(map[string]string)
}

func parseTime(t any) time.Time {
	if tStr, ok := t.(string); ok {
		parsed, err := time.Parse(time.RFC3339, tStr)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func createContextSnapshot(reason string) {
	snapshot := ContextSnapshot{
		Timestamp:      time.Now(),
		GlobalContext:  copyMap(globalContext),
		AgentContexts:  make(map[string]any),
		ActiveAgents:   make([]string, 0),
		WorkflowStates: make(map[string]any),
		Reason:         reason,
	}

	for agentID, ctx := range agentContexts {
		snapshot.AgentContexts[agentID] = copyMap(ctx)
	}

	for agentID, agent := range RegisteredAgents {
		if agent.Status == "active" {
			snapshot.ActiveAgents = append(snapshot.ActiveAgents, agentID)
		}
	}

	for workflowID, state := range workflowStates {
		snapshot.WorkflowStates[workflowID] = map[string]any{
			"id":         state.ID,
			"status":     state.Status,
			"steps":      len(state.Steps),
			"created_at": state.CreatedAt,
			"updated_at": state.UpdatedAt,
		}
	}

	contextHistory = append(contextHistory, snapshot)

	if len(contextHistory) > 50 {
		contextHistory = contextHistory[len(contextHistory)-50:]
	}
}

func generateTaskHash(task map[string]any) string {
	normalizedTask := make(map[string]any)
	keyFields := []string{"description", "type", "priority", "requirements", "context"}

	for _, field := range keyFields {
		if value, exists := task[field]; exists {
			normalizedTask[field] = value
		}
	}

	taskBytes, _ := json.Marshal(normalizedTask)
	hash := md5.Sum(taskBytes)
	return fmt.Sprintf("%x", hash)
}

func extractRelevantContext(response map[string]any, keyLimit int) map[string]any {
	context := make(map[string]any)
	keyCount := 0

	priorityKeys := []string{"status", "result", "output", "data", "content", "message"}

	for _, key := range priorityKeys {
		if keyCount >= keyLimit {
			break
		}
		if value, exists := response[key]; exists {
			context[key] = simplifyValue(value)
			keyCount++
		}
	}

	for key, value := range response {
		if keyCount >= keyLimit {
			break
		}

		found := false
		for _, priorityKey := range priorityKeys {
			if key == priorityKey {
				found = true
				break
			}
		}
		if found || key == "error" || key == "metadata" || key == "debug" {
			continue
		}

		context[key] = simplifyValue(value)
		keyCount++
	}

	return context
}

func simplifyValue(value any) any {
	switch v := value.(type) {
	case string:
		if len(v) > 500 {
			return v[:500] + "...[truncated]"
		}
		return v
	case map[string]any:
		if len(v) > 10 {
			simplified := make(map[string]any)
			count := 0
			for k, val := range v {
				if count >= 10 {
					simplified["..."] = "additional fields truncated"
					break
				}
				simplified[k] = simplifyValue(val)
				count++
			}
			return simplified
		}
		return v
	case []any:
		if len(v) > 5 {
			simplified := make([]any, 5)
			for i := 0; i < 5; i++ {
				simplified[i] = simplifyValue(v[i])
			}
			return append(simplified, "...[additional items truncated]")
		}
		return v
	default:
		return v
	}
}

func generateResponseTags(task map[string]any, response map[string]any) []string {
	tags := make([]string, 0)

	if taskType, exists := task["type"].(string); exists {
		tags = append(tags, "task_"+taskType)
	}

	if priority, exists := task["priority"].(string); exists {
		tags = append(tags, "priority_"+priority)
	}

	if status, exists := response["status"].(string); exists {
		tags = append(tags, "status_"+status)
	}

	if desc, exists := task["description"].(string); exists {
		keywords := extractKeywords(desc)
		for _, keyword := range keywords {
			tags = append(tags, "keyword_"+keyword)
		}
	}

	return tags
}

func extractKeywords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	keywords := make([]string, 0)

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
	}

	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 3 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	return keywords
}

func extractSearchTerms(task map[string]any) []string {
	terms := make([]string, 0)

	if desc, exists := task["description"].(string); exists {
		terms = append(terms, extractKeywords(desc)...)
	}

	if taskType, exists := task["type"].(string); exists {
		terms = append(terms, taskType)
	}

	stringFields := []string{"category", "domain", "module", "component"}
	for _, field := range stringFields {
		if value, exists := task[field].(string); exists {
			terms = append(terms, value)
		}
	}

	return terms
}

func calculateSimilarity(searchTerms, tags []string, context map[string]any) float64 {
	if len(searchTerms) == 0 {
		return 0.0
	}

	matches := 0

	for _, term := range searchTerms {
		for _, tag := range tags {
			if strings.Contains(strings.ToLower(tag), strings.ToLower(term)) {
				matches++
				break
			}
		}
	}

	contextStr := strings.ToLower(fmt.Sprintf("%v", context))
	for _, term := range searchTerms {
		if strings.Contains(contextStr, strings.ToLower(term)) {
			matches++
		}
	}

	return float64(matches) / float64(len(searchTerms))
}

func isExpired(resp *CachedResponse) bool {
	return time.Now().After(resp.ExpiresAt)
}

func deepCopyMap(original map[string]any) map[string]any {
	copy := make(map[string]any)
	for k, v := range original {
		switch val := v.(type) {
		case map[string]any:
			copy[k] = deepCopyMap(val)
		case []any:
			copySlice := make([]any, len(val))
			for i, item := range val {
				if itemMap, ok := item.(map[string]any); ok {
					copySlice[i] = deepCopyMap(itemMap)
				} else {
					copySlice[i] = item
				}
			}
			copy[k] = copySlice
		default:
			copy[k] = v
		}
	}
	return copy
}

func copyMap(original map[string]any) map[string]any {
	copy := make(map[string]any)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func GetCachedResponsesForAgent(data map[string]any) map[string]any {
	agentID, ok := data["agent_id"].(string)
	if !ok || agentID == "" {
		return map[string]any{"error": "agent_id is required"}
	}

	limit := 10
	if l, ok := data["limit"].(float64); ok {
		limit = int(l)
	}

	includeRaw := false
	if raw, ok := data["include_raw"].(bool); ok {
		includeRaw = raw
	}

	if globalResponseCache == nil {
		return map[string]any{"error": "cache not initialized"}
	}

	globalResponseCache.mutex.RLock()
	responseIDs, exists := globalResponseCache.agentIndex[agentID]
	globalResponseCache.mutex.RUnlock()

	if !exists || len(responseIDs) == 0 {
		return map[string]any{
			"status":    "no_responses",
			"agent_id":  agentID,
			"responses": []map[string]any{},
		}
	}

	sortedResponses := globalResponseCache.sortResponsesByTime(responseIDs, true)
	if limit > 0 && len(sortedResponses) > limit {
		sortedResponses = sortedResponses[:limit]
	}

	responses := make([]map[string]any, 0, len(sortedResponses))
	for _, respID := range sortedResponses {
		if cached := GetCachedResponse(respID, includeRaw); cached["error"] == nil {
			responses = append(responses, cached)
		}
	}

	return map[string]any{
		"status":    "found",
		"agent_id":  agentID,
		"responses": responses,
		"total":     len(responseIDs),
	}
}

func SearchCachedResponses(data map[string]any) map[string]any {
	query, ok := data["query"].(map[string]any)
	if !ok {
		return map[string]any{"error": "query is required"}
	}

	agentID := getStringFromMap(data, "agent_id", "")

	similarity := 0.7
	if s, ok := data["similarity"].(float64); ok {
		similarity = s
	}

	return FindSimilarCachedResponses(query, agentID, similarity)
}

func PurgeCacheByAgent(data map[string]any) map[string]any {
	agentID, ok := data["agent_id"].(string)
	if !ok || agentID == "" {
		return map[string]any{"error": "agent_id is required"}
	}

	if globalResponseCache == nil {
		return map[string]any{"error": "cache not initialized"}
	}

	globalResponseCache.mutex.Lock()
	defer globalResponseCache.mutex.Unlock()

	responseIDs, exists := globalResponseCache.agentIndex[agentID]
	if !exists {
		return map[string]any{
			"status":   "no_responses",
			"agent_id": agentID,
			"removed":  0,
		}
	}

	removedCount := 0
	for _, responseID := range responseIDs {
		if _, exists := globalResponseCache.responses[responseID]; exists {
			globalResponseCache.removeResponse(responseID)
			removedCount++
		}
	}

	delete(globalResponseCache.agentIndex, agentID)

	if globalResponseCache.config.PersistToDisk {
		globalResponseCache.saveToDisk()
	}

	return map[string]any{
		"status":   "purged",
		"agent_id": agentID,
		"removed":  removedCount,
	}
}

func OptimizeCache(data map[string]any) map[string]any {
	if globalResponseCache == nil {
		return map[string]any{"error": "cache not initialized"}
	}

	globalResponseCache.mutex.Lock()
	defer globalResponseCache.mutex.Unlock()

	initialCount := len(globalResponseCache.responses)
	initialSize := globalResponseCache.totalSizeBytes

	globalResponseCache.cleanExpired()
	globalResponseCache.enforceEvictionPolicies()
	globalResponseCache.rebuildIndexes()

	finalCount := len(globalResponseCache.responses)
	finalSize := globalResponseCache.totalSizeBytes

	if globalResponseCache.config.PersistToDisk {
		globalResponseCache.saveToDisk()
	}

	return map[string]any{
		"status":            "optimized",
		"initial_count":     initialCount,
		"final_count":       finalCount,
		"removed_count":     initialCount - finalCount,
		"initial_size_mb":   float64(initialSize) / (1024 * 1024),
		"final_size_mb":     float64(finalSize) / (1024 * 1024),
		"size_reduction_mb": float64(initialSize-finalSize) / (1024 * 1024),
	}
}

func (rc *ResponseCache) rebuildIndexes() {
	rc.agentIndex = make(map[string][]string)
	rc.taskIndex = make(map[string][]string)
	rc.hashIndex = make(map[string]string)
	rc.timeIndex = make([]string, 0, len(rc.responses))
	rc.accessIndex = make([]string, 0, len(rc.responses))

	for responseID, resp := range rc.responses {
		rc.agentIndex[resp.AgentID] = append(rc.agentIndex[resp.AgentID], responseID)
		rc.taskIndex[resp.TaskID] = append(rc.taskIndex[resp.TaskID], responseID)

		if existingID, exists := rc.hashIndex[resp.TaskHash]; exists {
			if existing := rc.responses[existingID]; existing.Timestamp.Before(resp.Timestamp) {
				rc.hashIndex[resp.TaskHash] = responseID
			}
		} else {
			rc.hashIndex[resp.TaskHash] = responseID
		}

		rc.timeIndex = append(rc.timeIndex, responseID)
		rc.accessIndex = append(rc.accessIndex, responseID)
	}

	sort.Slice(rc.timeIndex, func(i, j int) bool {
		resp1 := rc.responses[rc.timeIndex[i]]
		resp2 := rc.responses[rc.timeIndex[j]]
		return resp1.Timestamp.Before(resp2.Timestamp)
	})

	sort.Slice(rc.accessIndex, func(i, j int) bool {
		resp1 := rc.responses[rc.accessIndex[i]]
		resp2 := rc.responses[rc.accessIndex[j]]
		return resp1.LastAccessed.Before(resp2.LastAccessed)
	})
}

func (rc *ResponseCache) sortResponsesByTime(responseIDs []string, descending bool) []string {
	sorted := make([]string, len(responseIDs))
	copy(sorted, responseIDs)

	sort.Slice(sorted, func(i, j int) bool {
		resp1 := rc.responses[sorted[i]]
		resp2 := rc.responses[sorted[j]]
		if descending {
			return resp1.Timestamp.After(resp2.Timestamp)
		}
		return resp1.Timestamp.Before(resp2.Timestamp)
	})

	return sorted
}

func findRelatedContexts(task map[string]any) map[string]any {
	related := make(map[string]any)
	taskType := getStringFromMap(task, "type", "")
	keywords := getStringSliceFromMap(task, "keywords")

	for agentID, context := range agentContexts {
		if hasRelevantContext(context, taskType, keywords) {
			related[agentID] = context
		}
	}

	return related
}

func hasRelevantContext(context map[string]any, taskType string, keywords []string) bool {
	contextStr := fmt.Sprintf("%v", context)

	if taskType != "" && strings.Contains(strings.ToLower(contextStr), strings.ToLower(taskType)) {
		return true
	}

	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(contextStr), strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

func getStringFromMap(m map[string]any, key, defaultVal string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultVal
}

func getStringSliceFromMap(m map[string]any, key string) []string {
	if val, ok := m[key].([]string); ok {
		return val
	}
	if val, ok := m[key].([]any); ok {
		result := make([]string, 0, len(val))
		for _, v := range val {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return []string{}
}

func StartCacheMaintenanceScheduler() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range time.Tick(1 * time.Hour) {
			if globalResponseCache != nil {
				ClearExpiredResponses()

				if time.Now().Hour()%6 == 0 {
					OptimizeCache(map[string]any{})
				}
			}
		}
	}()
}

func UpdateAgentContext(data map[string]any) map[string]any {
	agentID, ok := data["agent_id"].(string)
	if !ok || agentID == "" {
		return map[string]any{"error": "agent_id is required"}
	}

	context, ok := data["context"].(map[string]any)
	if !ok {
		return map[string]any{"error": "context is required"}
	}

	mutex.Lock()
	defer mutex.Unlock()

	if agentContexts[agentID] == nil {
		agentContexts[agentID] = make(map[string]any)
	}

	for k, v := range context {
		agentContexts[agentID][k] = v
	}

	if agent, exists := RegisteredAgents[agentID]; exists {
		agent.Context = agentContexts[agentID]
		agent.LastActive = time.Now()
	}

	saveOrchestratorContext()

	return map[string]any{
		"status":   "updated",
		"agent_id": agentID,
		"context":  agentContexts[agentID],
	}
}

func GetAgentContext(data map[string]any) map[string]any {
	agentID, ok := data["agent_id"].(string)
	if !ok || agentID == "" {
		return map[string]any{"error": "agent_id is required"}
	}

	mutex.RLock()
	defer mutex.RUnlock()

	context, exists := agentContexts[agentID]
	if !exists {
		return map[string]any{"error": "context not found for agent"}
	}

	return map[string]any{
		"status":   "success",
		"agent_id": agentID,
		"context":  context,
	}
}

func executeTaskWithAgent(agentID string, task map[string]any, contextResult map[string]any) map[string]any {
	mutex.RLock()
	agent, exists := RegisteredLiveAgents[agentID]
	agentMetadata, metaExists := RegisteredAgents[agentID]
	mutex.RUnlock()

	if !exists || !metaExists {
		return map[string]any{"error": fmt.Sprintf("agent %s not found", agentID)}
	}

	agentMetadata.LastActive = time.Now()

	taskDescription := getStringFromMap(task, "description", "")
	globalCtx := contextResult["global_context"]

	// Create a new contents array for the conversational context
	contents := make([]map[string]any, 0)

	// Append a message with the role of "user" to represent the current query
	contents = append(contents, map[string]any{
		"role": "user",
		"parts": []map[string]any{
			{"text": fmt.Sprintf("Task: %s\nContext: %v", taskDescription, globalCtx)},
		},
	})

	// Check for conversation history in global context and append it
	if history, ok := globalContext["conversation_history"].([]map[string]any); ok {
		contents = append(contents, history...)
	}

	var output map[string]any

	switch agentMetadata.Type {
	case repository.AgentPlanner:
		if plannerAgent, ok := agent.(*planner.PlannerAgent); ok {
			output = plannerAgent.RequestAgent(contents)
		} else {
			return map[string]any{"error": "invalid planner agent type"}
		}

	case repository.AgentCoder:
		if coderAgent, ok := agent.(*coder.CoderAgent); ok {
			output = coderAgent.RequestAgent(contents)
		} else {
			return map[string]any{"error": "invalid coder agent type"}
		}

	default:
		if activeAgent, ok := agent.(repository.ActiveAgent); ok {
			output = activeAgent.RequestAgent(contents)
		} else {
			return map[string]any{"error": "agent does not implement ActiveAgent interface"}
		}
	}

	if output["error"] != nil {
		return map[string]any{
			"status":   "failed",
			"agent_id": agentID,
			"error":    output["error"],
		}
	}

	responseParts := make([]map[string]any, 0)
	if outputParts, ok := output["output"].([]map[string]any); ok {
		for _, part := range outputParts {
			if text, textExists := part["text"].(string); textExists {
				responseParts = append(responseParts, map[string]any{
					"text": text,
				})
			} else {
				responseParts = append(responseParts, part)
			}
		}
	}

	mutex.Lock()
	if globalContext["conversation_history"] == nil {
		globalContext["conversation_history"] = make([]map[string]any, 0)
	}
	globalContext["conversation_history"] = append(globalContext["conversation_history"].([]map[string]any), map[string]any{
		"role":  "model",
		"parts": responseParts,
	})
	mutex.Unlock()
	saveOrchestratorContext()

	return map[string]any{
		"status":   "completed",
		"agent_id": agentID,
		"task":     task,
		"context":  contextResult,
		"output":   output["output"],
	}
}

func isAgentTypeRegistered(agentType string) bool {
	mutex.RLock()
	defer mutex.RUnlock()
	for _, metadata := range RegisteredAgents {
		if string(metadata.Type) == agentType && metadata.Status == "active" {
			return true
		}
	}
	return false
}

func determineAgentTypeFromQuery(query string) string {
	lowerQuery := strings.ToLower(query)
	// Check for simple conversational queries first
	if strings.Contains(lowerQuery, "hello") || strings.Contains(lowerQuery, "hi") || strings.Contains(lowerQuery, "how are you") || strings.Contains(lowerQuery, "what's up") {
		return "general_responder"
	}
	if strings.Contains(lowerQuery, "plan") || strings.Contains(lowerQuery, "organize") || strings.Contains(lowerQuery, "break down") || strings.Contains(lowerQuery, "workflow") || (strings.Contains(lowerQuery, "make") && (strings.Contains(lowerQuery, "app") || strings.Contains(lowerQuery, "game") || strings.Contains(lowerQuery, "cli tool"))) || strings.Contains(lowerQuery, "3d rendering") {
		return string(repository.AgentPlanner)
	}

	if strings.Contains(lowerQuery, "code") || strings.Contains(lowerQuery, "implement") || strings.Contains(lowerQuery, "script") || strings.Contains(lowerQuery, "write a program") {
		return string(repository.AgentCoder)
	}
	return string(repository.AgentPlanner)
}
