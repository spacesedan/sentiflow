package clients

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

var (
	opensearchInstance Opensearch
	openseachOnce      sync.Once
)

type Opensearch struct {
	Client *opensearchapi.Client // Changed to *opensearchapi.Client
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

		// cfg is of type opensearch.Config
		// We need to wrap it in opensearchapi.Config
		apiCfg := opensearchapi.Config{
			Client: cfg, // Embed the opensearch.Config here
		}

		client, err := opensearchapi.NewClient(apiCfg) // Use opensearchapi.NewClient
		if err != nil {
			log.Fatalf("failed to initialize OpenSearch API Client: %v", err.Error())
		}

		opensearchInstance = Opensearch{
			Client: client,
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
