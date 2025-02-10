package processing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/models"
)

func GenerateTopicsFromHeadlines(headlinesResponse *models.NewsAPITopHeadlinesResponse) (*models.OpenAITopicResponse, error) {
	var topics models.OpenAITopicResponse

	slog.Info("[TopicGenerator] Generating Topics from NewsAPI Top Headlines")
	jsonBytes, err := json.Marshal(headlinesResponse)
	if err != nil {
		slog.Error("[TopicGenerator] Failed to marshal NewsAPI headlines", slog.String("error", err.Error()))
		return nil, fmt.Errorf("[TopicGenerator] Failed to marshal NewsAPI headlines: %w", err)
	}

	prompt := `
                    Extract key topics from the following JSON object containing news article titles.

                    🔹 **Rules:**
                    - Condense each topic into a **short, precise phrase** that can be used as a search query.
                    - Ensure topics are **general enough** to be searched across different APIs.
                    - Categorize each topic into one of the following categories:
                    - **Technology**
                    - **Business & Finance**
                    - **Politics & World Affairs**
                    - **Entertainment & Pop Culture**
                    - **Health & Science**
                    - **Sports**
                    - **Lifestyle & Society**
                    - **Memes & Internet Trends**
                    - **Crime & Law**

                    🔹 **Return JSON Output Format:**
                    using the following structure:
                    {
                        "topics" : [
                            {
                                "topic" : "XXX",
                                "category": "XXX"
                            }
                        ]
                    }
                    `

	start := time.Now()
	chatComplettion, err := clients.GetAIClient().Client.Chat.Completions.New(context.TODO(),
		openai.ChatCompletionNewParams{
			Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(prompt),
				openai.UserMessage(string(jsonBytes)),
			}),
			Model: openai.F(openai.ChatModelChatgpt4oLatest),
		})
	elapsed := time.Since(start)
	if err != nil {
		slog.Error("[TopicGenerator] OpenAI API call failed",
			slog.String("error", err.Error()),
			slog.Duration("duration", elapsed))

		return nil, fmt.Errorf("[TopicGenerator] OpenAI API call failed: %w", err)
	}

	slog.Info("[TopicGenerator] OpenAI API call completed", slog.Duration("duration", elapsed))

	if len(chatComplettion.Choices) == 0 || strings.TrimSpace(chatComplettion.Choices[0].Message.Content) == "" {
		slog.Error("[TopicGenerator] OpenAI returned an empty response")
		return nil, errors.New("[TopicGenerator] OpenAI returned an empty response")
	}

	topicsRaw := strings.TrimSpace(chatComplettion.Choices[0].Message.Content)
	topicsRaw = strings.TrimPrefix(topicsRaw, "```json")
	topicsRaw = strings.TrimSuffix(topicsRaw, "```")
	topicsRaw = strings.TrimSpace(topicsRaw)

	err = json.Unmarshal([]byte(topicsRaw), &topics)
	if err != nil {
		slog.Error("[TopicGenerator] Failed to parse OpenAI response", slog.String("error", err.Error()))
		return nil, fmt.Errorf("[TopicGenerator] Failed to parse OpenAI response: %w", err)
	}

	slog.Info("[TopicGenerator] Successfully generated topics from headlines")
	return &topics, nil
}
