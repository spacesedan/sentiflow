package streams

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/db"
)

func ConsumeTopicsStream(ctx context.Context) {
	client := clients.GetDynamoDBStreamClient()

	_, err := client.ListStreams(ctx, &dynamodbstreams.ListStreamsInput{
		TableName: aws.String(db.TOPICS_TABLE_NAME),
	}, nil)
	if err != nil {
		slog.Error("Uh-on", slog.String("err", err.Error()))
		log.Panic(err)
	}

	fmt.Println("Running ok")
}
