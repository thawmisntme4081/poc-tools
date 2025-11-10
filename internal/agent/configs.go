package agent

import (
	"os"
)

type Config struct {
	Database Database          `json:"database" yaml:"database"`
	LLM      LLMProviderConfig `json:"llmProvider" yaml:"llmProvider"`
}

type Database struct {
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password" yaml:"password"`
	Dbname   string `json:"dbname" yaml:"dbname"`
}

type LLMProviderConfig struct {
	OpenAI    OpenAIConfig    `json:"openai" yaml:"openai"`
	Anthropic AnthropicConfig `json:"anthropic" yaml:"anthropic"`
}

type OpenAIConfig struct {
	AuthType string `json:"authType" yaml:"authType"` // "openai" or "open_router"
	APIKey   string `json:"api_key" yaml:"api_key"`
	BaseURL  string `json:"baseURL" yaml:"baseURL"`
}

type AnthropicConfig struct {
	AuthType string              `json:"authType" yaml:"authType"` // "api_key" or "aws"
	APIKey   string              `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	AWS      AWSCredentialConfig `json:"aws,omitempty" yaml:"aws,omitempty"`
}

type AWSCredentialConfig struct {
	Type            string `json:"type" yaml:"type"` // "default" or "assume_role"
	Region          string `json:"region" yaml:"region"`
	RoleARN         string `json:"roleArn,omitempty" yaml:"roleArn,omitempty"`                 // required if Type is "assume_role"
	Duration        int64  `json:"duration,omitempty" yaml:"duration,omitempty"`               // in seconds, optional
	RoleSessionName string `json:"roleSessionName,omitempty" yaml:"roleSessionName,omitempty"` // optional
}

var OpenAIProvider = OpenAIConfig{
	AuthType: "open_router",
	APIKey:   os.Getenv("OPENROUTER_API_KEY"),
	BaseURL:  "https://openrouter.ai/api/v1",
}

var AnthropicProvider = AnthropicConfig{
	AuthType: "api_key",
	APIKey:   os.Getenv("ANTHROPIC_API_KEY"),
}

// func LoadConfig(filePath string) (*Config, error) {
// 	config := &Config{}
// 	data, err := os.ReadFile(filePath)
// 	if err != nil {
// 		return nil, err
// 	}
// 	expandedData, err := shell.Expand(string(data), func(env string) string {
// 		return os.Getenv(env)
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	data = []byte(expandedData) // Replace environment variables in the config file

// 	if err := yaml.UnmarshalStrict(data, config); err != nil {
// 		return nil, err
// 	}

// 	// Here you can add logic to replace environment variables in the config if needed
// 	// For example, using os.Getenv() to replace placeholders in the config struct

// 	return config, nil
// }
