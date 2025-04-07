package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/spacesedan/sentiflow/internal/models"
)

var (
	opensearchInstance Opensearch
	openseachOnce      sync.Once
)

type Opensearch struct {
	Client *opensearch.Client
}

func GetOpensearchClient(ctx context.Context) Opensearch {
	openseachOnce.Do(func() {
		appEnv := os.Getenv("APP_ENV")

		var cfg opensearch.Config

		if appEnv == "prod" {
			awsCfg, err := config.LoadDefaultConfig(ctx)
			if err != nil {
				log.Fatalf("failed to laod AWS config: %v", err)
			}

			signer := v4.NewSigner()
			creds := awsCfg.Credentials
			region := awsCfg.Region

			cfg = opensearch.Config{
				Addresses: []string{os.Getenv("AWS_OPENSEARCH_ENDPOINT")},
				Transport: NewSigV4Transport(creds, signer, region, "es"),
			}

		} else {

			if os.Getenv("OPENSEARCH_ENDPOINT") == "" || os.Getenv("OPENSEARCH_PASSWORD") == "" {
				log.Fatal("Missing credentials for opensearch")
			}
			cfg = opensearch.Config{
				Addresses: []string{os.Getenv("OPENSEARCH_ENDPOINT")},
				Password:  os.Getenv("OPENSEARCH_PASSWORD"),
			}
		}

		client, err := opensearch.NewClient(cfg)
		if err != nil {
			log.Fatalf("failed to initialize OpenSearch Client: %v", err.Error())
		}

		opensearchInstance = Opensearch{
			client,
		}
	})
	return opensearchInstance
}

type sigV4Transport struct {
	credentials aws.CredentialsProvider
	signer      *v4.Signer
	region      string
	service     string
	next        http.RoundTripper
}

func NewSigV4Transport(creds aws.CredentialsProvider, signer *v4.Signer, region string, service string) http.RoundTripper {
	return &sigV4Transport{
		credentials: creds,
		signer:      signer,
		region:      region,
		service:     service,
		next:        http.DefaultTransport,
	}
}

func (t *sigV4Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	creds, err := t.credentials.Retrieve(context.Background())
	if err != nil {
		return nil, err
	}

	signedReq := req.Clone(req.Context())

	signedReq.Header.Del("Authorization")

	err = t.signer.SignHTTP(
		context.Background(),
		creds,
		signedReq,
		v4.GetPayloadHash(req.Context()),
		t.service,
		t.region,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}

	return t.next.RoundTrip(signedReq)
}

func (o Opensearch) IsHealthy(ctx context.Context) bool {
	req := opensearchapi.ClusterHealthReq{}
	res, err := o.Client.Do(ctx, req, nil)
	if err != nil {
		return false
	}
	defer res.Body.Close()

	if res.IsError() {
		return false
	}

	return res.StatusCode == http.StatusOK
}

func (o Opensearch) IndexTopic(ctx context.Context, topic models.Topic) error {
	slog.Info("[OpenSearchClient] Indexing new topic",
		slog.String("topic", topic.Topic))

	index := "topics"

	payload, err := json.Marshal(topic)
	if err != nil {
		slog.Error("[OpenSearchClient] failed to marshal topic",
			slog.String("topic", topic.Topic),
			slog.String("error", err.Error()))
		return err
	}

	req := opensearchapi.IndexReq{
		Index:      index,
		DocumentID: topic.URL,
		Body:       strings.NewReader(string((payload))),
	}

	res, err := o.Client.Do(ctx, req, nil)
	if err != nil {
		slog.Error("[OpenSearchClient] Failed to index topic",
			slog.String("error", err.Error()))
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		slog.Error("[OpenSearchClient] OpenSearch indexing error",
			slog.String("status", res.Status()))
		return fmt.Errorf("opensearch error: %s", res.Status())
	}

	return nil
}

func (o Opensearch) IndexSentimentResult(ctx context.Context, result models.SentimentAnalysisResult) error {
	slog.Info("[OpenSearchClient] Indexing new sentiment result",
		slog.String("content_id", result.ContentID))

	index := "sentiment-results"

	payload, err := json.Marshal(result)
	if err != nil {
		slog.Error("[OpenSearchClient] failed to marshal sentiment result",
			slog.String("content_id", result.ContentID),
			slog.String("error", err.Error()))
		return err
	}

	req := opensearchapi.IndexReq{
		Index:      index,
		DocumentID: result.ContentID,
		Body:       strings.NewReader(string((payload))),
	}

	res, err := o.Client.Do(ctx, req, nil)
	if err != nil {
		slog.Error("[OpenSearchClient] Failed to index sentiment result",
			slog.String("error", err.Error()))
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		slog.Error("[OpenSearchClient] OpenSearch indexing error",
			slog.String("status", res.Status()))
		return fmt.Errorf("opensearch error: %s", res.Status())
	}

	return nil
}
