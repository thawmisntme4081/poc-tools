package agent

import (
	"context"
	"fmt"
	"log"
	"stockmind/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"
)

type LLMClientWrapper struct {
	OfOpenAI *openai.Client
}

type AgentService struct {
	config  LLMProviderConfig
	queries *database.Queries
	ctx     context.Context
}

func NewService(ctx context.Context, dbPool *pgxpool.Pool, providers database.ModelProvider) (*AgentService, error) {
	log.Println("Initializing LLM service...")
	var config LLMProviderConfig
	if providers == database.ModelProviderOpenAI {
		config = LLMProviderConfig{OpenAI: OpenAIProvider}
	}
	if providers == database.ModelProviderAnthropic {
		config = LLMProviderConfig{Anthropic: AnthropicProvider}
	}

	return &AgentService{
		config:  config,
		ctx:     ctx,
		queries: database.New(dbPool),
	}, nil
}

func (s *AgentService) getClientByProvider(provider database.ModelProvider) (*LLMClientWrapper, error) {
	var client *LLMClientWrapper
	var err error
	switch provider {
	case database.ModelProviderOpenAI:
		client, err = createOpenAIClient(s.config.OpenAI)
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenRouter client: %v", err)
		}
	case database.ModelProviderAnthropic:
		return nil, fmt.Errorf("unsupported model provider: %s", string(provider))
	default:
		log.Printf("Unsupported model provider: %s", string(provider))
		return nil, fmt.Errorf("unsupported model provider: %s", string(provider))
	}
	return client, nil
}

func (s *AgentService) GetOrCreateSession(userID, agentFlowID, sessionID *uuid.UUID, sessionName *string) (*SessionManager, error) {
	// Create a new session in the database
	var session database.Session
	var agentFlow database.AgentFlow
	var err error
	if sessionID == nil {
		newUUID := uuid.Must(uuid.NewV7())
		sessionID = &newUUID
		if userID == nil || agentFlowID == nil {
			return nil, fmt.Errorf("userID and agentFlowID are required to create a new session")
		}
		// Get Agent Flow Configuration
		agentFlow, err = s.queries.GetAgentFlowById(s.ctx, *agentFlowID)
		if err != nil {
			fmt.Errorf("Failed to get agent flow by ID", "error", err, "agent_flow_id", *agentFlowID)
			return nil, err
		}
		fmt.Errorf("Creating new session", "user_id", *userID, "agent_flow_id", *agentFlowID, "agent_flow_name", agentFlow.Name)
		newSessionName := "New Session"
		if sessionName != nil && *sessionName != "" {
			newSessionName = *sessionName
		}
		session, err = s.queries.CreateSession(s.ctx, database.CreateSessionParams{
			ID:          *sessionID,
			CreatedBy:   *userID,
			AgentFlowID: *agentFlowID,
			Title:       newSessionName,
		})
		if err != nil {
			fmt.Errorf("Failed to create new session", "error", err)
			return nil, err
		}
		fmt.Errorf("New session created", "session_id", *sessionID, "user_id", *userID)
	} else {
		// Fetch existing session from the database
		session, err = s.queries.GetSessionByID(s.ctx, *sessionID)
		if err != nil {
			fmt.Errorf("Failed to get session by ID", "error", err, "session_id", *sessionID)
			return nil, err
		}
		// Get Agent Flow Configuration
		agentFlow, err = s.queries.GetAgentFlowById(s.ctx, session.AgentFlowID)
		if err != nil {
			fmt.Errorf("Failed to get agent flow by ID", "error", err, "agent_flow_id", session.AgentFlowID)
			return nil, err
		}
		fmt.Errorf("Existing session fetched", "session_id", *sessionID, "user_id", session.CreatedBy, "agent_flow_id", session.AgentFlowID, "agent_flow_name", agentFlow.Name)
	}

	ctx, cancel := context.WithCancel(s.ctx)
	sm := &SessionManager{
		ctx:          ctx,
		cancel:       cancel,
		session:      session,
		agentFlowCfg: agentFlow.Config,
		llm:          s,
		history:      []database.SessionHistory{},
		nodes:        make(map[string]database.Node),
	}
	err = sm.Initialize()
	return sm, err
}
