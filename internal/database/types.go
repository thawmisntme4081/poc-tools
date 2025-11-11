package database

import (
	"github.com/mark3labs/mcp-go/mcp"
	openai "github.com/sashabaranov/go-openai"
)

type StopReason string

const (
	StopReasonMaxTokens  StopReason = "max_tokens"
	StopReasonUserInput  StopReason = "user_input"
	StopReasonToolCall   StopReason = "tool_call"
	StopReasonToolResult StopReason = "tool_result"
	StopReasonAgentDone  StopReason = "agent_done"
	StopReasonUnknown    StopReason = "unknown"
	StopReasonNil        StopReason = ""
)

type Node struct {
	ID        string      `json:"id"`
	Type      NodeType    `json:"type"` // start, agent
	AgentName *string     `json:"agentName,omitempty"`
	Next      *string     `json:"next,omitempty"`
	Output    *NodeOutput `json:"output,omitempty"`
}

type NodeType string
type NodeOutputType string
type NodeContentRole string
type ModelProvider string

const (
	NodeTypeStart            NodeType        = "start"
	NodeTypeAgent            NodeType        = "agent"
	NodeOutputTypeText       NodeOutputType  = "text"
	NodeOutputTypeStructured NodeOutputType  = "structured"
	NodeContentRoleUser      NodeContentRole = "user"
	NodeContentRoleSystem    NodeContentRole = "system"
	ModelProviderAnthropic   ModelProvider   = "anthropic"
	ModelProviderOpenAI      ModelProvider   = "openai"
)

type NodeOutput struct {
	Type          NodeOutputType  `json:"type"` // text or structured (JSON)
	ContentFormat string          `json:"contentFormat"`
	ContentRole   NodeContentRole `json:"contentRole"` // user, system, assistant
}

type AgentConfig struct {
	Description   string        `json:"description"`
	SystemPrompt  string        `json:"systemPrompt"`
	Provider      ModelProvider `json:"provider"` // anthropic or openai
	ModelID       string        `json:"modelId"`
	MaxTokens     int64         `json:"maxTokens"`
	Temperature   float64       `json:"temperature"`
	TopP          float64       `json:"topP"`
	TopK          int64         `json:"topK"`
	ThinkingToken int64         `json:"thinkingToken"`
	Tools         []mcp.Tool    `json:"tools"`
	McpServers    []MCPConfig   `json:"mcpServers"` // MCP servers to use
}

type MCPConfig struct {
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"` // stdio, streamablehttp
	Command        *string           `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Envs           map[string]string `json:"envs,omitempty"`
	URL            *string           `json:"url"` // Remote MCP server URL
	Authentication *string           `json:"key"` // API key or token for authentication
}

type AgentFlowConfig struct {
	Agents map[string]AgentConfig `json:"agents"`
	Nodes  []Node                 `json:"nodes"`
}

type MessageUnion struct {
	OfOpenAI *openai.ChatCompletionMessage `json:"of_openai,omitempty"`
	// OfAnthropic *anthropic.MessageParam `json:"of_anthropic,omitempty"`
}
