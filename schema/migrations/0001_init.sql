-- Database initialization script for PostgreSQL Docker Compose
-- This file will be executed automatically when the PostgreSQL container starts
-- Set client encoding and timezone
-- +goose Up
-- =============================================
-- SCHEMA CREATION AND UTILITY FUNCTIONS
-- =============================================
-- Function to automatically update the updated_at timestamp
-- +goose statementbegin
CREATE OR REPLACE FUNCTION trigger_set_timestamp () RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW(); -- Sets updated_at to the current transaction timestamp
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose statementend

-- =============================================
-- TABLE CREATION
-- =============================================

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    provider VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Agent flow table
CREATE TABLE IF NOT EXISTS agent_flows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    config JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Session table
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    turn_count INT4 NOT NULL DEFAULT 0,
    agent_flow_id UUID NOT NULL REFERENCES agent_flows(id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Messages table
CREATE TABLE IF NOT EXISTS session_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    node TEXT NOT NULL,
    content JSONB NOT NULL,
    stop_reason TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Default Agent Flow
INSERT INTO agent_flows (id, name, config) VALUES
('01993ca8-a62e-79e3-995c-a46e25a4a2a2', 'Default Flow', 
'{
    "agents": {
        "NormalChat": {
            "provider": "openai",
            "description": "Handles general chat interactions.",
            "modelId": "NEMOTRON_NANO_9B_V2",
            "systemPrompt": "You are a helpful assistant.",
            "maxTokens": 8192,
            "thinkingToken": 1024,
            "temperature": 0.7,
            "topP": 0.9,
            "topK": 40,
            "tools": [],
            "mcpServers": [
                {
                    "name": "stocks-mcp",
                    "protocol": "stdio",
                    "command": "go",
                    "args": ["run", "cmd/main.go", "mcp"]
                }
            ]
        }
    },
    "nodes": [
        {
            "id": "start",
            "type": "start",
            "next": "NormalChat"
        },
        {
            "id": "NormalChat",
            "type": "agent",
            "agentName": "NormalChat",
            "next": "end",
            "output": {
                "type": "text",
                "contentFormat": "{{.message}}",
                "contentRole": "assistant"
            }
        }
    ]
}');