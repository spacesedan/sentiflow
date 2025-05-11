package clients

import (
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	openAIRequestTimeout = 60 * time.Second // Timeout for individual OpenAI API requests
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
		config := openai.DefaultConfig(apiKey)
		httpClient := &http.Client{
			Timeout: openAIRequestTimeout,
		}
		config.HTTPClient = httpClient

		openAIClientInstance = &OpenAIClient{
			Client: openai.NewClientWithConfig(config),
		}
		slog.Info("[OpenAIClient] OpenAI client initialized with custom HTTP timeout", slog.Duration("timeout", openAIRequestTimeout))
	})
	return openAIClientInstance
}
