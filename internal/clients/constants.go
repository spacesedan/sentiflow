package clients

import "time"

const (
	MAX_RETRIES     = 5
	INITIAL_BACKOFF = 1 * time.Second
	MAX_BACKOFF     = 32 * time.Second
	USER_AGENT      = "sentiflow-client/1.0 (+https://github.com/spacesedan/sentiflow)"
)
