package agent

import (
	"context"
	"fmt"

	"stockmind/internal/database"

	"github.com/google/uuid"
	openai "github.com/sashabaranov/go-openai"
)

type ChatCallBack func(textContent string, thinking bool, endBlock bool) error

type SessionManager struct {
	ctx          context.Context
	cancel       context.CancelFunc
	session      database.Session
	agentFlowCfg database.AgentFlowConfig
	llm          *AgentService
	history      []database.SessionHistory
	nodes        map[string]database.Node // For quick lookup
	agents       map[string]*Agent        // For quick lookup
	chatCallback ChatCallBack
}

func (sm *SessionManager) Initialize() error {
	// Fetch existing messages from DB
	if len(sm.history) == 0 {
		msgs, err := sm.llm.queries.GetSessionHistoryBySessionID(sm.ctx, sm.session.ID)
		if err != nil {
			return err
		}
		sm.history = msgs
	}
	// Build node map for quick lookup
	sm.nodes = make(map[string]database.Node, len(sm.agentFlowCfg.Nodes))
	for _, node := range sm.agentFlowCfg.Nodes {
		sm.nodes[node.ID] = node
	}
	// Initialize all agents
	sm.agents = make(map[string]*Agent, len(sm.agentFlowCfg.Agents))
	for name, agentCfg := range sm.agentFlowCfg.Agents {
		// Get providers
		provider, err := sm.llm.getClientByProvider(agentCfg.Provider)
		if err != nil {
			return fmt.Errorf("failed to get LLM client for provider %s: %w", agentCfg.Provider, err)
		}
		agent, err := NewAgent(sm.ctx, sm.session, name, agentCfg, provider)
		if err != nil {
			return fmt.Errorf("failed to initialize agent %s: %w", name, err)
		}
		sm.agents[name] = agent
	}
	return nil
}

func (sm *SessionManager) AddChatCallback(cb ChatCallBack) {
	sm.chatCallback = cb
}

func (sm *SessionManager) lastHistoryInfo() (database.Node, database.SessionHistory, error) {
	if len(sm.history) == 0 {
		return database.Node{}, database.SessionHistory{}, fmt.Errorf("no history found")
	}
	lastHistory := sm.history[len(sm.history)-1]
	lastNodeID := lastHistory.Node
	lastNode, exists := sm.nodes[lastNodeID]
	if !exists {
		return database.Node{}, database.SessionHistory{}, fmt.Errorf("last node %s not found in agent flow config", lastNodeID)
	}
	return lastNode, lastHistory, nil
}

func (sm *SessionManager) IsHumanTurn() bool {
	// Check if we are correctly at the start of the flows (start node)
	// Either history is empty, or last node of the conversation is an end node
	startOfFlow := len(sm.history) == 0
	if !startOfFlow {
		// Check last node
		lastNode, lastHistory, err := sm.lastHistoryInfo()
		if err != nil {
			return false
		}
		// Not tool call, so it must be start of flow
		// If last node is agent, and stop reason is not tool_call, then we are at start of flow
		if lastNode.Type == database.NodeTypeAgent && lastHistory.StopReason != database.StopReasonToolCall && lastHistory.StopReason != database.StopReasonToolResult {
			startOfFlow = true
		}
		// If last node is start, we should not be here
		if lastNode.Type == database.NodeTypeStart {
			return false
		}
	}
	return startOfFlow
}

func (sm *SessionManager) HumanInput(message string) error {
	// Check if we are correctly at the start of the flows (start node)
	// Either history is empty, or last node of the conversation is an end node
	startOfFlow := len(sm.history) == 0
	if !startOfFlow {
		// Check last node
		lastNode, lastHistory, err := sm.lastHistoryInfo()
		if err != nil {
			return err
		}
		// Not tool call, so it must be start of flow
		// If last node is agent, and stop reason is not tool_call, then we are at start of flow
		// If last node is start, we should not be here
		if lastNode.Type == database.NodeTypeAgent && lastHistory.StopReason != "tool_call" {
			startOfFlow = true
		}
		// If last node is start, we should not be here
		if lastNode.Type == database.NodeTypeStart {
			return fmt.Errorf("last node %s is start node, but we are not at start of flow", lastNode.ID)
		}
	}
	if !startOfFlow {
		return fmt.Errorf("not at start of flow, cannot accept human input")
	}

	// Add human input to history
	// Check next agent node for provider
	startNode := sm.nodes["start"]
	if startNode.Next == nil {
		return fmt.Errorf("start node has no next node")
	}
	nextNodeID := *startNode.Next
	nextNode, exists := sm.nodes[nextNodeID]
	if !exists {
		return fmt.Errorf("next node %s not found in agent flow config", nextNodeID)
	}
	provider := sm.agentFlowCfg.Agents[*nextNode.AgentName].Provider
	humanMsg, err := newHumanMessage(message, provider)
	if err != nil {
		return err
	}

	// Create new history entry and store it to DB
	historyID := uuid.Must(uuid.NewV7())
	history, err := sm.llm.queries.SessionAddChatHistory(sm.ctx, database.SessionAddChatHistoryParams{
		ID:         historyID,
		SessionID:  sm.session.ID,
		Content:    humanMsg,
		StopReason: database.StopReasonUserInput,
		Node:       "start",
	})
	if err != nil {
		return err
	}
	sm.history = append(sm.history, history)
	return nil
}

func (sm *SessionManager) ContinueTurn() error {
	// Check last message
	var err error
	lastNode, lastHistory, err := sm.lastHistoryInfo()
	if err != nil {
		return err
	}
	switch lastHistory.StopReason {
	case database.StopReasonUserInput:
		// Continue with next agent node
		// Suppose to be start node
		err = sm.continueTurnHumanInput()
	case database.StopReasonToolCall:
		// Still agent node, call tools, append tool result to history
		err = sm.continueTurnToolCall()
	case database.StopReasonToolResult:
		// Still agent node, call the agent again
		err = sm.continueTurnToolResult()
	case database.StopReasonAgentDone:
		// Call next node. If no next node then end the turn
		if lastNode.Next == nil {
			fmt.Printf("No next node, turn is complete. To continue, add new human input %s\n", sm.session.ID.String())
			return nil
		}
		nextNode := sm.nodes[*lastNode.Next]
		if nextNode.Type != database.NodeTypeAgent {
			return fmt.Errorf("next node %s is not an agent node, cannot continue turn", nextNode.ID)
		}
		// Continue with next agent node
		// Processing message for next agent here
		// Push to next agent node
		// TODO: Implement next agent for multi agents here
	default:
		return fmt.Errorf("cannot continue turn, last history stop reason is %s", lastHistory.StopReason)
	}
	if err != nil {
		fmt.Printf("Failed to continue turn: %v\n", err)
		return err
	}
	sm.session.TurnCount++
	// Update session turn count in DB
	err = sm.llm.queries.UpdateSessionTurnCount(sm.ctx, database.UpdateSessionTurnCountParams{
		ID:        sm.session.ID,
		TurnCount: sm.session.TurnCount,
	})
	if err != nil {
		return fmt.Errorf("failed to update session turn count: %w", err)
	}
	return nil
}

func newHumanMessage(message string, provider database.ModelProvider) (database.MessageUnion, error) {
	switch provider {
	case database.ModelProviderOpenAI:
		return database.MessageUnion{
			OfOpenAI: &openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: message,
			},
		}, nil
	// case database.ModelProviderAnthropic:
	// 	return database.MessageUnion{
	// 		OfAnthropic: &anthropic.MessageParam{
	// 			Role: anthropic.MessageParamRoleUser,
	// 			Content: []anthropic.ContentBlockParamUnion{
	// 				{OfText: &anthropic.TextBlockParam{Text: message}},
	// 			},
	// 		},
	// 	}, nil
	default:
		return database.MessageUnion{}, fmt.Errorf("unsupported model provider: %s", provider)
	}
}

func (sm *SessionManager) continueTurnHumanInput() error {
	lastNode, _, err := sm.lastHistoryInfo()
	if err != nil {
		return err
	}
	if lastNode.Next == nil {
		return fmt.Errorf("last node %s has no next node, cannot continue turn", lastNode.ID)
	}
	nextNode := sm.nodes[*lastNode.Next]
	if nextNode.Type != database.NodeTypeAgent {
		return fmt.Errorf("next node %s is not an agent node, cannot continue turn", nextNode.ID)
	}
	agent := sm.agents[*nextNode.AgentName]
	if agent == nil {
		return fmt.Errorf("agent %s not found in session manager", *nextNode.AgentName)
	}
	// Not last history, all history
	messages := make([]*database.MessageUnion, 0, len(sm.history))
	for i := range sm.history {
		messages = append(messages, &sm.history[i].Content)
	}
	// Call the agent to complete the turn
	result, stopReason, err := agent.Completion(sm.ctx, messages, sm.chatCallback)
	if err != nil {
		return fmt.Errorf("failed to complete turn with agent %s: %w", *nextNode.AgentName, err)
	}
	// Store the result to history
	historyID := uuid.Must(uuid.NewV7())
	history, err := sm.llm.queries.SessionAddChatHistory(sm.ctx, database.SessionAddChatHistoryParams{
		ID:         historyID,
		SessionID:  sm.session.ID,
		Content:    result,
		StopReason: stopReason,
		Node:       nextNode.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to add chat history: %w", err)
	}
	sm.history = append(sm.history, history)
	return nil
}

// Continue turn with tool call
func (sm *SessionManager) continueTurnToolCall() error {
	lastNode, lastHistory, err := sm.lastHistoryInfo()
	if err != nil {
		return err
	}
	if lastNode.Type != database.NodeTypeAgent {
		return fmt.Errorf("last node %s is not an agent node, cannot continue turn", lastNode.ID)
	}
	agent := sm.agents[*lastNode.AgentName]
	if agent == nil {
		return fmt.Errorf("agent %s not found in session manager", *lastNode.AgentName)
	}
	message, err := agent.ToolUse(sm.ctx, &lastHistory.Content)
	if err != nil {
		return fmt.Errorf("failed to call tool use on agent %s: %w", *lastNode.AgentName, err)
	}
	// Store the result to history
	historyID := uuid.Must(uuid.NewV7())
	history, err := sm.llm.queries.SessionAddChatHistory(sm.ctx, database.SessionAddChatHistoryParams{
		ID:         historyID,
		SessionID:  sm.session.ID,
		Content:    message,
		StopReason: database.StopReasonToolResult,
		Node:       lastNode.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to add chat history: %w", err)
	}
	sm.history = append(sm.history, history)
	return nil
}

// Continue turn with tool results
func (sm *SessionManager) continueTurnToolResult() error {
	lastNode, _, err := sm.lastHistoryInfo()
	if err != nil {
		return err
	}
	if lastNode.Type != database.NodeTypeAgent {
		return fmt.Errorf("last node %s is not an agent node, cannot continue turn", lastNode.ID)
	}
	agent := sm.agents[*lastNode.AgentName]
	if agent == nil {
		return fmt.Errorf("agent %s not found in session manager", *lastNode.AgentName)
	}
	messages := make([]*database.MessageUnion, 0, len(sm.history))
	for i := range sm.history {
		messages = append(messages, &sm.history[i].Content)
	}
	// Call the agent to complete the turn
	result, stopReason, err := agent.Completion(sm.ctx, messages, sm.chatCallback)
	if err != nil {
		return fmt.Errorf("failed to complete turn with agent %s: %w", *lastNode.AgentName, err)
	}
	// Store the result to history
	historyID := uuid.Must(uuid.NewV7())
	history, err := sm.llm.queries.SessionAddChatHistory(sm.ctx, database.SessionAddChatHistoryParams{
		ID:         historyID,
		SessionID:  sm.session.ID,
		Content:    result,
		StopReason: stopReason,
		Node:       lastNode.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to add chat history: %w", err)
	}
	sm.history = append(sm.history, history)
	return nil
}
