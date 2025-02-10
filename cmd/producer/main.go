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
	"github.com/subosito/gotenv"
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
		Topic string `json:"topic"`
	} `json:"topics"`
}

func main() {
	_ = ProducerConfig{
		debug: true,
	}

	if err := LoadEnvironment(); err != nil {
		panic(err)
	}

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
                    generate a list of topics using the 'title' of the following json object
                    and return the list as a json array named 'topics' with an object of 'topic' followed by the topic.
                    can you condense the topic as much as possile so that i can use the result as a query in other apis
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

	topics := OpenAITopicResponse{}

	err = json.Unmarshal([]byte(topicsRaw), &topics)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	for _, topic := range topics.Topics {
		fmt.Println(topic.Topic)
	}
}

func OpenAIClient() *openai.Client {
	client := openai.NewClient(
		option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	return client
}

func LoadEnvironment() error {
	return gotenv.Load()
}
