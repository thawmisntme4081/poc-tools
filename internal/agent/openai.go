package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"stockmind/internal/database"

	"github.com/mark3labs/mcp-go/mcp"
	openai "github.com/sashabaranov/go-openai"
)

func createOpenAIClient(config OpenAIConfig) (*LLMClientWrapper, error) {
	var openaiClient *openai.Client

	if config.AuthType == "openai" {
		openaiClient = openai.NewClient(config.APIKey)
	}
	if config.AuthType == "open_router" {
		var defaultConfig openai.ClientConfig
		key := config.APIKey
		if key == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is not found")
		}
		defaultConfig = openai.DefaultConfig(key)
		defaultConfig.BaseURL = config.BaseURL

		openaiClient = openai.NewClientWithConfig(defaultConfig)
	}
	return &LLMClientWrapper{OfOpenAI: openaiClient}, nil
}

func (a *Agent) newOpenAIMessage() openai.ChatCompletionRequest {
	request := openai.ChatCompletionRequest{
		Model:       a.config.ModelID,
		MaxTokens:   int(a.config.MaxTokens),
		Temperature: float32(a.config.Temperature),
		Stream:      true,
	}

	var tools []openai.Tool
	for _, tool := range a.config.Tools {
		openAITool := openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
		tools = append(tools, openAITool)
	}
	request.Tools = tools
	request.Messages = []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: a.config.SystemPrompt},
	}
	return request
}

func openaiToDbStopReason(reason openai.FinishReason) database.StopReason {
	switch reason {
	case openai.FinishReasonLength: // Max tokens
		return database.StopReasonMaxTokens
	case openai.FinishReasonToolCalls: // Tool call
		return database.StopReasonToolCall
	case openai.FinishReasonStop: // End Turn
		return database.StopReasonAgentDone
	default:
		return database.StopReasonUnknown
	}
}

func (a *Agent) completionOpenAI(ctx context.Context, messages []*database.MessageUnion, callback ChatCallBack) (database.MessageUnion, database.StopReason, error) {
	// Prepare messages for OpenAI
	body := a.newOpenAIMessage()
	for _, m := range messages {
		if am := m.OfOpenAI; am != nil {
			body.Messages = append(body.Messages, *am)
		}
	}
	result := database.MessageUnion{
		OfOpenAI: &openai.ChatCompletionMessage{
			Role:         openai.ChatMessageRoleAssistant,
			MultiContent: []openai.ChatMessagePart{},
			ToolCalls:    []openai.ToolCall{},
		},
	}
	var stopReason database.StopReason
	// Call OpenAI API
	if a.provider == nil || a.provider.OfOpenAI == nil {
		return result, stopReason, fmt.Errorf("openAI client is not initialized")
	}
	stream, err := a.provider.OfOpenAI.CreateChatCompletionStream(ctx, body)
	if err != nil {
		return result, stopReason, err
	}

	defer stream.Close()

	contentItem := openai.ChatCompletionMessage{}

	// Handle the stream
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			fmt.Println("\nStream finished")
			return result, stopReason, err
		}

		if err != nil {
			fmt.Printf("\nStream error: %v\n", err)
			return result, stopReason, err
		}

		for _, chunk := range response.Choices {
			// Handle the chunk
			delta := chunk.Delta
			switch {
			case delta.Content != "":
				contentItem.MultiContent = append(contentItem.MultiContent, openai.ChatMessagePart{Type: openai.ChatMessagePartTypeText, Text: delta.Content})
			case len(delta.ToolCalls) > 0:
				contentItem.ToolCalls = delta.ToolCalls
			default:
				continue
			}
		}
		return result, stopReason, nil
	}
}

func (a *Agent) toolUseOpenAI(ctx context.Context, message *database.MessageUnion) (database.MessageUnion, error) {
	lastMessage := message.OfOpenAI
	result := database.MessageUnion{}
	if lastMessage == nil {
		return result, fmt.Errorf("last message is not an Anthropic message")
	}
	// Find the tool use block
	toolUseBlocks := []openai.ToolCall{}

	for _, block := range lastMessage.ToolCalls {
		toolUseBlocks = append(toolUseBlocks, block)
	}
	if len(toolUseBlocks) == 0 {
		fmt.Println("No tool use blocks found in chat history", "sessionId", a.session.ID, "agentName", a.name)
		return result, fmt.Errorf("no tool use blocks found in chat history")
	}
	toolUseMessage := openai.ChatCompletionMessage{
		Role:         openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{},
	}
	for _, toolUse := range toolUseBlocks {
		fmt.Println("Invoking tool", "name", toolUse.Function.Name, "input", toolUse.Function.Arguments)
		// Normally toolUse.Name will have format <mcp>/<tool_name>
		parts := strings.SplitN(toolUse.Function.Name, "--", 2)
		if len(parts) != 2 {
			fmt.Println("Invalid tool name format, expected <mcp>--<tool_name>", "sessionId", a.session.ID, "agentName", a.name, "tool_name", toolUse.Function.Name)
			return result, fmt.Errorf("invalid tool name format, expected <mcp>--<tool_name>")
		}
		mcpName := parts[0]
		toolName := parts[1]
		mcpClient, ok := a.mcpClients[mcpName]
		if !ok {
			fmt.Println("MCP client not found", "sessionId", a.session.ID, "agentName", a.name, "mcpName", mcpName)
			return result, fmt.Errorf("MCP client not found: %s", mcpName)
		}
		// Serialize the input JSON into map[string] any
		toolResponse, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      toolName,
				Arguments: toolUse.Function.Arguments,
				Meta: &mcp.Meta{
					AdditionalFields: map[string]any{
						"user_id":    a.session.CreatedBy,
						"session_id": a.session.ID,
					},
				},
			},
			// Header: http.Header{
			// 	"X-Session-ID": []string{a.session.ID.String()},
			// 	"X-User-ID":    []string{a.session.UserID.String()},
			// },
		})
		if err != nil {
			fmt.Println("Failed to call tool", "sessionId", a.session.ID, "agentName", a.name, "toolName", toolUse.Function.Name, "error", err)
			return result, fmt.Errorf("failed to call tool %s: %w", toolUse.Function.Name, err)
		}

		toolResult := openai.ChatCompletionMessage{
			Role:         openai.ChatMessageRoleTool,
			ToolCallID:   toolUse.ID,
			MultiContent: []openai.ChatMessagePart{},
			Name:         toolUse.Function.Name,
		}
		// Convert the tool response content to anthropic format
		for _, content := range toolResponse.Content {
			switch content := content.(type) {
			case mcp.TextContent:
				toolResult.MultiContent = append(
					toolResult.MultiContent,
					openai.ChatMessagePart{Type: openai.ChatMessagePartTypeText, Text: content.Text},
				)
				fmt.Println("Tool result: ", "sessionId", a.session.ID, "agentName", a.name, "tool_id", toolUse.ID, "tool_name", toolUse.Function.Name, "text", content.Text)
			}
		}
		toolUseMessage.MultiContent = append(toolUseMessage.MultiContent, toolResult.MultiContent...)
	}
	result.OfOpenAI = &toolUseMessage
	return result, nil
}
