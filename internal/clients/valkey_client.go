package clients

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/valkey-io/valkey-go"
)

var (
	valkeyInstance *ValkeyClient
	valkeyOnce     sync.Once
)

type ValkeyClient struct {
	Client valkey.Client
}

func InitValkey() *ValkeyClient {
	valkeyOnce.Do(func() {
		valkeyAddr := os.Getenv("VALKEY_INIT_ADDRESS")
		valkeyPassword := os.Getenv("VALKEY_PASSWORD")
		useTLS := os.Getenv("VALKEY_TLS") == "true"

		opts := valkey.ClientOption{
			InitAddress: []string{
				valkeyAddr,
			},
			Password:         valkeyPassword,
			ConnWriteTimeout: 5 * time.Second,
			SelectDB:         0,
		}

		if useTLS {
			opts.TLSConfig = &tls.Config{InsecureSkipVerify: false}
		}

		client, err := valkey.NewClient(opts)
		if err != nil {
			panic(fmt.Errorf("[ValkeyClient] failed to create Valkey: %w", err))
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		c := client.Do(ctx, client.B().Ping().Build())
		if c.Error() != nil {
			panic(fmt.Errorf("[ValkeyClient] failed to ping Valkey: %w", err))
		}

		slog.Info("[ValkeyClient] Successfully connected to valkey")

		valkeyInstance = &ValkeyClient{Client: client}
	})
	return valkeyInstance
}

func CloseValkey() {
	if valkeyInstance != nil {
		valkeyInstance.Client.Close()
	}
}

func GetValkeyClient() valkey.Client {
	if valkeyInstance == nil {
		panic("[ValkeyClient] Error: Valkey client is not initilialized")
	}
	return valkeyInstance.Client
}
