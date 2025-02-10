package clients

import (
	"log/slog"
	"os"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

var (
	openAIClientInstance *OpenAIClient
	openAIOnce           sync.Once
)

type OpenAIClient struct {
	Client *openai.Client
}

func GetAIClient() *OpenAIClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		slog.Error("[OpenAICLient] Missing OPENAI_API_KEY in environment variables")
		panic("[OpenAICLient] Missing OPENAI_API_KEY in environment variables")
	}
	openAIOnce.Do(func() {
		openAIClientInstance = &OpenAIClient{
			Client: openai.NewClient(option.WithAPIKey(apiKey)),
		}
	})
	return openAIClientInstance
}
