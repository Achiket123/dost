package orchestrator

// const orchestratorName = "orchestrator"
// const orchestratorVersion = "0.1.0"

// type OrchestratorAgent repository.Agent

// func (o *OrchestratorAgent) NewAgent() *OrchestratorAgent {

// 	model := viper.GetString("ORCHESTRATOR.MODEL")
// 	endPoints := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
// 	agentMetadata := repository.AgentMetadata{
// 		ID:             fmt.Sprintf("orchestrator-%s", uuid.NewString()),
// 		Name:           orchestratorName,
// 		Version:        orchestratorVersion,
// 		Type:           repository.AgentOrchestrator,
// 		Instructions:   repository.OrchestratorInstructions,
// 		MaxConcurrency: 3,
// 		Timeout:        5 * time.Minute,
// 		Tags:           []string{"orchestrator", "agent"},
// 		Endpoints:      map[string]string{"http": endPoints},
// 	}

// 	agent := repository.Agent{
// 		Metadata:     agentMetadata,
// 		Capabilities: GetOrchestratorCapabilities(),
// 	}
// 	orchestratorAgent := OrchestratorAgent(agent)
// 	return &orchestratorAgent

// }

// func (o *OrchestratorAgent) RequestAgent(contents []map[string]any) map[string]any {
// 	fmt.Println("REQ :", o.Metadata.Name)

// 	request := map[string]any{
// 		"systemInstruction": map[string]any{
// 			"parts": []map[string]any{
// 				{"text": o.Metadata.Instructions},
// 			},
// 		},
// 		"contents": contents,
// 		"tools": []map[string]any{
// 			{"function_declarations": GetOrchestratorCapabilitiesArrayMap()},
// 		},
// 	}

// 	jsonBody, err := json.Marshal(request)

// 	if err != nil {
// 		return map[string]any{"error": err.Error(), "output": nil}
// 	}

// 	req, err := http.NewRequestWithContext(context.Background(), "POST", o.Metadata.Endpoints["http"], bytes.NewBuffer(jsonBody))
// 	if err != nil {
// 		return map[string]any{"error": err.Error(), "output": nil}
// 	}
// 	req.Header.Set("Content-Type", "application/json")
// 	req.Header.Set("X-goog-api-key", viper.GetString("ORCHESTRATOR.API_KEY"))

// 	resp, err := http.DefaultClient.Do(req)
// 	if err != nil {
// 		return map[string]any{"error": err.Error()}
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		bodyBytes, _ := io.ReadAll(resp.Body)
// 		var response map[string]any

// 		if err = json.Unmarshal(bodyBytes, &response); err != nil {
// 			return map[string]any{"error": fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)), "output": nil}
// 		}

// 		return map[string]any{
// 			"error": map[string]any{
// 				"status":      resp.Status,
// 				"status_code": resp.StatusCode,
// 				"body":        response,
// 			},
// 			"output": nil,
// 		}
// 	}

// 	// Parse the response
// 	bodyBytes, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return map[string]any{"error": err.Error(), "output": nil}
// 	}

// 	var response repository.Response
// 	if err = json.Unmarshal(bodyBytes, &response); err != nil {
// 		return map[string]any{"error": err.Error(), "output": nil}
// 	}

// 	newResponse := make([]map[string]any, 0)
// 	for _, candidate := range response.Candidates {
// 		for _, part := range candidate.Content.Parts {
// 			if part.Text != "" {
// 				newResponse = append(newResponse, map[string]any{"text": part.Text})
// 			} else if part.FunctionCall.Name != "" {
// 				newResponse = append(newResponse, map[string]any{
// 					"text":          "",
// 					"function_name": part.FunctionCall.Name,
// 					"parameters":    part.FunctionCall.Args,
// 				})
// 			}
// 		}
// 	}

// 	return map[string]any{"error": nil, "output": newResponse}
// }
