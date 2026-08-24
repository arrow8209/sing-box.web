package settings

import (
	"net"
	"net/http"
	"time"
)

const (
	connectTimeout        = 10 * time.Second
	responseHeaderTimeout = 15 * time.Second
)

// NewDownloadTransport builds the shared transport for every service-owned
// HTTP/HTTPS download. Its proxy callback reads Store on every request, so a
// saved setting takes effect without restarting the service.
func NewDownloadTransport(store *Store) *http.Transport {
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}
	if store != nil {
		transport.Proxy = store.ProxyURL
	}
	return transport
}
