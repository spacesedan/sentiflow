package clients

import (
	"context"
	"log/slog"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var (
	awsCfg  aws.Config
	awsOnce sync.Once
)

func GetAWSConfig() aws.Config {
	awsOnce.Do(func() {
		slog.Info("[AWSClient] Initializing AWS Config...")
		// TODO: add environment based config
		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithRegion("us-west-2"))
		if err != nil {
			slog.Error("[AWSClient] Failed to load AWS config")
			panic(err)
		}
		awsCfg = cfg
		slog.Info("[AWSClient] AWS Config Initialized")
	})

	return awsCfg
}

func GetDynamoDBClient() *dynamodb.Client {
	localDynamoDBEnpoint := "http://localhost:8000"
	return dynamodb.NewFromConfig(GetAWSConfig(), func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(localDynamoDBEnpoint)
	})
}
