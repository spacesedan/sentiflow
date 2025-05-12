package clients

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
)

var (
	awsCfg   aws.Config
	awsOnce  sync.Once
	endpoint string
)

func GetAWSConfig() aws.Config {
	awsOnce.Do(func() {
		awsEndpoint := os.Getenv("AWS_ENDPOINT")
		if awsEndpoint == "" {
			awsEndpoint = "http://localhost:8000"
		}

		slog.Info("[AWSClient] Initializing AWS Config...")
		// TODO: add environment based config
		cfg, err := config.LoadDefaultConfig(context.Background(), // Changed context.TODO() to context.Background()
			config.WithRegion("us-west-2"))
		if err != nil {
			slog.Error("[AWSClient] Failed to load AWS config")
			panic(err)
		}

		awsCfg = cfg
		endpoint = awsEndpoint
		slog.Info("[AWSClient] AWS Config Initialized")
	})

	return awsCfg
}

func GetDynamoDBClient() *dynamodb.Client {
	return dynamodb.NewFromConfig(GetAWSConfig(), func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
}

func GetDynamoDBStreamClient() *dynamodbstreams.Client {
	return dynamodbstreams.NewFromConfig(GetAWSConfig(), func(o *dynamodbstreams.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
}
