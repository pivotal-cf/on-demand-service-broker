package network

import (
	"github.com/craigfurman/herottp"
	"time"
)

func NewDefaultHTTPClient() *herottp.Client {
	return herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
	})
}
