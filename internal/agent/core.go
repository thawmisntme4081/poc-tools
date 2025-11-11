package agent

import (
	"context"
	"fmt"
	"stockmind/internal/database"

	"github.com/joho/godotenv"
	mcp_client "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// Model ID in OpenRouter support function calling and free (https://openrouter.ai/models)
const (
	NEMOTRON_NANO_9B_V2 = "nvidia/nemotron-nano-9b-v2:free"
	GLM_4_5_AIR         = "z-ai/glm-4.5-air:free"
	QWEN3_CODE          = "qwen/qwen3-coder:free"
	QWEN3_4B            = "qwen/qwen3-4b:free"
	QWEN3_235B          = "qwen/qwen3-235b-a22b:free"
	KIMI_K2             = "moonshotai/kimi-k2:free"
	MISTRAL_SMALL       = "mistralai/mistral-small-3.2-24b-instruct:free"
	DEVTRAL_SMALL       = "mistralai/devstral-small-2505:free"
	DEEPSEEK_V3         = "deepseek/deepseek-chat-v3-0324:free"
)

func init() {
	_ = godotenv.Load()
}

// type Agent struct {
// 	SystemPrompt string
// 	Description  string
// 	Provider     ModelProvider
// 	ModelId      string
// 	MaxTokens    int
// 	Temperature  float32
// 	Tools        []ToolWrapper
// 	Stream       bool
// }

type Agent struct {
	name       string
	session    database.Session
	config     database.AgentConfig
	provider   *LLMClientWrapper
	tools      []mcp.Tool
	mcpClients map[string]*mcp_client.Client // Cache of MCP clients by mcp config
}

func NewAgent(ctx context.Context, session database.Session, name string, config database.AgentConfig, provider *LLMClientWrapper) (*Agent, error) {
	a := &Agent{
		name:       name,
		session:    session,
		config:     config,
		provider:   provider,
		tools:      []mcp.Tool{},
		mcpClients: make(map[string]*mcp_client.Client),
	}

	// Initialize all MCP and put them to mcpClient map
	a.mcpClients = make(map[string]*mcp_client.Client, len(config.McpServers))
	for _, mcpCfg := range config.McpServers {
		if _, exists := a.mcpClients[mcpCfg.Name]; exists {
			continue
		}
		mcpClient, err := createMCPClient(ctx, mcpCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create MCP client for %s: %w", mcpCfg.Name, err)
		}
		a.mcpClients[mcpCfg.Name] = mcpClient
	}

	fmt.Println("MCP clients initialized", "count", len(a.mcpClients))
	// Fetch all tools from MCP and put them to tools array
	for mcpName, mcpClient := range a.mcpClients {
		// Assume very litle tools, so we can fetch all at once
		// If there are too many tools, we need to implement pagination
		// But for now, we assume there are very litle tools
		res, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
		if err != nil {
			return nil, fmt.Errorf("failed to list tools from MCP: %w", err)
		}
		fmt.Println("Fetched tools from MCP", "mcp_name", mcpName, "tool_count", len(res.Tools))
		tools := res.Tools
		// Rename tool names to include MCP name as prefix to avoid name collision
		for i := range tools {
			tools[i].Name = fmt.Sprintf("%s--%s", mcpName, tools[i].Name)
		}
		a.tools = append(a.tools, tools...)
	}
	fmt.Println("MCP tools initialized", "count", len(a.tools))
	return a, nil
}

func createMCPClient(ctx context.Context, cfg database.MCPConfig) (*mcp_client.Client, error) {
	// Create transport first
	var mcpTransport transport.Interface
	var err error
	switch cfg.Protocol {
	case "stdio":
		// Create stdio transport
		// Transform envs from map to array
		envs := []string{}
		for k, v := range cfg.Envs {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}
		mcpTransport = transport.NewStdio(*cfg.Command, envs, cfg.Args...)
	case "streamablehttp":
		// Create streamablehttp transport
		if cfg.URL == nil {
			return nil, fmt.Errorf("URL is required for streamablehttp protocol")
		}
		options := []transport.StreamableHTTPCOption{}
		if cfg.Authentication != nil {
			options = append(options, transport.WithHTTPHeaders(map[string]string{
				"Authorization": *cfg.Authentication,
			}))
		}
		mcpTransport, err = transport.NewStreamableHTTP(*cfg.URL, options...)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown protocol: %s", cfg.Protocol)
	}
	err = mcpTransport.Start(ctx)
	if err != nil {
		return nil, err
	}
	// Then create mcp client
	mcpClient := mcp_client.NewClient(mcpTransport)
	// Initialize the client (fetch tools, etc)
	_, err = mcpClient.Initialize(ctx, mcp.InitializeRequest{})
	if err != nil {
		return nil, err
	}
	return mcpClient, nil
}

func (a *Agent) Completion(ctx context.Context, messages []*database.MessageUnion, callback ChatCallBack) (database.MessageUnion, database.StopReason, error) {
	fmt.Println("Agent Completion called", "sessionId", a.session.ID, "agent_name", a.name, "model_provider", a.config.Provider)
	switch a.config.Provider {
	case database.ModelProviderOpenAI:
		return a.completionOpenAI(ctx, messages, callback)
	// case database.ModelProviderAnthropic:
	// 	return a.completionAnthropic(ctx, messages, callback)
	default:
		return database.MessageUnion{}, database.StopReasonUnknown, fmt.Errorf("unsupported model provider: %s", a.config.Provider)
	}
}

func (a *Agent) ToolUse(ctx context.Context, message *database.MessageUnion) (database.MessageUnion, error) {
	switch a.config.Provider {
	case database.ModelProviderOpenAI:
		return a.toolUseOpenAI(ctx, message)
	// case database.ModelProviderAnthropic:
	// 	return a.toolUseAnthropic(ctx, message)
	default:
		return database.MessageUnion{}, fmt.Errorf("unsupported model provider: %s", a.config.Provider)
	}
}
