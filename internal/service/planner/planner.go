package planner

import "dost/internal/repository"

type AgentPlanner repository.Agent

func Instruction(map[string]any) map[string]any {
	
	return map[string]any{
		"output": "",
		"error":  "",
	}
}
