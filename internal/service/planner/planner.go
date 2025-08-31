package planner

import (
	"bytes"
	"context"
	"dost/internal/repository"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// Minimal repository package definition for Planner to function independently
// In a real application, this would be a separate shared package.
const (
	// AgentType defines the type of an agent, e.g., 'planner', 'coder'.
	AgentPlanner repository.AgentType = "planner"
	AgentCoder   repository.AgentType = "coder"
)

// Status constants
const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Placeholder for instructions
const PlannerInstructions = `You are a highly intelligent and meticulous planner agent. Your primary role is to decompose complex tasks into smaller, manageable subtasks, create detailed execution plans, manage workflows, and track progress. You are capable of analyzing task descriptions, assessing complexity, identifying dependencies, and assigning subtasks to the appropriate specialized agents (e.g., 'coder', 'analyst').

Your core responsibilities include:
1.  **Decomposition**: Breaking down high-level objectives into actionable steps and subtasks.
2.  **Planning**: Creating comprehensive 'ExecutionPlan' objects with subtasks, timelines, risk factors, and success metrics.
3.  **Coordination**: Defining workflows and dependencies to ensure a logical and efficient execution flow.
4.  **Monitoring**: Tracking the progress of tasks and plans, identifying blockers, and providing recommendations.

You have access to a suite of powerful tools to perform these tasks. You must use these tools whenever a request requires planning, task management, or progress evaluation. Do not attempt to complete the task yourself; instead, generate a plan or a series of tool calls that other agents can execute.

When a user provides a task, your first action should be to analyze it. If it's a complex or multi-step task, use the 'breakdown_task' or 'create_workflow' tool. If the task is to create a new plan, use the 'create_task' tool with the 'execution_plan' parameter. If you need to update an existing plan, use the 'update_plan' tool. Always prioritize using the provided tools to fulfill the user's request.`

// End of minimal repository package definition

const plannerName = "planner"
const plannerVersion = "0.1.0"
const plannerStateFile = "planner_state.json"

type PlannerAgent repository.Agent

// PlannerState manages all active plans, breakdowns, and history.
type PlannerState struct {
	ActivePlans    map[string]*ExecutionPlan `json:"active_plans"`
	TaskBreakdowns map[string]*TaskBreakdown `json:"task_breakdowns"`
	WorkflowStates map[string]*WorkflowState `json:"workflow_states"`
	PlanHistory    []PlanSnapshot            `json:"plan_history"`
	Metrics        *PlannerMetrics           `json:"metrics"`
	mu             sync.RWMutex              `json:"-"`
}

type ExecutionPlan struct {
	ID             string              `json:"id"`
	Overview       string              `json:"overview"`
	Subtasks       []SubTask           `json:"subtasks"`
	RiskFactors    []string            `json:"risk_factors"`
	SuccessMetrics string              `json:"success_metrics"`
	Dependencies   map[string][]string `json:"dependencies"`
	Timeline       *PlanTimeline       `json:"timeline"`
	Resources      []string            `json:"resources"`
	Context        map[string]any      `json:"context"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
	Status         string              `json:"status"`
}

type SubTask struct {
	ID                  string         `json:"id"`
	Description         string         `json:"description"`
	RequiredAgent       string         `json:"required_agent"`
	Priority            string         `json:"priority"`
	Dependencies        []string       `json:"dependencies"`
	EstimatedComplexity string         `json:"estimated_complexity"`
	Tools               []string       `json:"tools"`
	AcceptanceCriteria  string         `json:"acceptance_criteria"`
	Status              string         `json:"status"`
	Context             map[string]any `json:"context"`
	EstimatedDuration   time.Duration  `json:"estimated_duration"`
	ActualDuration      *time.Duration `json:"actual_duration,omitempty"`
	StartTime           *time.Time     `json:"start_time,omitempty"`
	EndTime             *time.Time     `json:"end_time,omitempty"`
}

type TaskBreakdown struct {
	ID           string         `json:"id"`
	OriginalTask string         `json:"original_task"`
	Strategy     string         `json:"strategy"`
	Steps        []TaskStep     `json:"steps"`
	Complexity   string         `json:"complexity"`
	Estimates    TaskEstimates  `json:"estimates"`
	Context      map[string]any `json:"context"`
	CreatedAt    time.Time      `json:"created_at"`
	Status       string         `json:"status"`
}

type TaskStep struct {
	ID           string         `json:"id"`
	Order        int            `json:"order"`
	Description  string         `json:"description"`
	Type         string         `json:"type"`
	AgentType    string         `json:"agent_type"`
	Dependencies []string       `json:"dependencies"`
	Context      map[string]any `json:"context"`
	Status       string         `json:"status"`
}

type TaskEstimates struct {
	TotalDuration   time.Duration `json:"total_duration"`
	ComplexityScore float64       `json:"complexity_score"`
	ResourcesNeeded []string      `json:"resources_needed"`
	RiskLevel       string        `json:"risk_level"`
	ConfidenceLevel float64       `json:"confidence_level"`
}

type WorkflowState struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
	Context     map[string]any `json:"context"`
	Status      string         `json:"status"`
	Progress    float64        `json:"progress"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

type WorkflowStep struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	AgentType    string         `json:"agent_type"`
	Dependencies []string       `json:"dependencies"`
	Context      map[string]any `json:"context"`
	Status       string         `json:"status"`
	Order        int            `json:"order"`
	StartTime    *time.Time     `json:"start_time,omitempty"`
	EndTime      *time.Time     `json:"end_time,omitempty"`
	Result       map[string]any `json:"result,omitempty"`
}

type PlanTimeline struct {
	StartDate    time.Time           `json:"start_date"`
	EndDate      time.Time           `json:"end_date"`
	Milestones   []Milestone         `json:"milestones"`
	CriticalPath []string            `json:"critical_path"`
	Dependencies map[string][]string `json:"dependencies"`
}

type Milestone struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
}

type PlanSnapshot struct {
	Timestamp time.Time      `json:"timestamp"`
	PlanID    string         `json:"plan_id"`
	Status    string         `json:"status"`
	Progress  float64        `json:"progress"`
	Context   map[string]any `json:"context"`
	Reason    string         `json:"reason"`
}

type PlannerMetrics struct {
	TotalPlansCreated   int           `json:"total_plans_created"`
	CompletedPlans      int           `json:"completed_plans"`
	FailedPlans         int           `json:"failed_plans"`
	AverageCompletion   float64       `json:"average_completion"`
	AveragePlanDuration time.Duration `json:"average_plan_duration"`
	LastActivity        time.Time     `json:"last_activity"`
}

// Global planner state
var plannerState *PlannerState

func init() {
	plannerState = &PlannerState{
		ActivePlans:    make(map[string]*ExecutionPlan),
		TaskBreakdowns: make(map[string]*TaskBreakdown),
		WorkflowStates: make(map[string]*WorkflowState),
		PlanHistory:    make([]PlanSnapshot, 0),
		Metrics: &PlannerMetrics{
			LastActivity: time.Now(),
		},
	}
	loadPlannerState()
}

// savePlannerState saves the current planner state to a JSON file.
func savePlannerState() {
	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	data, err := json.MarshalIndent(plannerState, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling planner state: %v\n", err)
		return
	}

	stateDir := getPlannerStateDir()
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		fmt.Printf("Error creating planner state directory: %v\n", err)
		return
	}

	filePath := filepath.Join(stateDir, plannerStateFile)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		fmt.Printf("Error writing planner state file: %v\n", err)
	}
}

// loadPlannerState loads the planner state from a JSON file.
func loadPlannerState() {
	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	stateDir := getPlannerStateDir()
	filePath := filepath.Join(stateDir, plannerStateFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Error reading planner state file: %v\n", err)
		}
		return
	}

	if err := json.Unmarshal(data, plannerState); err != nil {
		fmt.Printf("Error unmarshaling planner state: %v\n", err)
	}
}

// getPlannerStateDir returns the directory for storing planner state.
func getPlannerStateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dost", "planner")
}

func (p *PlannerState) GetPlannerState() map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return map[string]any{
		"active_plans":    len(p.ActivePlans),
		"task_breakdowns": len(p.TaskBreakdowns),
		"workflow_states": len(p.WorkflowStates),
		"plan_history":    len(p.PlanHistory),
		"metrics":         p.Metrics,
	}
}

func (p *PlannerState) GetActivePlans() map[string]*ExecutionPlan {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ActivePlans
}

func (p *PlannerState) ArchiveCompletedPlans() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	archived := 0

	for planID, plan := range p.ActivePlans {
		if isPlanComplete(plan) {
			createPlanSnapshot(planID, "archived")
			delete(p.ActivePlans, planID)
			archived++
		}
	}
	savePlannerState()
	return archived
}

func (p *PlannerState) GetPlannerMetrics() *PlannerMetrics {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Metrics.LastActivity = time.Now()

	totalDuration := time.Duration(0)
	completedCount := 0

	for _, plan := range p.ActivePlans {
		if isPlanComplete(plan) {
			if plan.Timeline != nil {
				duration := plan.Timeline.EndDate.Sub(plan.Timeline.StartDate)
				totalDuration += duration
				completedCount++
			}
		}
	}

	if completedCount > 0 {
		p.Metrics.AveragePlanDuration = totalDuration / time.Duration(completedCount)
	}
	savePlannerState()
	return p.Metrics
}

func (p *PlannerAgent) NewAgent() *PlannerAgent {
	model := viper.GetString("ORCHESTRATOR.MODEL")
	if model == "" {
		// Fallback model if not set in config
		model = "gemini-1.5-pro"
	}
	endPoints := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	id := fmt.Sprintf("planner-%s", uuid.NewString())

	agentMetadata := repository.AgentMetadata{
		ID:             id,
		Name:           plannerName,
		Version:        plannerVersion,
		Type:           repository.AgentPlanner,
		Instructions:   PlannerInstructions,
		MaxConcurrency: 3,
		Timeout:        5 * time.Minute,
		Tags:           []string{"planner", "agent"},
		Endpoints:      map[string]string{"http": endPoints},
		Context:        make(map[string]any),
		Status:         "active",

		LastActive: time.Now(),
	}

	agent := repository.Agent{
		Metadata:     agentMetadata,
		Capabilities: GetPlannerCapabilities(),
	}

	plannerAgent := PlannerAgent(agent)
	return &plannerAgent
}

func (p *PlannerAgent) RequestAgent(contents []map[string]any) map[string]any {
	fmt.Printf("Processing request with Planner Agent: %s\n", p.Metadata.Name)

	// Build request payload for the AI model
	request := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]any{
				{"text": p.Metadata.Instructions},
			},
		},
		"contents": contents,
		"tools": []map[string]any{
			{"function_declarations": GetPlannerCapabilitiesArrayMap()},
		},
	}

	// Marshal request body
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(
		context.Background(),
		"POST",
		p.Metadata.Endpoints["http"],
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-goog-api-key", viper.GetString("PLANNER.API_KEY"))

	// Execute request with timeout
	client := &http.Client{Timeout: p.Metadata.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return map[string]any{
			"error":  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)),
			"output": nil,
		}
	}

	// Parse response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	var response repository.Response
	if err = json.Unmarshal(bodyBytes, &response); err != nil {
		return map[string]any{"error": err.Error(), "output": nil}
	}

	// Process and normalize response
	newResponse := make([]map[string]any, 0)
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				newResponse = append(newResponse, map[string]any{
					"text": part.Text,
				})
			} else if part.FunctionCall != nil && part.FunctionCall.Name != "" {
				// Execute function call locally if it's a planner capability
				if funcResult := p.executePlannerFunction(part.FunctionCall.Name, part.FunctionCall.Args); funcResult != nil {
					newResponse = append(newResponse, map[string]any{
						"function_name": part.FunctionCall.Name,
						"parameters":    part.FunctionCall.Args,
						"result":        funcResult,
					})
				} else {
					newResponse = append(newResponse, map[string]any{
						"function_name": part.FunctionCall.Name,
						"parameters":    part.FunctionCall.Args,
					})
				}
			}
		}
	}

	// Update agent activity
	p.Metadata.LastActive = time.Now()
	plannerState.mu.Lock()
	plannerState.Metrics.LastActivity = time.Now()
	plannerState.mu.Unlock()
	savePlannerState()

	return map[string]any{"error": nil, "output": newResponse}
}

// Execute planner-specific functions
func (p *PlannerAgent) executePlannerFunction(funcName string, args map[string]any) map[string]any {
	capabilities := GetPlannerCapabilitiesMap()

	if capability, exists := capabilities[funcName]; exists {
		return capability.Service(args)
	}

	return nil
}

// Enhanced task decomposition with intelligent analysis
func DecomposeTask(data map[string]any) map[string]any {
	taskDescription, _ := data["description"].(string)
	domain, _ := data["domain"].(string)

	if taskDescription == "" {
		return map[string]any{"error": "task description is required"}
	}

	// Analyze task complexity and domain
	analysis := analyzeTaskComplexity(taskDescription, domain)

	breakdownID := fmt.Sprintf("breakdown_%s", uuid.NewString())

	// Generate subtasks based on analysis
	subtasks := generateIntelligentSubtasks(taskDescription, analysis)

	// Create task breakdown
	breakdown := &TaskBreakdown{
		ID:           breakdownID,
		OriginalTask: taskDescription,
		Strategy:     analysis.Strategy,
		Steps:        convertSubtasksToSteps(subtasks),
		Complexity:   analysis.ComplexityLevel,
		Estimates:    analysis.Estimates,
		Context:      copyMapSafe(data),
		CreatedAt:    time.Now(),
		Status:       "created",
	}

	// Store breakdown
	plannerState.mu.Lock()
	plannerState.TaskBreakdowns[breakdownID] = breakdown
	plannerState.Metrics.TotalPlansCreated++
	plannerState.mu.Unlock()
	savePlannerState()

	return map[string]any{
		"status":       "decomposed",
		"breakdown_id": breakdownID,
		"subtasks":     subtasks,
		"strategy":     analysis.Strategy,
		"complexity":   analysis.ComplexityLevel,
		"estimates":    analysis.Estimates,
		"dependencies": extractDependencies(subtasks),
	}
}

// Create comprehensive workflow with enhanced planning
func CreateWorkflow(data map[string]any) map[string]any {
	name, _ := data["name"].(string)
	description, _ := data["description"].(string)
	stepsData, _ := data["steps"].([]any)

	if name == "" {
		return map[string]any{"error": "workflow name is required"}
	}

	workflowID := fmt.Sprintf("workflow_%s", uuid.NewString())

	// Convert steps data to workflow steps
	steps := make([]WorkflowStep, 0)
	for i, stepData := range stepsData {
		if stepMap, ok := stepData.(map[string]any); ok {
			step := WorkflowStep{
				ID:           fmt.Sprintf("%s_step_%d", workflowID, i),
				Name:         getStringFromMapSafe(stepMap, "name", fmt.Sprintf("Step %d", i+1)),
				Description:  getStringFromMapSafe(stepMap, "description", ""),
				AgentType:    getStringFromMapSafe(stepMap, "agent_type", "planner"),
				Dependencies: getStringSliceFromMapSafe(stepMap, "dependencies"),
				Context:      getMapFromMapSafe(stepMap, "context"),
				Status:       "pending",
				Order:        i,
			}
			steps = append(steps, step)
		}
	}

	// Validate workflow dependencies
	if err := validateWorkflowDependencies(steps); err != nil {
		return map[string]any{"error": fmt.Sprintf("workflow validation failed: %v", err)}
	}

	workflow := &WorkflowState{
		ID:          workflowID,
		Name:        name,
		Description: description,
		Steps:       steps,
		Context:     getMapFromMapSafe(data, "context"),
		Status:      "created",
		Progress:    0.0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Store workflow
	plannerState.mu.Lock()
	plannerState.WorkflowStates[workflowID] = workflow
	plannerState.mu.Unlock()
	savePlannerState()

	return map[string]any{
		"status":      "workflow_created",
		"workflow_id": workflowID,
		"step_count":  len(steps),
		"workflow":    workflow,
	}
}

// Update existing plan with new information
func UpdatePlan(data map[string]any) map[string]any {
	planID, _ := data["plan_id"].(string)
	updates, _ := data["updates"].(map[string]any)

	if planID == "" {
		return map[string]any{"error": "plan_id is required"}
	}

	plannerState.mu.RLock()
	plan, exists := plannerState.ActivePlans[planID]
	plannerState.mu.RUnlock()

	if !exists {
		return map[string]any{"error": "plan not found"}
	}

	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	// Create snapshot before update
	createPlanSnapshot(planID, "before_update")

	// Apply updates
	if overview, ok := updates["overview"].(string); ok {
		plan.Overview = overview
	}

	if riskFactors, ok := updates["risk_factors"].([]any); ok {
		plan.RiskFactors = convertToStringSlice(riskFactors)
	}

	if successMetrics, ok := updates["success_metrics"].(string); ok {
		plan.SuccessMetrics = successMetrics
	}

	if subtaskUpdates, ok := updates["subtasks"].([]any); ok {
		updateSubtasks(plan, subtaskUpdates)
	}

	plan.UpdatedAt = time.Now()

	// Create snapshot after update
	createPlanSnapshot(planID, "after_update")

	savePlannerState()

	return map[string]any{
		"status":     "plan_updated",
		"plan_id":    planID,
		"updated_at": plan.UpdatedAt,
		"progress":   calculatePlanProgress(plan),
	}
}

// Get next actionable step in a plan
func GetNextStep(data map[string]any) map[string]any {
	planID, _ := data["plan_id"].(string)
	// agentCapabilities, _ := data["agent_capabilities"].([]any)

	if planID == "" {
		return map[string]any{"error": "plan_id is required"}
	}

	plannerState.mu.RLock()
	plan, exists := plannerState.ActivePlans[planID]
	plannerState.mu.RUnlock()

	if !exists {
		return map[string]any{"error": "plan not found"}
	}

	// Find next executable subtask
	nextTask := findNextExecutableTask(plan)

	if nextTask == nil {
		// Check if plan is complete
		if isPlanComplete(plan) {
			return map[string]any{
				"status":   "plan_complete",
				"plan_id":  planID,
				"message":  "All tasks in the plan have been completed",
				"progress": 1.0,
			}
		}

		// No executable tasks available (blocked by dependencies)
		return map[string]any{
			"status":   "waiting_for_dependencies",
			"plan_id":  planID,
			"message":  "No tasks are currently executable due to unmet dependencies",
			"progress": calculatePlanProgress(plan),
		}
	}

	return map[string]any{
		"status":              "next_step_found",
		"plan_id":             planID,
		"next_task":           nextTask,
		"estimated_duration":  nextTask.EstimatedDuration.String(),
		"required_agent":      nextTask.RequiredAgent,
		"tools_needed":        nextTask.Tools,
		"acceptance_criteria": nextTask.AcceptanceCriteria,
		"priority":            nextTask.Priority,
	}
}

// Evaluate plan progress and identify issues
func EvaluatePlanProgress(data map[string]any) map[string]any {
	planID, _ := data["plan_id"].(string)

	if planID == "" {
		return map[string]any{"error": "plan_id is required"}
	}

	plannerState.mu.RLock()
	plan, exists := plannerState.ActivePlans[planID]
	plannerState.mu.RUnlock()

	if !exists {
		return map[string]any{"error": "plan not found"}
	}

	// Analyze plan status
	progress := calculatePlanProgress(plan)
	blockers := findPlanBlockers(plan)
	risks := assessPlanRisks(plan)
	recommendations := generateRecommendations(plan, blockers, risks)

	evaluation := map[string]any{
		"status":               "evaluation_complete",
		"plan_id":              planID,
		"progress":             progress,
		"completion_rate":      fmt.Sprintf("%.1f%%", progress*100),
		"blockers":             blockers,
		"risks":                risks,
		"recommendations":      recommendations,
		"next_actions":         getNextActions(plan),
		"estimated_completion": estimateCompletionTime(plan),
	}

	// Update metrics
	plannerState.mu.Lock()
	plannerState.Metrics.AverageCompletion = (plannerState.Metrics.AverageCompletion + progress) / 2
	plannerState.mu.Unlock()
	savePlannerState()

	return evaluation
}

// Request missing information for planning
func RequestMissingInfo(data map[string]any) map[string]any {
	planID, _ := data["plan_id"].(string)
	infoType, _ := data["info_type"].(string)

	if planID == "" {
		return map[string]any{"error": "plan_id is required"}
	}

	plannerState.mu.RLock()
	plan, exists := plannerState.ActivePlans[planID]
	plannerState.mu.RUnlock()

	if !exists {
		return map[string]any{"error": "plan not found"}
	}

	// Analyze what information is missing
	missingInfo := analyzeMissingInformation(plan, infoType)

	return map[string]any{
		"status":            "missing_info_identified",
		"plan_id":           planID,
		"missing_info":      missingInfo,
		"info_requests":     generateInfoRequests(missingInfo),
		"impact_assessment": assessMissingInfoImpact(plan, missingInfo),
	}
}

// Enhanced task creation with planning intelligence
func CreateTask(data map[string]any) map[string]any {
	title, _ := data["title"].(string)
	// _, _ := data["created_by"].(string)
	planningComplete, _ := data["planning_complete"].(bool)

	if title == "" {
		return map[string]any{"error": "title is required"}
	}

	// Placeholder for repository.NewTask
	task := map[string]any{"output": map[string]any{"id": uuid.NewString(), "title": title}}

	// If planning is requested, create execution plan
	var executionPlan *ExecutionPlan
	if planningComplete {
		if execPlanData, ok := data["execution_plan"].(map[string]any); ok {
			planID := fmt.Sprintf("plan_%s", uuid.NewString())

			executionPlan = &ExecutionPlan{
				ID:             planID,
				Overview:       getStringFromMapSafe(execPlanData, "overview", ""),
				Subtasks:       parseSubtasks(execPlanData["subtasks"]),
				RiskFactors:    convertToStringSlice(execPlanData["risk_factors"].([]any)),
				SuccessMetrics: getStringFromMapSafe(execPlanData, "success_metrics", ""),
				Context:        copyMapSafe(data),
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
				Status:         "active",
			}

			// Generate timeline
			executionPlan.Timeline = generatePlanTimeline(executionPlan.Subtasks)

			// Store plan
			plannerState.mu.Lock()
			plannerState.ActivePlans[planID] = executionPlan
			plannerState.Metrics.TotalPlansCreated++
			plannerState.mu.Unlock()
			savePlannerState()
		}
	}

	response := map[string]any{
		"status": "task_created",
		"task":   task["output"],
	}

	if executionPlan != nil {
		response["execution_plan"] = executionPlan
		response["planning_complete"] = true
		response["plan_id"] = executionPlan.ID
	}

	if nextStep, ok := data["next_step"].(string); ok && nextStep != "" {
		response["next_step"] = nextStep
	}

	return response
}

// Break down task with intelligent analysis
func BreakDownTask(data map[string]any) map[string]any {
	taskID, _ := data["task_id"].(string)
	description, _ := data["description"].(string)

	if taskID == "" || description == "" {
		return map[string]any{"error": "task_id and description are required"}
	}

	// Perform intelligent task analysis
	analysis := analyzeTaskComplexity(description, "")

	// Generate breakdown strategy
	strategy := determineBreakdownStrategy(description, analysis)

	// Create detailed breakdown
	breakdown := &TaskBreakdown{
		ID:           fmt.Sprintf("breakdown_%s", uuid.NewString()),
		OriginalTask: description,
		Strategy:     strategy,
		Steps:        generateDetailedSteps(description, strategy, analysis),
		Complexity:   analysis.ComplexityLevel,
		Estimates:    analysis.Estimates,
		Context:      copyMapSafe(data),
		CreatedAt:    time.Now(),
		Status:       "created",
	}

	plannerState.mu.Lock()
	plannerState.TaskBreakdowns[breakdown.ID] = breakdown
	plannerState.mu.Unlock()
	savePlannerState()

	return map[string]any{
		"status":             "task_broken_down",
		"task_id":            taskID,
		"breakdown_id":       breakdown.ID,
		"steps":              breakdown.Steps,
		"strategy":           strategy,
		"complexity":         analysis.ComplexityLevel,
		"total_steps":        len(breakdown.Steps),
		"estimated_duration": analysis.Estimates.TotalDuration.String(),
		"confidence":         analysis.Estimates.ConfidenceLevel,
	}
}

// Update task status with context awareness
func UpdateTaskStatus(data map[string]any) map[string]any {
	taskID, _ := data["task_id"].(string)
	status, _ := data["status"].(string)
	context, _ := data["context"].(map[string]any)

	if taskID == "" || status == "" {
		return map[string]any{"error": "task_id and status are required"}
	}

	// Update associated plans if task is part of one
	updatedPlans := updateRelatedPlans(taskID, status, context)
	savePlannerState()

	response := map[string]any{
		"status":     "task_status_updated",
		"task_id":    taskID,
		"new_status": status,
		"updated_at": time.Now(),
	}

	if len(updatedPlans) > 0 {
		response["updated_plans"] = updatedPlans
		response["plan_impacts"] = assessPlanImpacts(updatedPlans, status)
	}

	return response
}

// Track progress with detailed metrics
func TrackProgress(data map[string]any) map[string]any {
	taskID, _ := data["task_id"].(string)
	phase, _ := data["phase"].(string)
	progressValue, _ := data["progress"].(float64)
	metrics, _ := data["metrics"].(map[string]any)

	if taskID == "" || phase == "" {
		return map[string]any{"error": "task_id and phase are required"}
	}

	// Create progress entry
	progressEntry := map[string]any{
		"task_id":   taskID,
		"phase":     phase,
		"progress":  progressValue,
		"metrics":   metrics,
		"logged_at": time.Now(),
		"logged_by": "planner_agent",
	}

	// Update related plans
	planUpdates := updatePlanProgress(taskID, phase, progressValue)
	savePlannerState()

	response := map[string]any{
		"status":         "progress_logged",
		"task_id":        taskID,
		"phase":          phase,
		"progress_entry": progressEntry,
	}

	if len(planUpdates) > 0 {
		response["plan_updates"] = planUpdates
		response["overall_progress"] = calculateOverallProgress(planUpdates)
	}

	return response
}

// Evaluate task completion with comprehensive analysis
func EvaluateTaskCompletion(data map[string]any) map[string]any {
	trackerData, ok := data["tracker"].(map[string]any)
	if !ok {
		return map[string]any{"error": "tracker is required"}
	}

	tracker := repository.TaskTracker{}
	if err := mapToStruct(trackerData, &tracker); err != nil {
		return map[string]any{"error": "invalid tracker data"}
	}

	// Enhanced completion evaluation
	isComplete := repository.IsTaskLikelyComplete(&tracker)
	completionConfidence := calculateCompletionConfidence(&tracker)
	completionMetrics := analyzeCompletionMetrics(&tracker)

	// Check plan completion if task is part of a plan
	planStatus := checkPlanCompletion(tracker.TaskID)

	evaluation := map[string]any{
		"status":                "evaluation_complete",
		"task_id":               tracker.TaskID,
		"is_complete":           isComplete,
		"completion_confidence": completionConfidence,
		"completion_metrics":    completionMetrics,
		"evaluation_timestamp":  time.Now(),
	}

	if planStatus != nil {
		evaluation["plan_status"] = planStatus
		evaluation["plan_completion"] = planStatus["completion_percentage"]
	}

	// Update planner metrics
	plannerState.mu.Lock()
	if isComplete {
		plannerState.Metrics.CompletedPlans++
	}
	plannerState.mu.Unlock()
	savePlannerState()

	return evaluation
}

// PausePlan pauses a plan's execution.
func PausePlan(data map[string]any) map[string]any {
	planID, ok := data["plan_id"].(string)
	if !ok || planID == "" {
		return map[string]any{"error": "plan_id is required"}
	}

	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	plan, exists := plannerState.ActivePlans[planID]
	if !exists {
		return map[string]any{"error": "plan not found"}
	}

	plan.Status = "paused"
	plan.UpdatedAt = time.Now()
	createPlanSnapshot(planID, "paused")
	savePlannerState()

	return map[string]any{
		"status":  "plan_paused",
		"plan_id": planID,
	}
}

// ResumePlan resumes a paused plan.
func ResumePlan(data map[string]any) map[string]any {
	planID, ok := data["plan_id"].(string)
	if !ok || planID == "" {
		return map[string]any{"error": "plan_id is required"}
	}

	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	plan, exists := plannerState.ActivePlans[planID]
	if !exists {
		return map[string]any{"error": "plan not found"}
	}

	if plan.Status != "paused" {
		return map[string]any{"error": "plan is not paused"}
	}

	plan.Status = "active"
	plan.UpdatedAt = time.Now()
	createPlanSnapshot(planID, "resumed")
	savePlannerState()

	return map[string]any{
		"status":  "plan_resumed",
		"plan_id": planID,
	}
}

// CancelPlan cancels a plan and marks it as failed.
func CancelPlan(data map[string]any) map[string]any {
	planID, ok := data["plan_id"].(string)
	if !ok || planID == "" {
		return map[string]any{"error": "plan_id is required"}
	}

	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	plan, exists := plannerState.ActivePlans[planID]
	if !exists {
		return map[string]any{"error": "plan not found"}
	}

	plan.Status = "canceled"
	plan.UpdatedAt = time.Now()
	createPlanSnapshot(planID, "canceled")
	savePlannerState()

	return map[string]any{
		"status":  "plan_canceled",
		"plan_id": planID,
	}
}

// ClonePlan creates a new plan from an existing one.
func ClonePlan(data map[string]any) map[string]any {
	planID, ok := data["plan_id"].(string)
	if !ok || planID == "" {
		return map[string]any{"error": "plan_id is required"}
	}

	plannerState.mu.RLock()
	plan, exists := plannerState.ActivePlans[planID]
	plannerState.mu.RUnlock()

	if !exists {
		return map[string]any{"error": "plan not found"}
	}

	newPlanID := fmt.Sprintf("plan_%s", uuid.NewString())

	// Deep copy the plan
	newPlan := &ExecutionPlan{
		ID:             newPlanID,
		Overview:       plan.Overview,
		Subtasks:       make([]SubTask, len(plan.Subtasks)),
		RiskFactors:    append([]string{}, plan.RiskFactors...),
		SuccessMetrics: plan.SuccessMetrics,
		Dependencies:   deepCopyDependencies(plan.Dependencies),
		Resources:      append([]string{}, plan.Resources...),
		Context:        copyMapSafe(plan.Context),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Status:         "active",
	}

	// Copy and reset subtask status
	for i, subtask := range plan.Subtasks {
		newSubtaskID := fmt.Sprintf("subtask_%s_%d", uuid.NewString(), i)
		newSubtask := subtask
		newSubtask.ID = newSubtaskID
		newSubtask.Status = "pending"
		newSubtask.StartTime = nil
		newSubtask.EndTime = nil
		newSubtask.ActualDuration = nil
		newSubtask.Dependencies = updateDependenciesForClone(subtask.Dependencies, plan.Subtasks, newPlan.Subtasks)
		newPlan.Subtasks[i] = newSubtask
	}

	newPlan.Timeline = generatePlanTimeline(newPlan.Subtasks)

	plannerState.mu.Lock()
	plannerState.ActivePlans[newPlanID] = newPlan
	plannerState.Metrics.TotalPlansCreated++
	plannerState.mu.Unlock()
	savePlannerState()

	return map[string]any{
		"status":      "plan_cloned",
		"old_plan_id": planID,
		"new_plan_id": newPlanID,
		"new_plan":    newPlan,
	}
}

func deepCopyDependencies(deps map[string][]string) map[string][]string {
	newDeps := make(map[string][]string)
	for k, v := range deps {
		newDeps[k] = append([]string{}, v...)
	}
	return newDeps
}

func updateDependenciesForClone(deps []string, oldSubtasks, newSubtasks []SubTask) []string {
	newDeps := make([]string, len(deps))
	for i, depID := range deps {
		// This is a simplified approach. A more robust solution would map old IDs to new IDs.
		// For now, we assume dependencies are within the same plan and the order is preserved.
		for j, oldSub := range oldSubtasks {
			if oldSub.ID == depID {
				newDeps[i] = newSubtasks[j].ID
				break
			}
		}
	}
	return newDeps
}

// Helper functions for enhanced planning capabilities

type TaskAnalysis struct {
	ComplexityLevel string
	Strategy        string
	Estimates       TaskEstimates
	RequiredAgents  []string
	RiskFactors     []string
}

func analyzeTaskComplexity(description, domain string) TaskAnalysis {
	description = strings.ToLower(description)

	// Complexity indicators
	complexityIndicators := map[string]int{
		"complex":      3,
		"advanced":     3,
		"enterprise":   3,
		"scalable":     2,
		"integrate":    2,
		"system":       2,
		"architecture": 3,
		"optimize":     2,
		"implement":    2,
		"design":       2,
		"create":       1,
		"simple":       1,
		"basic":        1,
	}

	totalComplexity := 1
	requiredAgents := make([]string, 0)
	riskFactors := make([]string, 0)

	// Analyze complexity
	for indicator, weight := range complexityIndicators {
		if strings.Contains(description, indicator) {
			totalComplexity += weight
		}
	}

	// Determine required agents
	if strings.Contains(description, "code") || strings.Contains(description, "implement") {
		requiredAgents = append(requiredAgents, "coder")
	}
	if strings.Contains(description, "plan") || strings.Contains(description, "design") {
		requiredAgents = append(requiredAgents, "planner")
	}
	if strings.Contains(description, "test") || strings.Contains(description, "validate") {
		requiredAgents = append(requiredAgents, "tester")
	}

	// Assess risk factors
	if totalComplexity > 5 {
		riskFactors = append(riskFactors, "high_complexity")
	}
	if strings.Contains(description, "integration") {
		riskFactors = append(riskFactors, "integration_challenges")
	}
	if strings.Contains(description, "performance") {
		riskFactors = append(riskFactors, "performance_requirements")
	}

	// Determine complexity level
	var complexityLevel string
	var estimatedDuration time.Duration
	var confidenceLevel float64

	switch {
	case totalComplexity <= 2:
		complexityLevel = "simple"
		estimatedDuration = 30 * time.Minute
		confidenceLevel = 0.9
	case totalComplexity <= 4:
		complexityLevel = "moderate"
		estimatedDuration = 2 * time.Hour
		confidenceLevel = 0.7
	case totalComplexity <= 6:
		complexityLevel = "complex"
		estimatedDuration = 8 * time.Hour
		confidenceLevel = 0.5
	default:
		complexityLevel = "very_complex"
		estimatedDuration = 24 * time.Hour
		confidenceLevel = 0.3
	}

	strategy := determineStrategy(description, complexityLevel, requiredAgents)

	return TaskAnalysis{
		ComplexityLevel: complexityLevel,
		Strategy:        strategy,
		RequiredAgents:  requiredAgents,
		RiskFactors:     riskFactors,
		Estimates: TaskEstimates{
			TotalDuration:   estimatedDuration,
			ComplexityScore: float64(totalComplexity),
			ResourcesNeeded: requiredAgents,
			RiskLevel:       assessRiskLevel(riskFactors),
			ConfidenceLevel: confidenceLevel,
		},
	}
}

func determineStrategy(description, complexity string, agents []string) string {
	strategies := make([]string, 0)

	if len(agents) > 1 {
		strategies = append(strategies, "multi_agent")
	}

	if complexity == "complex" || complexity == "very_complex" {
		strategies = append(strategies, "iterative_refinement")
	}

	if strings.Contains(description, "test") {
		strategies = append(strategies, "test_driven")
	}

	if len(strategies) == 0 {
		strategies = append(strategies, "sequential")
	}

	return strings.Join(strategies, "+")
}

func assessRiskLevel(riskFactors []string) string {
	switch len(riskFactors) {
	case 0:
		return "low"
	case 1:
		return "medium"
	default:
		return "high"
	}
}

func generateIntelligentSubtasks(description string, analysis TaskAnalysis) []SubTask {
	subtasks := make([]SubTask, 0)
	baseID := fmt.Sprintf("subtask_%d", time.Now().UnixNano())

	// Generate subtasks based on strategy and complexity
	switch analysis.Strategy {
	case "multi_agent":
		subtasks = append(subtasks, generateMultiAgentSubtasks(baseID, description, analysis)...)
	case "iterative_refinement":
		subtasks = append(subtasks, generateIterativeSubtasks(baseID, description, analysis)...)
	case "test_driven":
		subtasks = append(subtasks, generateTestDrivenSubtasks(baseID, description, analysis)...)
	default:
		subtasks = append(subtasks, generateSequentialSubtasks(baseID, description, analysis)...)
	}

	return subtasks
}

func generateMultiAgentSubtasks(baseID, description string, analysis TaskAnalysis) []SubTask {
	subtasks := make([]SubTask, 0)

	// Planning phase
	subtasks = append(subtasks, SubTask{
		ID:                  fmt.Sprintf("%s_plan", baseID),
		Description:         fmt.Sprintf("Plan and analyze approach for: %s", description),
		RequiredAgent:       "planner",
		Priority:            "high",
		Dependencies:        []string{},
		EstimatedComplexity: "moderate",
		Tools:               []string{"analysis", "planning"},
		AcceptanceCriteria:  "Clear plan with defined steps and success criteria",
		Status:              "pending",
		EstimatedDuration:   analysis.Estimates.TotalDuration / 4,
	})

	// Implementation phase
	if contains(analysis.RequiredAgents, "coder") {
		subtasks = append(subtasks, SubTask{
			ID:                  fmt.Sprintf("%s_implement", baseID),
			Description:         fmt.Sprintf("Implement solution for: %s", description),
			RequiredAgent:       "coder",
			Priority:            "high",
			Dependencies:        []string{fmt.Sprintf("%s_plan", baseID)},
			EstimatedComplexity: analysis.ComplexityLevel,
			Tools:               []string{"coding", "development"},
			AcceptanceCriteria:  "Working implementation that meets requirements",
			Status:              "pending",
			EstimatedDuration:   analysis.Estimates.TotalDuration / 2,
		})
	}

	// Validation phase
	subtasks = append(subtasks, SubTask{
		ID:                  fmt.Sprintf("%s_validate", baseID),
		Description:         fmt.Sprintf("Validate and test: %s", description),
		RequiredAgent:       "planner",
		Priority:            "medium",
		Dependencies:        getDependenciesForValidation(subtasks),
		EstimatedComplexity: "simple",
		Tools:               []string{"testing", "validation"},
		AcceptanceCriteria:  "Solution validated and meets success criteria",
		Status:              "pending",
		EstimatedDuration:   analysis.Estimates.TotalDuration / 4,
	})

	return subtasks
}

func generateIterativeSubtasks(baseID, description string, analysis TaskAnalysis) []SubTask {
	subtasks := make([]SubTask, 0)

	// Initial analysis
	subtasks = append(subtasks, SubTask{
		ID:                  fmt.Sprintf("%s_analyze", baseID),
		Description:         fmt.Sprintf("Initial analysis of: %s", description),
		RequiredAgent:       "planner",
		Priority:            "high",
		Dependencies:        []string{},
		EstimatedComplexity: "moderate",
		Tools:               []string{"analysis"},
		AcceptanceCriteria:  "Requirements clearly understood",
		Status:              "pending",
		EstimatedDuration:   analysis.Estimates.TotalDuration / 6,
	})

	// Iterative development cycles
	iterations := 3
	for i := 1; i <= iterations; i++ {
		iterationID := fmt.Sprintf("%s_iteration_%d", baseID, i)

		subtasks = append(subtasks, SubTask{
			ID:                  iterationID,
			Description:         fmt.Sprintf("Iteration %d: Develop and refine %s", i, description),
			RequiredAgent:       determineIterationAgent(i, analysis.RequiredAgents),
			Priority:            "high",
			Dependencies:        getIterationDependencies(i, baseID),
			EstimatedComplexity: analysis.ComplexityLevel,
			Tools:               getIterationTools(i),
			AcceptanceCriteria:  fmt.Sprintf("Iteration %d objectives met", i),
			Status:              "pending",
			EstimatedDuration:   analysis.Estimates.TotalDuration / time.Duration(iterations+1),
		})
	}

	return subtasks
}

func generateTestDrivenSubtasks(baseID, description string, analysis TaskAnalysis) []SubTask {
	subtasks := make([]SubTask, 0)

	// Test planning
	subtasks = append(subtasks, SubTask{
		ID:                  fmt.Sprintf("%s_test_plan", baseID),
		Description:         fmt.Sprintf("Create test plan for: %s", description),
		RequiredAgent:       "planner",
		Priority:            "high",
		Dependencies:        []string{},
		EstimatedComplexity: "moderate",
		Tools:               []string{"test_planning", "analysis"},
		AcceptanceCriteria:  "Comprehensive test plan with success criteria",
		Status:              "pending",
		EstimatedDuration:   analysis.Estimates.TotalDuration / 5,
	})

	// Implementation with tests
	subtasks = append(subtasks, SubTask{
		ID:                  fmt.Sprintf("%s_implement_with_tests", baseID),
		Description:         fmt.Sprintf("Implement with tests: %s", description),
		RequiredAgent:       "coder",
		Priority:            "high",
		Dependencies:        []string{fmt.Sprintf("%s_test_plan", baseID)},
		EstimatedComplexity: analysis.ComplexityLevel,
		Tools:               []string{"coding", "testing", "development"},
		AcceptanceCriteria:  "Implementation passes all tests",
		Status:              "pending",
		EstimatedDuration:   analysis.Estimates.TotalDuration * 3 / 5,
	})

	// Validation
	subtasks = append(subtasks, SubTask{
		ID:                  fmt.Sprintf("%s_final_validation", baseID),
		Description:         fmt.Sprintf("Final validation of: %s", description),
		RequiredAgent:       "planner",
		Priority:            "medium",
		Dependencies:        []string{fmt.Sprintf("%s_implement_with_tests", baseID)},
		EstimatedComplexity: "simple",
		Tools:               []string{"validation", "testing"},
		AcceptanceCriteria:  "All acceptance criteria met",
		Status:              "pending",
		EstimatedDuration:   analysis.Estimates.TotalDuration / 5,
	})

	return subtasks
}

func generateSequentialSubtasks(baseID, description string, analysis TaskAnalysis) []SubTask {
	subtasks := make([]SubTask, 0)

	phases := []struct {
		name       string
		agent      string
		complexity string
		tools      []string
		portion    float64
	}{
		{"analyze", "planner", "moderate", []string{"analysis"}, 0.2},
		{"design", "planner", "moderate", []string{"design", "planning"}, 0.3},
		{"implement", determinePrimaryAgent(analysis.RequiredAgents), analysis.ComplexityLevel, []string{"implementation"}, 0.4},
		{"validate", "planner", "simple", []string{"validation"}, 0.1},
	}

	for i, phase := range phases {
		var deps []string
		if i > 0 {
			deps = []string{fmt.Sprintf("%s_%s", baseID, phases[i-1].name)}
		}

		subtasks = append(subtasks, SubTask{
			ID:                  fmt.Sprintf("%s_%s", baseID, phase.name),
			Description:         fmt.Sprintf("%s: %s", strings.Title(phase.name), description),
			RequiredAgent:       phase.agent,
			Priority:            "medium",
			Dependencies:        deps,
			EstimatedComplexity: phase.complexity,
			Tools:               phase.tools,
			AcceptanceCriteria:  fmt.Sprintf("%s phase completed successfully", strings.Title(phase.name)),
			Status:              "pending",
			EstimatedDuration:   time.Duration(float64(analysis.Estimates.TotalDuration) * phase.portion),
		})
	}

	return subtasks
}

func convertSubtasksToSteps(subtasks []SubTask) []TaskStep {
	steps := make([]TaskStep, len(subtasks))

	for i, subtask := range subtasks {
		steps[i] = TaskStep{
			ID:           subtask.ID,
			Order:        i,
			Description:  subtask.Description,
			Type:         inferStepType(subtask),
			AgentType:    subtask.RequiredAgent,
			Dependencies: subtask.Dependencies,
			Context:      subtask.Context,
			Status:       subtask.Status,
		}
	}

	return steps
}

func generatePlanTimeline(subtasks []SubTask) *PlanTimeline {
	if len(subtasks) == 0 {
		return nil
	}

	startDate := time.Now()
	var totalDuration time.Duration

	// Calculate total duration and create milestones
	milestones := make([]Milestone, 0)
	dependencies := make(map[string][]string)
	criticalPath := make([]string, 0)

	for _, subtask := range subtasks {
		totalDuration += subtask.EstimatedDuration
		dependencies[subtask.ID] = subtask.Dependencies

		// Create milestone for each major subtask
		if subtask.Priority == "high" {
			milestones = append(milestones, Milestone{
				ID:          fmt.Sprintf("milestone_%s", subtask.ID),
				Name:        fmt.Sprintf("Complete %s", subtask.Description),
				Date:        startDate.Add(totalDuration),
				Description: subtask.AcceptanceCriteria,
				Status:      "pending",
			})
			criticalPath = append(criticalPath, subtask.ID)
		}
	}

	return &PlanTimeline{
		StartDate:    startDate,
		EndDate:      startDate.Add(totalDuration),
		Milestones:   milestones,
		CriticalPath: criticalPath,
		Dependencies: dependencies,
	}
}

func findNextExecutableTask(plan *ExecutionPlan) *SubTask {
	for i := range plan.Subtasks {
		subtask := &plan.Subtasks[i]

		if subtask.Status != "pending" {
			continue
		}

		// Check if all dependencies are completed
		dependenciesMet := true
		for _, depID := range subtask.Dependencies {
			depMet := false
			for _, otherSubtask := range plan.Subtasks {
				if otherSubtask.ID == depID && otherSubtask.Status == "completed" {
					depMet = true
					break
				}
			}
			if !depMet {
				dependenciesMet = false
				break
			}
		}

		if dependenciesMet {
			return subtask
		}
	}

	return nil
}

func calculatePlanProgress(plan *ExecutionPlan) float64 {
	if len(plan.Subtasks) == 0 {
		return 0.0
	}

	completed := 0
	for _, subtask := range plan.Subtasks {
		if subtask.Status == "completed" {
			completed++
		}
	}

	return float64(completed) / float64(len(plan.Subtasks))
}

func isPlanComplete(plan *ExecutionPlan) bool {
	for _, subtask := range plan.Subtasks {
		if subtask.Status != "completed" {
			return false
		}
	}
	return true
}

func findPlanBlockers(plan *ExecutionPlan) []map[string]any {
	blockers := make([]map[string]any, 0)

	for _, subtask := range plan.Subtasks {
		if subtask.Status == "pending" {
			// Check for unmet dependencies
			unmetDeps := make([]string, 0)
			for _, depID := range subtask.Dependencies {
				depMet := false
				for _, otherSubtask := range plan.Subtasks {
					if otherSubtask.ID == depID && otherSubtask.Status == "completed" {
						depMet = true
						break
					}
				}
				if !depMet {
					unmetDeps = append(unmetDeps, depID)
				}
			}

			if len(unmetDeps) > 0 {
				blockers = append(blockers, map[string]any{
					"type":        "dependency_blocker",
					"task_id":     subtask.ID,
					"description": subtask.Description,
					"blocked_by":  unmetDeps,
				})
			}
		}

		// Check for failed tasks blocking others
		if subtask.Status == "failed" {
			dependentTasks := findDependentTasks(plan, subtask.ID)
			if len(dependentTasks) > 0 {
				blockers = append(blockers, map[string]any{
					"type":           "failure_blocker",
					"failed_task_id": subtask.ID,
					"blocking_tasks": dependentTasks,
				})
			}
		}
	}

	return blockers
}

func assessPlanRisks(plan *ExecutionPlan) []map[string]any {
	risks := make([]map[string]any, 0)

	// Time-based risks
	now := time.Now()
	if plan.Timeline != nil && now.After(plan.Timeline.EndDate) {
		risks = append(risks, map[string]any{
			"type":        "schedule_overrun",
			"severity":    "high",
			"description": "Plan is past scheduled completion date",
		})
	}

	// Complexity risks
	complexTasks := 0
	for _, subtask := range plan.Subtasks {
		if subtask.EstimatedComplexity == "complex" || subtask.EstimatedComplexity == "very_complex" {
			complexTasks++
		}
	}

	if float64(complexTasks)/float64(len(plan.Subtasks)) > 0.5 {
		risks = append(risks, map[string]any{
			"type":        "complexity_risk",
			"severity":    "medium",
			"description": "High percentage of complex tasks may cause delays",
		})
	}

	// Dependency risks
	for _, subtask := range plan.Subtasks {
		if len(subtask.Dependencies) > 3 {
			risks = append(risks, map[string]any{
				"type":        "dependency_complexity",
				"severity":    "medium",
				"description": fmt.Sprintf("Task %s has many dependencies", subtask.ID),
				"task_id":     subtask.ID,
			})
		}
	}

	return risks
}

func generateRecommendations(plan *ExecutionPlan, blockers, risks []map[string]any) []string {
	recommendations := make([]string, 0)

	// Recommendations based on blockers
	for _, blocker := range blockers {
		blockerType, _ := blocker["type"].(string)
		switch blockerType {
		case "dependency_blocker":
			recommendations = append(recommendations, "Consider parallelizing independent tasks to reduce wait time")
		case "failure_blocker":
			recommendations = append(recommendations, "Address failed tasks immediately to unblock dependent work")
		}
	}

	// Recommendations based on risks
	for _, risk := range risks {
		riskType, _ := risk["type"].(string)
		switch riskType {
		case "schedule_overrun":
			recommendations = append(recommendations, "Re-evaluate timeline and consider scope reduction")
		case "complexity_risk":
			recommendations = append(recommendations, "Break complex tasks into smaller, manageable pieces")
		case "dependency_complexity":
			recommendations = append(recommendations, "Review dependencies and optimize task ordering")
		}
	}

	// General recommendations
	progress := calculatePlanProgress(plan)
	if progress < 0.3 && time.Since(plan.CreatedAt) > 24*time.Hour {
		recommendations = append(recommendations, "Plan progress is slow - consider resource reallocation")
	}

	return recommendations
}

func getNextActions(plan *ExecutionPlan) []string {
	actions := make([]string, 0)

	// Find immediately actionable tasks
	for _, subtask := range plan.Subtasks {
		if subtask.Status == "pending" {
			dependenciesMet := true
			for _, depID := range subtask.Dependencies {
				depMet := false
				for _, otherSubtask := range plan.Subtasks {
					if otherSubtask.ID == depID && otherSubtask.Status == "completed" {
						depMet = true
						break
					}
				}
				if !depMet {
					dependenciesMet = false
					break
				}
			}

			if dependenciesMet {
				actions = append(actions, fmt.Sprintf("Execute task: %s", subtask.Description))
			}
		}
	}

	if len(actions) == 0 {
		actions = append(actions, "Review and resolve blockers to proceed")
	}

	return actions
}

func estimateCompletionTime(plan *ExecutionPlan) *time.Time {
	progress := calculatePlanProgress(plan)
	if progress == 0 {
		return nil
	}

	elapsed := time.Since(plan.CreatedAt)
	totalEstimated := time.Duration(float64(elapsed) / progress)
	completion := plan.CreatedAt.Add(totalEstimated)

	return &completion
}

func analyzeMissingInformation(plan *ExecutionPlan, infoType string) []string {
	missing := make([]string, 0)

	switch infoType {
	case "requirements":
		if plan.Overview == "" {
			missing = append(missing, "plan_overview")
		}
		if plan.SuccessMetrics == "" {
			missing = append(missing, "success_metrics")
		}

	case "resources":
		if len(plan.Resources) == 0 {
			missing = append(missing, "resource_list")
		}

	case "timeline":
		if plan.Timeline == nil {
			missing = append(missing, "timeline_information")
		}

	default:
		// General analysis
		if plan.Overview == "" {
			missing = append(missing, "plan_overview")
		}
		if len(plan.Subtasks) == 0 {
			missing = append(missing, "task_breakdown")
		}
		if len(plan.RiskFactors) == 0 {
			missing = append(missing, "risk_assessment")
		}
	}

	return missing
}

func generateInfoRequests(missingInfo []string) []map[string]any {
	requests := make([]map[string]any, 0)

	infoDescriptions := map[string]string{
		"plan_overview":        "Please provide a high-level overview of the plan objectives",
		"success_metrics":      "Define measurable success criteria for this plan",
		"resource_list":        "Specify required resources (people, tools, systems)",
		"timeline_information": "Provide timeline constraints or deadlines",
		"task_breakdown":       "Break down the work into specific actionable tasks",
		"risk_assessment":      "Identify potential risks and mitigation strategies",
	}

	for _, info := range missingInfo {
		if description, exists := infoDescriptions[info]; exists {
			requests = append(requests, map[string]any{
				"info_type":   info,
				"description": description,
				"priority":    determineMissingInfoPriority(info),
			})
		}
	}

	return requests
}

func assessMissingInfoImpact(plan *ExecutionPlan, missingInfo []string) string {
	criticalInfo := []string{"plan_overview", "task_breakdown"}

	for _, info := range missingInfo {
		if contains(criticalInfo, info) {
			return "high"
		}
	}

	if len(missingInfo) > 2 {
		return "medium"
	}

	return "low"
}

func createPlanSnapshot(planID, reason string) {
	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	plan, exists := plannerState.ActivePlans[planID]
	if !exists {
		return
	}

	snapshot := PlanSnapshot{
		Timestamp: time.Now(),
		PlanID:    planID,
		Status:    plan.Status,
		Progress:  calculatePlanProgress(plan),
		Context:   copyMapSafe(plan.Context),
		Reason:    reason,
	}

	plannerState.PlanHistory = append(plannerState.PlanHistory, snapshot)

	// Keep only last 50 snapshots
	if len(plannerState.PlanHistory) > 50 {
		plannerState.PlanHistory = plannerState.PlanHistory[len(plannerState.PlanHistory)-50:]
	}
}

func updateRelatedPlans(taskID, status string, context map[string]any) []string {
	updatedPlans := make([]string, 0)

	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	for planID, plan := range plannerState.ActivePlans {
		for i := range plan.Subtasks {
			if plan.Subtasks[i].ID == taskID {
				plan.Subtasks[i].Status = status
				plan.Subtasks[i].Context = copyMapSafe(context)

				if status == "completed" {
					now := time.Now()
					plan.Subtasks[i].EndTime = &now
					if plan.Subtasks[i].StartTime != nil {
						duration := now.Sub(*plan.Subtasks[i].StartTime)
						plan.Subtasks[i].ActualDuration = &duration
					}
				} else if status == "in_progress" && plan.Subtasks[i].StartTime == nil {
					now := time.Now()
					plan.Subtasks[i].StartTime = &now
				}

				plan.UpdatedAt = time.Now()
				updatedPlans = append(updatedPlans, planID)

				createPlanSnapshot(planID, fmt.Sprintf("task_status_updated_%s", status))
				break
			}
		}
	}

	return updatedPlans
}

func assessPlanImpacts(updatedPlans []string, status string) []map[string]any {
	impacts := make([]map[string]any, 0)

	plannerState.mu.RLock()
	defer plannerState.mu.RUnlock()

	for _, planID := range updatedPlans {
		plan := plannerState.ActivePlans[planID]
		progress := calculatePlanProgress(plan)

		impact := map[string]any{
			"plan_id":  planID,
			"progress": progress,
			"status":   determinePlanStatus(plan),
		}

		if status == "completed" {
			impact["impact"] = "positive"
			impact["message"] = "Task completion advances plan progress"
		} else if status == "failed" {
			impact["impact"] = "negative"
			impact["message"] = "Task failure may block dependent tasks"
			impact["blocked_tasks"] = findDependentTasks(plan, "")
		}

		impacts = append(impacts, impact)
	}

	return impacts
}

func updatePlanProgress(taskID, phase string, progressValue float64) []map[string]any {
	updates := make([]map[string]any, 0)

	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	for planID, plan := range plannerState.ActivePlans {
		for i := range plan.Subtasks {
			if plan.Subtasks[i].ID == taskID {
				// Update subtask context with progress info
				if plan.Subtasks[i].Context == nil {
					plan.Subtasks[i].Context = make(map[string]any)
				}

				plan.Subtasks[i].Context["current_phase"] = phase
				plan.Subtasks[i].Context["progress"] = progressValue
				plan.Subtasks[i].Context["last_update"] = time.Now()

				plan.UpdatedAt = time.Now()

				updates = append(updates, map[string]any{
					"plan_id":          planID,
					"task_id":          taskID,
					"phase":            phase,
					"progress":         progressValue,
					"overall_progress": calculatePlanProgress(plan),
				})

				createPlanSnapshot(planID, "progress_updated")
				break
			}
		}
	}

	return updates
}

func calculateOverallProgress(planUpdates []map[string]any) float64 {
	if len(planUpdates) == 0 {
		return 0.0
	}

	totalProgress := 0.0
	for _, update := range planUpdates {
		if progress, ok := update["overall_progress"].(float64); ok {
			totalProgress += progress
		}
	}

	return totalProgress / float64(len(planUpdates))
}

func calculateCompletionConfidence(tracker *repository.TaskTracker) float64 {
	return 0.8
}

func analyzeCompletionMetrics(tracker *repository.TaskTracker) map[string]any {
	return map[string]any{
		"completion_indicators": []string{"basic_analysis"},
		"confidence_factors":    []string{"tracker_data_available"},
	}
}

func checkPlanCompletion(taskID string) map[string]any {
	plannerState.mu.RLock()
	defer plannerState.mu.RUnlock()

	for planID, plan := range plannerState.ActivePlans {
		for _, subtask := range plan.Subtasks {
			if subtask.ID == taskID {
				progress := calculatePlanProgress(plan)
				return map[string]any{
					"plan_id":               planID,
					"completion_percentage": progress * 100,
					"is_complete":           isPlanComplete(plan),
					"remaining_tasks":       countRemainingTasks(plan),
				}
			}
		}
	}
	return nil
}

func validateWorkflowDependencies(steps []WorkflowStep) error {
	stepIDs := make(map[string]bool)

	// Collect all step IDs
	for _, step := range steps {
		stepIDs[step.ID] = true
	}

	// Validate dependencies
	for _, step := range steps {
		for _, depID := range step.Dependencies {
			if !stepIDs[depID] {
				return fmt.Errorf("step %s depends on non-existent step %s", step.ID, depID)
			}
		}
	}

	// Check for circular dependencies
	if hasCycles(steps) {
		return fmt.Errorf("circular dependencies detected in workflow")
	}

	return nil
}

func hasCycles(steps []WorkflowStep) bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for _, step := range steps {
		if !visited[step.ID] {
			if hasCyclesDFS(step.ID, steps, visited, recStack) {
				return true
			}
		}
	}

	return false
}

func hasCyclesDFS(stepID string, steps []WorkflowStep, visited, recStack map[string]bool) bool {
	visited[stepID] = true
	recStack[stepID] = true

	// Find the step and check its dependencies
	for _, step := range steps {
		if step.ID == stepID {
			for _, depID := range step.Dependencies {
				if !visited[depID] {
					if hasCyclesDFS(depID, steps, visited, recStack) {
						return true
					}
				} else if recStack[depID] {
					return true
				}
			}
			break
		}
	}

	recStack[stepID] = false
	return false
}

func updateSubtasks(plan *ExecutionPlan, subtaskUpdates []any) {
	for _, updateInterface := range subtaskUpdates {
		if update, ok := updateInterface.(map[string]any); ok {
			subtaskID := getStringFromMapSafe(update, "id", "")

			for i := range plan.Subtasks {
				if plan.Subtasks[i].ID == subtaskID {
					// Update fields
					if desc, ok := update["description"].(string); ok {
						plan.Subtasks[i].Description = desc
					}
					if priority, ok := update["priority"].(string); ok {
						plan.Subtasks[i].Priority = priority
					}
					if status, ok := update["status"].(string); ok {
						plan.Subtasks[i].Status = status
					}
					if tools, ok := update["tools"].([]any); ok {
						plan.Subtasks[i].Tools = convertToStringSlice(tools)
					}
					break
				}
			}
		}
	}
}

func findDependentTasks(plan *ExecutionPlan, taskID string) []string {
	dependentTasks := make([]string, 0)

	for _, subtask := range plan.Subtasks {
		if contains(subtask.Dependencies, taskID) {
			dependentTasks = append(dependentTasks, subtask.ID)
		}
	}

	return dependentTasks
}

func determinePlanStatus(plan *ExecutionPlan) string {
	progress := calculatePlanProgress(plan)

	if progress == 1.0 {
		return "completed"
	}

	hasInProgress := false
	hasFailed := false

	for _, subtask := range plan.Subtasks {
		if subtask.Status == "in_progress" {
			hasInProgress = true
		}
		if subtask.Status == "failed" {
			hasFailed = true
		}
	}

	if hasFailed {
		return "blocked"
	}
	if hasInProgress {
		return "in_progress"
	}
	if progress > 0 {
		return "partially_complete"
	}

	return "pending"
}

func countRemainingTasks(plan *ExecutionPlan) int {
	remaining := 0
	for _, subtask := range plan.Subtasks {
		if subtask.Status != "completed" {
			remaining++
		}
	}
	return remaining
}

func parseSubtasks(subtasksData any) []SubTask {
	subtasks := make([]SubTask, 0)

	if subtaskSlice, ok := subtasksData.([]any); ok {
		for _, subtaskInterface := range subtaskSlice {
			if subtaskMap, ok := subtaskInterface.(map[string]any); ok {
				subtask := SubTask{
					ID:                  getStringFromMapSafe(subtaskMap, "id", fmt.Sprintf("subtask_%d", time.Now().UnixNano())),
					Description:         getStringFromMapSafe(subtaskMap, "description", ""),
					RequiredAgent:       getStringFromMapSafe(subtaskMap, "required_agent", "planner"),
					Priority:            getStringFromMapSafe(subtaskMap, "priority", "medium"),
					Dependencies:        getStringSliceFromMapSafe(subtaskMap, "dependencies"),
					EstimatedComplexity: getStringFromMapSafe(subtaskMap, "estimated_complexity", "moderate"),
					Tools:               getStringSliceFromMapSafe(subtaskMap, "tools"),
					AcceptanceCriteria:  getStringFromMapSafe(subtaskMap, "acceptance_criteria", ""),
					Status:              "pending",
					Context:             getMapFromMapSafe(subtaskMap, "context"),
				}

				// Parse duration if provided
				if durationStr, ok := subtaskMap["estimated_duration"].(string); ok {
					if duration, err := time.ParseDuration(durationStr); err == nil {
						subtask.EstimatedDuration = duration
					}
				}

				subtasks = append(subtasks, subtask)
			}
		}
	}

	return subtasks
}

func extractDependencies(subtasks []SubTask) map[string][]string {
	dependencies := make(map[string][]string)

	for _, subtask := range subtasks {
		if len(subtask.Dependencies) > 0 {
			dependencies[subtask.ID] = subtask.Dependencies
		}
	}

	return dependencies
}

func inferStepType(subtask SubTask) string {
	description := strings.ToLower(subtask.Description)

	if strings.Contains(description, "plan") || strings.Contains(description, "analyze") {
		return "planning"
	}
	if strings.Contains(description, "code") || strings.Contains(description, "implement") {
		return "implementation"
	}
	if strings.Contains(description, "test") || strings.Contains(description, "validate") {
		return "validation"
	}
	if strings.Contains(description, "deploy") || strings.Contains(description, "release") {
		return "deployment"
	}

	return "general"
}

func getDependenciesForValidation(subtasks []SubTask) []string {
	dependencies := make([]string, 0)

	for _, subtask := range subtasks {
		if subtask.RequiredAgent != "planner" || !strings.Contains(subtask.ID, "validate") {
			dependencies = append(dependencies, subtask.ID)
		}
	}

	return dependencies
}

func determineIterationAgent(iteration int, requiredAgents []string) string {
	if iteration == 1 && contains(requiredAgents, "planner") {
		return "planner"
	}
	if contains(requiredAgents, "coder") {
		return "coder"
	}
	return "planner"
}

func getIterationDependencies(iteration int, baseID string) []string {
	if iteration == 1 {
		return []string{fmt.Sprintf("%s_analyze", baseID)}
	}
	return []string{fmt.Sprintf("%s_iteration_%d", baseID, iteration-1)}
}

func getIterationTools(iteration int) []string {
	switch iteration {
	case 1:
		return []string{"prototyping", "initial_development"}
	case 2:
		return []string{"refinement", "optimization"}
	default:
		return []string{"finalization", "testing"}
	}
}

func determinePrimaryAgent(requiredAgents []string) string {
	if contains(requiredAgents, "coder") {
		return "coder"
	}
	return "planner"
}

func determineMissingInfoPriority(infoType string) string {
	criticalInfo := []string{"plan_overview", "task_breakdown"}

	if contains(criticalInfo, infoType) {
		return "high"
	}
	return "medium"
}

// Utility functions
func getStringFromMapSafe(m map[string]any, key, defaultVal string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultVal
}

func getStringSliceFromMapSafe(m map[string]any, key string) []string {
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

func getMapFromMapSafe(m map[string]any, key string) map[string]any {
	if val, ok := m[key].(map[string]any); ok {
		return val
	}
	return make(map[string]any)
}

func copyMapSafe(original map[string]any) map[string]any {
	if original == nil {
		return make(map[string]any)
	}

	copy := make(map[string]any)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

func convertToStringSlice(slice []any) []string {
	if slice == nil {
		return []string{}
	}

	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func mapToStruct(data map[string]any, tracker *repository.TaskTracker) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, tracker)
}

func generateDetailedSteps(description, strategy string, analysis TaskAnalysis) []TaskStep {
	steps := make([]TaskStep, 0)
	baseID := fmt.Sprintf("step_%d", time.Now().UnixNano())

	switch strategy {
	case "waterfall":
		steps = append(steps, generateWaterfallSteps(baseID, description, analysis)...)
	case "agile":
		steps = append(steps, generateAgileSteps(baseID, description, analysis)...)
	case "spiral":
		steps = append(steps, generateSpiralSteps(baseID, description, analysis)...)
	default:
		steps = append(steps, generateDefaultSteps(baseID, description, analysis)...)
	}

	return steps
}

func generateWaterfallSteps(baseID, description string, analysis TaskAnalysis) []TaskStep {
	return []TaskStep{
		{
			ID:           fmt.Sprintf("%s_requirements", baseID),
			Order:        0,
			Description:  fmt.Sprintf("Gather requirements for: %s", description),
			Type:         "requirements",
			AgentType:    "planner",
			Dependencies: []string{},
			Status:       "pending",
		},
		{
			ID:           fmt.Sprintf("%s_design", baseID),
			Order:        1,
			Description:  fmt.Sprintf("Design solution for: %s", description),
			Type:         "design",
			AgentType:    "planner",
			Dependencies: []string{fmt.Sprintf("%s_requirements", baseID)},
			Status:       "pending",
		},
		{
			ID:           fmt.Sprintf("%s_implementation", baseID),
			Order:        2,
			Description:  fmt.Sprintf("Implement: %s", description),
			Type:         "implementation",
			AgentType:    determinePrimaryAgent(analysis.RequiredAgents),
			Dependencies: []string{fmt.Sprintf("%s_design", baseID)},
			Status:       "pending",
		},
		{
			ID:           fmt.Sprintf("%s_testing", baseID),
			Order:        3,
			Description:  fmt.Sprintf("Test implementation of: %s", description),
			Type:         "testing",
			AgentType:    "planner",
			Dependencies: []string{fmt.Sprintf("%s_implementation", baseID)},
			Status:       "pending",
		},
	}
}

func generateAgileSteps(baseID, description string, analysis TaskAnalysis) []TaskStep {
	sprintCount := 3
	steps := make([]TaskStep, 0)

	// Initial planning
	steps = append(steps, TaskStep{
		ID:           fmt.Sprintf("%s_backlog", baseID),
		Order:        0,
		Description:  fmt.Sprintf("Create backlog for: %s", description),
		Type:         "backlog_creation",
		AgentType:    "planner",
		Dependencies: []string{},
		Status:       "pending",
	})

	// Sprint cycles
	for i := 1; i <= sprintCount; i++ {
		sprintID := fmt.Sprintf("%s_sprint_%d", baseID, i)
		var deps []string

		if i == 1 {
			deps = []string{fmt.Sprintf("%s_backlog", baseID)}
		} else {
			deps = []string{fmt.Sprintf("%s_sprint_%d", baseID, i-1)}
		}

		steps = append(steps, TaskStep{
			ID:           sprintID,
			Order:        i,
			Description:  fmt.Sprintf("Sprint %d: Develop %s", i, description),
			Type:         "sprint",
			AgentType:    determinePrimaryAgent(analysis.RequiredAgents),
			Dependencies: deps,
			Status:       "pending",
		})
	}

	return steps
}

func generateSpiralSteps(baseID, description string, analysis TaskAnalysis) []TaskStep {
	cycles := 2
	steps := make([]TaskStep, 0)

	for cycle := 1; cycle <= cycles; cycle++ {
		phases := []string{"planning", "risk_analysis", "engineering", "evaluation"}

		for j, phase := range phases {
			stepID := fmt.Sprintf("%s_cycle_%d_%s", baseID, cycle, phase)
			var deps []string

			if cycle == 1 && j == 0 {
				deps = []string{}
			} else if j == 0 {
				deps = []string{fmt.Sprintf("%s_cycle_%d_evaluation", baseID, cycle-1)}
			} else {
				deps = []string{fmt.Sprintf("%s_cycle_%d_%s", baseID, cycle, phases[j-1])}
			}

			steps = append(steps, TaskStep{
				ID:           stepID,
				Order:        (cycle-1)*len(phases) + j,
				Description:  fmt.Sprintf("Cycle %d - %s: %s", cycle, strings.Title(phase), description),
				Type:         phase,
				AgentType:    determinePhaseAgent(phase, analysis.RequiredAgents),
				Dependencies: deps,
				Status:       "pending",
			})
		}
	}

	return steps
}

func generateDefaultSteps(baseID, description string, analysis TaskAnalysis) []TaskStep {
	return []TaskStep{
		{
			ID:           fmt.Sprintf("%s_analyze", baseID),
			Order:        0,
			Description:  fmt.Sprintf("Analyze: %s", description),
			Type:         "analysis",
			AgentType:    "planner",
			Dependencies: []string{},
			Status:       "pending",
		},
		{
			ID:           fmt.Sprintf("%s_execute", baseID),
			Order:        1,
			Description:  fmt.Sprintf("Execute: %s", description),
			Type:         "execution",
			AgentType:    determinePrimaryAgent(analysis.RequiredAgents),
			Dependencies: []string{fmt.Sprintf("%s_analyze", baseID)},
			Status:       "pending",
		},
	}
}

func determineBreakdownStrategy(description string, analysis TaskAnalysis) string {
	description = strings.ToLower(description)

	if strings.Contains(description, "agile") || strings.Contains(description, "sprint") {
		return "agile"
	}
	if strings.Contains(description, "spiral") || analysis.ComplexityLevel == "very_complex" {
		return "spiral"
	}
	if analysis.ComplexityLevel == "complex" {
		return "waterfall"
	}

	return "sequential"
}

func determinePhaseAgent(phase string, requiredAgents []string) string {
	switch phase {
	case "planning", "risk_analysis", "evaluation":
		return "planner"
	case "engineering":
		if contains(requiredAgents, "coder") {
			return "coder"
		}
		return "planner"
	default:
		return "planner"
	}
}

// Enhanced planner state management functions
func (p *PlannerAgent) GetPlannerState() map[string]any {
	plannerState.mu.RLock()
	defer plannerState.mu.RUnlock()
	return map[string]any{
		"active_plans":    len(plannerState.ActivePlans),
		"task_breakdowns": len(plannerState.TaskBreakdowns),
		"workflow_states": len(plannerState.WorkflowStates),
		"plan_history":    len(plannerState.PlanHistory),
		"metrics":         plannerState.Metrics,
	}
}

func (p *PlannerAgent) GetActivePlans() map[string]*ExecutionPlan {
	plannerState.mu.RLock()
	defer plannerState.mu.RUnlock()
	return plannerState.ActivePlans
}

func (p *PlannerAgent) GetPlanByID(planID string) (*ExecutionPlan, bool) {
	plannerState.mu.RLock()
	defer plannerState.mu.RUnlock()
	plan, exists := plannerState.ActivePlans[planID]
	return plan, exists
}

func (p *PlannerAgent) ArchiveCompletedPlans() int {
	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	archived := 0

	for planID, plan := range plannerState.ActivePlans {
		if isPlanComplete(plan) {
			// Move to history
			createPlanSnapshot(planID, "archived")
			delete(plannerState.ActivePlans, planID)
			archived++
		}
	}
	savePlannerState()
	return archived
}

func (p *PlannerAgent) GetPlannerMetrics() *PlannerMetrics {
	plannerState.mu.Lock()
	defer plannerState.mu.Unlock()

	// Update real-time metrics
	plannerState.Metrics.LastActivity = time.Now()

	totalDuration := time.Duration(0)
	completedCount := 0

	for _, plan := range plannerState.ActivePlans {
		if isPlanComplete(plan) {
			if plan.Timeline != nil {
				duration := plan.Timeline.EndDate.Sub(plan.Timeline.StartDate)
				totalDuration += duration
				completedCount++
			}
		}
	}

	if completedCount > 0 {
		plannerState.Metrics.AveragePlanDuration = totalDuration / time.Duration(completedCount)
	}
	savePlannerState()
	return plannerState.Metrics
}

func (p *PlannerAgent) ToMap() map[string]any {
	fmt.Printf("Planner Agent Created: %v\n", p.Metadata.ID)

	return map[string]any{
		"id":              p.Metadata.ID,
		"name":            p.Metadata.Name,
		"version":         p.Metadata.Version,
		"type":            p.Metadata.Type,
		"instructions":    p.Metadata.Instructions,
		"capabilities":    p.Capabilities,
		"max_concurrency": p.Metadata.MaxConcurrency,
		"timeout":         p.Metadata.Timeout.String(),
		"tags":            p.Metadata.Tags,
		"endpoints":       p.Metadata.Endpoints,
		"status":          p.Metadata.Status,

		"last_active":   p.Metadata.LastActive,
		"planner_state": p.GetPlannerState(),
	}
}

func GetPlannerMap() map[string]any {
	var pagent PlannerAgent
	agent := pagent.NewAgent()

	// Marshal capabilities to JSON
	capsJSON, err := json.Marshal(agent.Capabilities)
	if err != nil {
		return map[string]any{
			"agent":        plannerName,
			"metadata":     agent.Metadata,
			"capabilities": []map[string]any{}, // fallback: empty
		}
	}

	// Unmarshal JSON back into []map[string]any
	var caps []map[string]any
	if err := json.Unmarshal(capsJSON, &caps); err != nil {
		caps = []map[string]any{}
	}

	return map[string]any{
		"agent":         plannerName,
		"metadata":      agent.Metadata,
		"capabilities":  caps,
		"planner_state": agent.GetPlannerState(),
	}
}

// --------------------------- Enhanced Capability Constants ---------------------------
const (
	// Core planning capabilities
	CreateTaskName             = "create_task"
	BreakDownTaskName          = "breakdown_task"
	UpdateTaskStatusName       = "update_task_status"
	TrackProgressName          = "track_progress"
	EvaluateTaskCompletionName = "evaluate_task_completion"

	// Advanced planning capabilities
	DecomposeTaskName        = "decompose_task"
	CreateWorkflowName       = "create_workflow"
	UpdatePlanName           = "update_plan"
	GetNextStepName          = "get_next_step"
	EvaluatePlanProgressName = "evaluate_plan_progress"
	RequestMissingInfoName   = "request_missing_info"
	RequestUserInputName     = "request_user_input"

	// State management capabilities
	GetPlannerStateName       = "get_planner_state"
	GetActivePlansName        = "get_active_plans"
	ArchiveCompletedPlansName = "archive_completed_plans"
	GetPlannerMetricsName     = "get_planner_metrics"

	// Plan lifecycle capabilities
	PausePlanName  = "pause_plan"
	ResumePlanName = "resume_plan"
	CancelPlanName = "cancel_plan"
	ClonePlanName  = "clone_plan"
)

// --------------------------- Capability Descriptions ---------------------------

const CreateTaskDescription = `
Create a new structured task with optional execution planning.
Can include detailed execution plans with subtasks, risk factors, and success metrics.
Required: "title". Optional: "created_by", "planning_complete", "execution_plan", "next_step"
`

const BreakDownTaskDescription = `
Break down a high-level task into smaller, manageable subtasks with intelligent analysis.
Analyzes complexity, determines strategy, and creates detailed breakdown with estimates.
Required: "task_id", "description". Optional: "complexity", "domain"
`

const UpdateTaskStatusDescription = `
Update task status with context awareness and plan impact analysis.
Automatically updates related plans and assesses impacts on overall progress.
Required: "task_id", "status". Optional: "context"
`

const TrackProgressDescription = `
Log detailed progress with metrics and plan updates.
Tracks phase completion, updates related plans, and calculates overall progress.
Required: "task_id", "phase". Optional: "progress", "metrics"
`

const EvaluateTaskCompletionDescription = `
Comprehensive task completion evaluation with confidence metrics.
Analyzes completion indicators and checks plan-level completion status.
Required: "tracker" (TaskTracker object)
`

const DecomposeTaskDescription = `
Advanced task decomposition with intelligent strategy selection.
Analyzes complexity, selects optimal breakdown strategy, and generates detailed subtasks.
Required: "description". Optional: "complexity", "domain", "constraints"
`

const CreateWorkflowDescription = `
Create comprehensive workflows with dependency validation.
Validates dependencies, checks for cycles, and creates executable workflows.
Required: "name". Optional: "description", "steps", "context"
`

const UpdatePlanDescription = `
Update existing execution plans with versioning and impact analysis.
Maintains plan history and analyzes update impacts on progress and dependencies.
Required: "plan_id", "updates" (map of fields to update)
`

const GetNextStepDescription = `
Find the next actionable step in a plan based on dependencies and agent capabilities.
Considers agent availability, dependencies, and priority to determine optimal next action.
Required: "plan_id". Optional: "agent_capabilities"
`

const EvaluatePlanProgressDescription = `
Comprehensive plan progress evaluation with blocker and risk analysis.
Identifies blockers, assesses risks, and provides actionable recommendations.
Required: "plan_id"
`

const RequestMissingInfoDescription = `
Identify and request missing information for effective planning.
Analyzes plan completeness and generates specific information requests.
Required: "plan_id". Optional: "info_type"
`

// --------------------------- Plan Lifecycle Capability Descriptions ---------------------------

const PausePlanDescription = `
Pause the execution of an active plan.
Required: "plan_id"
`

const ResumePlanDescription = `
Resume a paused plan's execution.
Required: "plan_id"
`

const CancelPlanDescription = `
Cancel a plan's execution and mark it as terminated.
Required: "plan_id"
`

const ClonePlanDescription = `
Create a new, duplicate plan from an existing plan.
Required: "plan_id"
`
const RequestUserInputDescription = `
Prompts the user for a single line of input from the terminal and returns the response.
`

// --------------------------- Enhanced Planner Capability Array ---------------------------

var EnhancedPlannerCapabilities = []repository.Function{
	// Core task management
	{
		Name:        CreateTaskName,
		Description: CreateTaskDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"title": {
					Type:        "string",
					Description: "Title or name of the task (required).",
				},
				"created_by": {
					Type:        "string",
					Description: "Optional: Identifier of the task creator.",
				},
				"planning_complete": {
					Type:        "boolean",
					Description: "Optional: Whether the planning phase is complete.",
				},
				"execution_plan": {
					Type:        "object",
					Description: "Optional: Detailed execution plan with subtasks, overview, and metrics.",

					Properties: map[string]*repository.Properties{
						"overview": {
							Type:        "string",
							Description: "High-level overview of the plan",
						},
						"subtasks": {
							Type:        "array",
							Description: "Array of subtask objects",
						},
						"risk_factors": {
							Type:        "array",
							Description: "Array of identified risk factors",
						},
						"success_metrics": {
							Type:        "string",
							Description: "Measurable success criteria",
						},
					},
				},
				"next_step": {
					Type:        "string",
					Description: "Optional: Description of what should happen next.",
				},
			},
			Required: []string{"title"},
		},
		Service: CreateTask,
	},

	{
		Name:        BreakDownTaskName,
		Description: BreakDownTaskDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"task_id": {
					Type:        "string",
					Description: "Unique identifier of the task to break down.",
				},
				"description": {
					Type:        "string",
					Description: "Description or context of the task to break down.",
				},
				"complexity": {
					Type:        "string",
					Description: "Optional: Expected complexity level (simple, moderate, complex, very_complex).",
				},
				"domain": {
					Type:        "string",
					Description: "Optional: Domain or area of the task (e.g., 'web_development', 'data_analysis').",
				},
			},
			Required: []string{"task_id", "description"},
		},
		Service: BreakDownTask,
	},

	{
		Name:        UpdateTaskStatusName,
		Description: UpdateTaskStatusDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"task_id": {
					Type:        "string",
					Description: "Unique identifier of the task to update.",
				},
				"status": {
					Type:        "string",
					Description: "The new status for the task (pending, in_progress, completed, failed).",
					Enum:        []string{"pending", "in_progress", "completed", "failed", "paused", "canceled"},
				},
				"context": {
					Type:        "object",
					Description: "Optional: Additional context about the status update.",
				},
			},
			Required: []string{"task_id", "status"},
		},
		Service: UpdateTaskStatus,
	},

	{
		Name:        TrackProgressName,
		Description: TrackProgressDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"task_id": {
					Type:        "string",
					Description: "Unique identifier of the task being tracked.",
				},
				"phase": {
					Type:        "string",
					Description: "Current phase of the task being logged.",
				},
				"progress": {
					Type:        "number",
					Description: "Optional: Progress value between 0.0 and 1.0.",
				},
				"metrics": {
					Type:        "object",
					Description: "Optional: Additional metrics and measurements.",
				},
			},
			Required: []string{"task_id", "phase"},
		},
		Service: TrackProgress,
	},

	{
		Name:        EvaluateTaskCompletionName,
		Description: EvaluateTaskCompletionDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"tracker": {
					Type:        "object",
					Description: "TaskTracker object containing evaluation data.",
				},
			},
			Required: []string{"tracker"},
		},
		Service: EvaluateTaskCompletion,
	},
	{
		Name:        DecomposeTaskName,
		Description: DecomposeTaskDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"description": {
					Type:        "string",
					Description: "The description of the task to decompose.",
				},
				"complexity": {
					Type:        "string",
					Description: "Optional: The expected complexity level of the task.",
				},
				"domain": {
					Type:        "string",
					Description: "Optional: The domain or area of the task.",
				},
				"constraints": {
					Type:        "array",
					Description: "Optional: Any specific constraints for decomposition.",
				},
			},
			Required: []string{"description"},
		},
		Service: DecomposeTask,
	},
	{
		Name:        CreateWorkflowName,
		Description: CreateWorkflowDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"name": {
					Type:        "string",
					Description: "The name of the new workflow.",
				},
				"description": {
					Type:        "string",
					Description: "A detailed description of the workflow.",
				},
				"steps": {
					Type:        "array",
					Description: "An array of workflow step objects.",
				},
				"context": {
					Type:        "object",
					Description: "Optional: Contextual information for the workflow.",
				},
			},
			Required: []string{"name", "steps"},
		},
		Service: CreateWorkflow,
	},
	{
		Name:        UpdatePlanName,
		Description: UpdatePlanDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"plan_id": {
					Type:        "string",
					Description: "The unique identifier of the plan to update.",
				},
				"updates": {
					Type:        "object",
					Description: "A map of fields to update, e.g., {'status': 'completed'}.",
				},
			},
			Required: []string{"plan_id", "updates"},
		},
		Service: UpdatePlan,
	},
	{
		Name:        GetNextStepName,
		Description: GetNextStepDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"plan_id": {
					Type:        "string",
					Description: "The unique identifier of the plan.",
				},
				"agent_capabilities": {
					Type:        "array",
					Description: "Optional: A list of capabilities of the requesting agent.",
				},
			},
			Required: []string{"plan_id"},
		},
		Service: GetNextStep,
	},
	{
		Name:        EvaluatePlanProgressName,
		Description: EvaluatePlanProgressDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"plan_id": {
					Type:        "string",
					Description: "The unique identifier of the plan to evaluate.",
				},
			},
			Required: []string{"plan_id"},
		},
		Service: EvaluatePlanProgress,
	},
	{
		Name:        RequestMissingInfoName,
		Description: RequestMissingInfoDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"plan_id": {
					Type:        "string",
					Description: "The unique identifier of the plan that needs information.",
				},
				"info_type": {
					Type:        "string",
					Description: "The type of information missing (e.g., 'requirements', 'resources').",
				},
			},
			Required: []string{"plan_id"},
		},
		Service: RequestMissingInfo,
	},
	{
		Name:        PausePlanName,
		Description: PausePlanDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"plan_id": {
					Type:        "string",
					Description: "The ID of the plan to pause.",
				},
			},
			Required: []string{"plan_id"},
		},
		Service: PausePlan,
	},
	{
		Name:        ResumePlanName,
		Description: ResumePlanDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"plan_id": {
					Type:        "string",
					Description: "The ID of the plan to resume.",
				},
			},
			Required: []string{"plan_id"},
		},
		Service: ResumePlan,
	},
	{
		Name:        CancelPlanName,
		Description: CancelPlanDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"plan_id": {
					Type:        "string",
					Description: "The ID of the plan to cancel.",
				},
			},
			Required: []string{"plan_id"},
		},
		Service: CancelPlan,
	},
	{
		Name:        ClonePlanName,
		Description: ClonePlanDescription,
		Parameters: repository.Parameters{
			Type: "object",
			Properties: map[string]*repository.Properties{
				"plan_id": {
					Type:        "string",
					Description: "The ID of the plan to clone.",
				},
			},
			Required: []string{"plan_id"},
		},
		Service: ClonePlan,
	},
	{
		Name:        GetPlannerStateName,
		Description: "Retrieve the current high-level state of the planner.",
		Parameters: repository.Parameters{
			Type: "object",
		},
		Service: func(data map[string]any) map[string]any {
			return plannerState.GetPlannerState()
		},
	},
	{
		Name:        GetActivePlansName,
		Description: "Get a list of all active plans with their details.",
		Parameters: repository.Parameters{
			Type: "object",
		},
		Service: func(data map[string]any) map[string]any {
			return map[string]any{
				"active_plans": plannerState.GetActivePlans(),
			}
		},
	},
	{
		Name:        ArchiveCompletedPlansName,
		Description: "Archive all completed plans to clean up the active plans list.",
		Parameters: repository.Parameters{
			Type: "object",
		},
		Service: func(data map[string]any) map[string]any {
			count := plannerState.ArchiveCompletedPlans()
			return map[string]any{
				"status": "archived",
				"count":  count,
			}
		},
	},
	{
		Name:        GetPlannerMetricsName,
		Description: "Get performance metrics for the planner, such as completion rates.",
		Parameters: repository.Parameters{
			Type: "object",
		},
		Service: func(data map[string]any) map[string]any {
			return map[string]any{
				"metrics": plannerState.GetPlannerMetrics(),
			}
		},
	},
	{
		Name:        RequestUserInputName,
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
		Service: func(data map[string]any) map[string]any {
			return nil
		},
	},
}

func GetPlannerCapabilities() []repository.Function {
	return EnhancedPlannerCapabilities
}

func GetPlannerCapabilitiesArrayMap() []map[string]any {
	plannerMap := make([]map[string]any, 0)
	for _, v := range EnhancedPlannerCapabilities {
		plannerMap = append(plannerMap, v.ToObject())
	}
	return plannerMap
}

func GetPlannerCapabilitiesMap() map[string]repository.Function {
	plannerMap := make(map[string]repository.Function)
	for _, v := range EnhancedPlannerCapabilities {
		plannerMap[v.Name] = v
	}
	return plannerMap
}
