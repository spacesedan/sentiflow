package clients

import (
	"log/slog"
	"os"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

var (
	openAIClientInstance *OpenAIClient
	openAIOnce           sync.Once
)

type OpenAIClient struct {
	Client *openai.Client
}

func GetOpenAIClient() *OpenAIClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		slog.Error("[OpenAICLient] Missing OPENAI_API_KEY in environment variables")
		panic("[OpenAICLient] Missing OPENAI_API_KEY in environment variables")
	}
	openAIOnce.Do(func() {
		openAIClientInstance = &OpenAIClient{
			Client: openai.NewClient(apiKey),
		}
	})
	return openAIClientInstance
}
