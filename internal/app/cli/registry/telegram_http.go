package registry

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/drybin/palisade/internal/app/cli/config"
	"github.com/drybin/palisade/pkg/wrap"
	"github.com/go-resty/resty/v2"
	"golang.org/x/net/proxy"
)

// newTelegramRestyClient returns a Resty client for Telegram API.
// If tg.Socks5ProxyURL is set (TG_SOCKS5_PROXY), outbound TCP uses SOCKS5; MEXC clients are unaffected.
func newTelegramRestyClient(tg config.TgConfig) (*resty.Client, error) {
	raw := strings.TrimSpace(tg.Socks5ProxyURL)
	if raw == "" {
		return resty.New(), nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, wrap.Errorf("invalid TG_SOCKS5_PROXY: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "socks5", "socks5h":
	default:
		return nil, wrap.Errorf("TG_SOCKS5_PROXY must use socks5 or socks5h scheme, got %q", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return nil, wrap.Errorf("TG_SOCKS5_PROXY: host is required")
	}
	port := u.Port()
	if port == "" {
		port = "1080"
	}
	addr := net.JoinHostPort(host, port)

	var auth *proxy.Auth
	if u.User != nil {
		user := u.User.Username()
		pass, hasPass := u.User.Password()
		if user != "" || hasPass {
			auth = &proxy.Auth{User: user, Password: pass}
		}
	}

	socksDialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
	if err != nil {
		return nil, wrap.Errorf("TG_SOCKS5_PROXY: SOCKS5 dialer: %w", err)
	}

	contextDialer, ok := socksDialer.(proxy.ContextDialer)
	if !ok {
		return nil, wrap.Errorf("TG_SOCKS5_PROXY: internal error: SOCKS5 dialer has no DialContext")
	}

	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           contextDialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{Transport: transport}
	return resty.NewWithClient(httpClient), nil
}
