package gateway

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

// Proxier forwards a request to the given upstream URL.
type Proxier interface {
	Proxy(w http.ResponseWriter, r *http.Request, upstream string)
}

// reverseProxier wraps net/http/httputil.ReverseProxy, caching one proxy
// per upstream so each request doesn't re-parse the upstream URL.
type reverseProxier struct {
	mu      sync.RWMutex
	proxies map[string]*httputil.ReverseProxy
}

// NewReverseProxier returns a Proxier backed by httputil.ReverseProxy.
func NewReverseProxier() Proxier {
	return &reverseProxier{proxies: make(map[string]*httputil.ReverseProxy)}
}

// Proxy implements Proxier.
func (p *reverseProxier) Proxy(w http.ResponseWriter, r *http.Request, upstream string) {
	proxy, err := p.proxyFor(upstream)
	if err != nil {
		writeError(w, http.StatusBadGateway, "bad_gateway", "upstream is misconfigured")
		return
	}
	proxy.ServeHTTP(w, r)
}

func (p *reverseProxier) proxyFor(upstream string) (*httputil.ReverseProxy, error) {
	p.mu.RLock()
	proxy, ok := p.proxies[upstream]
	p.mu.RUnlock()
	if ok {
		return proxy, nil
	}

	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("parse upstream url %q: %w", upstream, err)
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("upstream url %q must include scheme and host", upstream)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if proxy, ok := p.proxies[upstream]; ok {
		return proxy, nil
	}
	proxy = httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, _ error) {
		writeError(w, http.StatusBadGateway, "bad_gateway", "upstream unavailable")
	}
	p.proxies[upstream] = proxy
	return proxy, nil
}
