package orchestrator

// import (
// 	"dost/internal/repository"
// )

// // --------------------------- Capability Constants ---------------------------

// // RegisterAgent capability constants
// const RegisterAgentName = "register_agent"
// const RegisterAgentDescription = `
// Register a new agent with the 
// This function ensures the orchestrator is aware of the agent, its capabilities, and metadata.
// Important: "agent_metadata.id" must be unique across the system.
// `

// // DeregisterAgent capability constants
// const DeregisterAgentName = "deregister_agent"
// const DeregisterAgentDescription = `
// Remove an agent from the orchestrator registry.
// After deregistration, the orchestrator will not route tasks to this agent.
// `

// // RouteTask capability constants
// const RouteTaskName = "route_task"
// const RouteTaskDescription = `
// Automatically route a task to the most suitable agent based on its metadata and capabilities.
// This is used for dynamic agent selection when the orchestrator decides the best match.
// `

// // AssignTask capability constants
// const AssignTaskName = "assign_task"
// const AssignTaskDescription = `
// Assign a specific task directly to a known agent.
// This bypasses routing logic and explicitly binds a task to a target agent_id.
// `

// // SendMessage capability constants
// const SendMessageName = "send_message"
// const SendMessageDescription = `
// Send a message from the orchestrator (or another agent) to a target agent or external 
// Useful for communication, signaling, or coordination across agents.
// `

// // ShareContext capability constants
// const ShareContextName = "share_context"
// const ShareContextDescription = `
// Share a local context object with another agent.
// This allows one agent to provide partial knowledge/state to another for collaboration.
// `

// // MergeContexts capability constants
// const MergeContextsName = "merge_contexts"
// const MergeContextsDescription = `
// Merge multiple context objects into one combined context.
// This is typically used when multiple agents contribute to the same task.
// `

// // UpdateGlobalContext capability constants
// const UpdateGlobalContextName = "update_global_context"
// const UpdateGlobalContextDescription = `
// Update the orchestrator's global context state with new or updated fields.
// This represents shared state accessible across all agents in the system.
// `

// // GetRelevantContext capability constants
// const GetRelevantContextName = "get_relevant_context"
// const GetRelevantContextDescription = `
// Retrieve the most relevant global context for a given task.
// This helps agents get additional knowledge/state to complete their work more effectively.
// `

// // StartWorkflow capability constants
// const StartWorkflowName = "start_workflow"
// const StartWorkflowDescription = `
// Start a multi-step workflow involving multiple agents.
// A workflow defines an ordered sequence of tasks that may be routed or assigned to multiple agents.
// `

// // UpdateAgentContext capability constants
// const UpdateAgentContextName = "update_agent_context"
// const UpdateAgentContextDescription = `
// Update the context state for a specific agent.
// This allows agents to maintain their own state and share it with the 
// `

// // GetAgentContext capability constants
// const GetAgentContextName = "get_agent_context"
// const GetAgentContextDescription = `
// Retrieve the context state for a specific agent.
// This allows other agents or the orchestrator to access an agent's current state.
// `

// // UpdateWorkflowContext capability constants
// const UpdateWorkflowContextName = "update_workflow_context"
// const UpdateWorkflowContextDescription = `
// Update the context state for a specific workflow.
// This allows workflow steps to maintain shared state throughout the workflow execution.
// `

// // GetContextHistory capability constants
// const GetContextHistoryName = "get_context_history"
// const GetContextHistoryDescription = `
// Retrieve historical snapshots of context states.
// This provides audit trail and debugging capabilities for context changes over time.
// `

// // --------------------------- Orchestrator Capability Map ---------------------------

// var OrchestratorCapabilities = []repository.Function{
// 	{
// 		Name:        RegisterAgentName,
// 		Description: RegisterAgentDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"agent_names": {
// 					Type: repository.TypeArray,
// 					Items: &repository.Properties{
// 						Type: repository.TypeString,
// 						Enum: []string{string(repository.AgentPlanner), string(repository.AgentCoder), string(repository.AgentCritic), string(repository.AgentKnowledge), string(repository.AgentCritic)},
// 					},
// 					Description: "Name of the agent",
// 				},
// 			},
// 			Required: []string{"agent_names"},
// 		},
// 		Service: RegisterAgent,
// 	},
// 	{
// 		Name:        DeregisterAgentName,
// 		Description: DeregisterAgentDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"agent_id": {
// 					Type:        repository.TypeString,
// 					Description: "Agent ID from the registered Agents",
// 					Enum:        []string{},
// 				},
// 			},
// 			Required: []string{"agent_id"},
// 		},
// 		Service: DeregisterAgent,
// 	},
// 	{
// 		Name:        RouteTaskName,
// 		Description: RouteTaskDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"task": {
// 					Type:        repository.TypeObject,
// 					Description: "The task object (including fields like `description`, `priority`, etc.) to be routed.",
// 				},
// 				"agent_id": {
// 					Type:        repository.TypeString,
// 					Description: "Agent ID from the registered Agents",
// 					Enum:        []string{},
// 				},
// 				"context": {
// 					Type:        repository.TypeObject,
// 					Description: "Context object containing relevant state information for task routing.",
// 				},
// 			},
// 			Required: []string{"task", "agent_id", "context"},
// 		},
// 		Service: RouteTask,
// 	},
// 	{
// 		Name:        MergeContextsName,
// 		Description: MergeContextsDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"contexts": {
// 					Type: repository.TypeArray,
// 					Items: &repository.Properties{
// 						Type:        repository.TypeObject,
// 						Description: "Individual context object from an agent.",
// 					},
// 					Description: "Array of context objects to merge into one.",
// 				},
// 			},
// 			Required: []string{"contexts"},
// 		},
// 		Service: MergeContexts,
// 	},
// 	{
// 		Name:        UpdateGlobalContextName,
// 		Description: UpdateGlobalContextDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"context": {
// 					Type:        repository.TypeObject,
// 					Description: "The new context object that should be merged into the orchestrator's global context.",
// 				},
// 			},
// 			Required: []string{"context"},
// 		},
// 		Service: UpdateGlobalContext,
// 	},
// 	{
// 		Name:        GetRelevantContextName,
// 		Description: GetRelevantContextDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"task": {
// 					Type:        repository.TypeObject,
// 					Description: "The task for which context should be retrieved.",
// 				},
// 				"agent_id": {
// 					Type:        "string",
// 					Description: "Optional: The agent ID requesting the context.",
// 				},
// 			},
// 			Required: []string{"task"},
// 		},
// 		Service: GetRelevantContext,
// 	},
// 	{
// 		Name:        StartWorkflowName,
// 		Description: StartWorkflowDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"workflow": {
// 					Type:        repository.TypeObject,
// 					Description: "The workflow object containing ordered steps and tasks across agents.",
// 				},
// 			},
// 			Required: []string{"workflow"},
// 		},
// 		Service: StartWorkflow,
// 	},
// 	{
// 		Name:        UpdateAgentContextName,
// 		Description: UpdateAgentContextDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"agent_id": {
// 					Type:        "string",
// 					Description: "The unique ID of the agent whose context should be updated.",
// 				},
// 				"context": {
// 					Type:        repository.TypeObject,
// 					Description: "The context object to update for the agent.",
// 				},
// 			},
// 			Required: []string{"agent_id", "context"},
// 		},
// 		Service: UpdateAgentContext,
// 	},
// 	{
// 		Name:        GetAgentContextName,
// 		Description: GetAgentContextDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"agent_id": {
// 					Type:        "string",
// 					Description: "The unique ID of the agent whose context should be retrieved.",
// 				},
// 			},
// 			Required: []string{"agent_id"},
// 		},
// 		Service: GetAgentContext,
// 	},
// 	{
// 		Name:        UpdateWorkflowContextName,
// 		Description: UpdateWorkflowContextDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"workflow_id": {
// 					Type:        "string",
// 					Description: "The unique ID of the workflow whose context should be updated.",
// 				},
// 				"context": {
// 					Type:        repository.TypeObject,
// 					Description: "The context object to update for the workflow.",
// 				},
// 			},
// 			Required: []string{"workflow_id", "context"},
// 		},
// 		Service: UpdateWorkflowContext,
// 	},
// 	{
// 		Name:        GetContextHistoryName,
// 		Description: GetContextHistoryDescription,
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"limit": {
// 					Type:        "number",
// 					Description: "Optional: Maximum number of historical snapshots to retrieve (default: 10).",
// 				},
// 			},
// 			Required: []string{},
// 		},
// 		Service: GetContextHistory,
// 	},
// 	{
// 		Name:        "get_cached_responses",
// 		Description: "Retrieve cached responses for a specific agent with optional filtering",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"agent_id": {
// 					Type:        repository.TypeString,
// 					Description: "Agent ID to retrieve cached responses for",
// 				},
// 				"limit": {
// 					Type:        repository.TypeNumber,
// 					Description: "Maximum number of responses to return (default: 10)",
// 				},
// 				"include_raw": {
// 					Type:        repository.TypeBoolean,
// 					Description: "Whether to include raw response data (default: false)",
// 				},
// 			},
// 			Required: []string{"agent_id"},
// 		},
// 		Service: GetCachedResponsesForAgent,
// 	},
// 	{
// 		Name:        "search_cached_responses",
// 		Description: "Search for cached responses similar to a given task",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"query": {
// 					Type:        repository.TypeObject,
// 					Description: "Task object to search for similar cached responses",
// 				},
// 				"agent_id": {
// 					Type:        repository.TypeString,
// 					Description: "Optional: Filter by specific agent",
// 				},
// 				"similarity": {
// 					Type:        repository.TypeNumber,
// 					Description: "Minimum similarity threshold (0.0-1.0, default: 0.7)",
// 				},
// 			},
// 			Required: []string{"query"},
// 		},
// 		Service: SearchCachedResponses,
// 	},
// 	{
// 		Name:        "get_cache_statistics",
// 		Description: "Get comprehensive statistics about the response cache",
// 		Parameters: repository.Parameters{
// 			Type:       repository.TypeObject,
// 			Properties: map[string]*repository.Properties{},
// 			Required:   []string{},
// 		},
// 		Service: func(data map[string]any) map[string]any {
// 			return GetCacheStatistics()
// 		},
// 	},
// 	{
// 		Name:        "clear_expired_cache",
// 		Description: "Remove expired cached responses to free up space",
// 		Parameters: repository.Parameters{
// 			Type:       repository.TypeObject,
// 			Properties: map[string]*repository.Properties{},
// 			Required:   []string{},
// 		},
// 		Service: func(data map[string]any) map[string]any {
// 			return ClearExpiredResponses()
// 		},
// 	},
// 	{
// 		Name:        "optimize_cache",
// 		Description: "Optimize the response cache by applying eviction policies and rebuilding indexes",
// 		Parameters: repository.Parameters{
// 			Type:       repository.TypeObject,
// 			Properties: map[string]*repository.Properties{},
// 			Required:   []string{},
// 		},
// 		Service: OptimizeCache,
// 	},
// 	{
// 		Name:        "purge_agent_cache",
// 		Description: "Remove all cached responses for a specific agent",
// 		Parameters: repository.Parameters{
// 			Type: repository.TypeObject,
// 			Properties: map[string]*repository.Properties{
// 				"agent_id": {
// 					Type:        repository.TypeString,
// 					Description: "Agent ID whose cache should be purged",
// 				},
// 			},
// 			Required: []string{"agent_id"},
// 		},
// 		Service: PurgeCacheByAgent,
// 	},
// }

// // --------------------------- Helper Functions ---------------------------

// // func GetOrchestratorCapabilities() []repository.Function {
// // 	return OrchestratorCapabilities
// // }
// func GetOrchestratorCapabilitiesArrayMap() []map[string]any {
// 	orchestratorMap := make([]map[string]any, 0)

// 	for _, v := range OrchestratorCapabilities {

// 		orchestratorMap = append(orchestratorMap, v.ToObject())
// 	}

// 	return orchestratorMap
// }

// func GetOrchestratorCapabilitiesMap() map[string]repository.Function {
// 	orchestratorMap := make(map[string]repository.Function)
// 	for _, v := range OrchestratorCapabilities {
// 		orchestratorMap[v.Name] = v
// 	}
// 	return orchestratorMap
// }
