package clients

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"strings"
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
	mu     sync.Mutex
}

const VALKEY_REDDIT_KEY = "reddit:processed_posts"

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

func (vc *ValkeyClient) recreateClient() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("[ValkeyClient] Recreate failed and was recovered from panic",
				slog.Any("panic", r))
		}
	}()

	vc.mu.Lock()
	defer vc.mu.Unlock()
	slog.Warn("[ValkeyClient] Attempting to recreate Valkey client...")
	vc.Client.Close()
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
	vc.Client = client
}

func CloseValkey() {
	if valkeyInstance != nil {
		valkeyInstance.Client.Close()
	}
}

func GetValkeyClient() *ValkeyClient {
	if valkeyInstance == nil {
		panic("[ValkeyClient] Error: Valkey client is not initilialized")
	}
	return valkeyInstance
}

func (vc *ValkeyClient) MarkProcessed(ctx context.Context, source string, key string) error {
	sourceKey := keyFromSource(source)
	completed := []valkey.Completed{
		vc.Client.B().Sadd().Key(sourceKey).Member(key).Build(),
		vc.Client.B().Expire().Key(sourceKey).Seconds(86400).Build(),
	}

	responses := vc.DoMultiWithRetry(ctx, completed, 3)
	for _, res := range responses {
		if err := res.Error(); err != nil {
			return err
		}
	}

	slog.Info("[ValkeyClient] Proccessed to Valkey Successfully",
		slog.String("key", key))
	return nil
}

func (vc *ValkeyClient) IsPostProcessed(ctx context.Context, source string, key string) bool {
	sourceKey := keyFromSource(source)
	res := vc.DoWithRetry(ctx, vc.Client.B().Sismember().Key(sourceKey).Member(key).Build(), 3)

	if err := res.Error(); isConnectionError(err) {
		vc.recreateClient()
	}

	ok, err := res.AsBool()
	if err != nil {
		return false
	}

	return ok
}

func keyFromSource(source string) string {
	switch source {
	case "reddit":
		return VALKEY_REDDIT_KEY
	default:
		return ""
	}
}

func (vc *ValkeyClient) DoMultiWithRetry(ctx context.Context, completed []valkey.Completed, retries int) []valkey.ValkeyResult {
	var results []valkey.ValkeyResult

	for i := 0; i < retries; i++ {
		results = vc.Client.DoMulti(ctx, completed...)
		hasErr := false
		for _, r := range results {
			if r.Error() != nil {
				hasErr = true
				slog.Warn("[ValkeyClient] Do Multi failed",
					slog.Int("attempt", i+1),
					slog.String("error", r.Error().Error()))
				if isConnectionError(r.Error()) {
					vc.recreateClient()
				}
				break
			}
		}
		if !hasErr {
			break
		}
		time.Sleep(time.Millisecond * 250)
	}

	return results
}

func (vc *ValkeyClient) DoWithRetry(ctx context.Context, completed valkey.Completed, retries int) valkey.ValkeyResult {
	var result valkey.ValkeyResult
	for i := 0; i < retries; i++ {
		result = vc.Client.Do(ctx, completed)
		if result.Error() == nil {
			break
		}

		slog.Warn("[ValkeyClient] Do failed",
			slog.Int("attempt", i+1),
			slog.String("error", result.Error().Error()))

		time.Sleep(250 * time.Millisecond)
	}

	return result
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "i/o timeout")
}
