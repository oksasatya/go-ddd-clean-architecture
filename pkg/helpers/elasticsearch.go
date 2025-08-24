package helpers

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

// NewESClient creates an Elasticsearch client with sane defaults and optional basic auth.
func NewESClient(addrs []string, username, password string) (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: addrs,
		Username:  username,
		Password:  password,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: 5 * time.Second,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		},
	}
	return elasticsearch.NewClient(cfg)
}
