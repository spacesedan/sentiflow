package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/spacesedan/sentiflow/config"
)

const (
	NEWS_API_ENDPOINT = "https://newsapi.org/v2/top-headlines?country=us&pageSize=100&apiKey="
)

type ProducerConfig = struct {
	debug bool
}

type NewsAPIResponse = struct {
	Status       string            `json:"status"`
	TotalResults int               `json:"totalResults"`
	Articels     []NewsAPIArticles `json:"articles"`
}

type NewsAPIArticles = struct {
	Source struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"source"`
	Title string `json:"title"`
}

type OpenAITopicResponse = struct {
	Topics []struct {
		Topic    string `json:"topic"`
		Category string `json:"category"`
	} `json:"topics"`
}

func main() {
	_ = ProducerConfig{
		debug: true,
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	config.LoadEnv(env)

	client := &http.Client{}

	url := NEWS_API_ENDPOINT + os.Getenv("NEWS_API_KEY")

	req, err := http.NewRequest(
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		panic(err)
	}

	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	newsAPIResponse := NewsAPIResponse{}
	_ = json.Unmarshal(body, &newsAPIResponse)

	formattedResponse, err := json.MarshalIndent(newsAPIResponse, "", "     ")
	if err != nil {
		panic(err)
	}
	stringRepsonse := string(formattedResponse)

	openaiClient := OpenAIClient()

	chatComplettion, err := openaiClient.Chat.Completions.New(context.TODO(),
		openai.ChatCompletionNewParams{
			Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(`
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
                    `),

				openai.UserMessage(stringRepsonse),
			}),
			Model: openai.F(openai.ChatModelChatgpt4oLatest),
		})
	if err != nil {
		panic(err)
	}
	topicsRaw := chatComplettion.Choices[0].Message.Content

	topicsRaw = strings.TrimPrefix(topicsRaw, "```json")
	topicsRaw = strings.TrimSuffix(topicsRaw, "```")

	fmt.Println(topicsRaw)

	topics := OpenAITopicResponse{}

	err = json.Unmarshal([]byte(topicsRaw), &topics)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	for _, topic := range topics.Topics {
		fmt.Printf("Topic: %v Category: %v\n", topic.Topic, topic.Category)
	}
}

func OpenAIClient() *openai.Client {
	client := openai.NewClient(
		option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	return client
}
