// vpngate-csv-hostless: minimal authenticated VPNGate CSV proxy.
// Mirrors the Cloudflare Worker "vpngate-csv-proxy" contract:
//   GET /csv  (Bearer token or X-API-Key) -> fresh VPNGate CSV, 120s in-memory cache
//   GET /health -> {"status":"ok"} (for platform health checks, no auth)
//   everything else -> 404, silently
// API_TOKEN comes from the environment; if unset the /csv endpoint fails closed (503).
package main

import (
	"crypto/subtle"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const upstreamURL = "https://www.vpngate.net/api/iphone/"
const cacheTTL = 120 * time.Second

var (
	mu       sync.Mutex
	cache    []byte
	cacheAt  time.Time
	client   = &http.Client{Timeout: 30 * time.Second}
	token    = os.Getenv("API_TOKEN")
)

func extractToken(r *http.Request) string {
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		return strings.TrimSpace(a[len("Bearer "):])
	}
	return r.Header.Get("X-API-Key")
}

func fetchCSV() ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()
	if cache != nil && time.Since(cacheAt) < cacheTTL {
		return cache, nil
	}
	req, err := http.NewRequest(http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	// VPNGate serves a tiny HTML stub page to filtered egress IPs.
	// Treat anything that is not the real CSV as an upstream failure
	// so a blocked egress is loud (502), never a silent bad list.
	if !strings.HasPrefix(string(body), "*vpn_servers") {
		return nil, &upstreamError{status: resp.StatusCode, size: len(body)}
	}
	cache = body
	cacheAt = time.Now()
	return body, nil
}

type upstreamError struct {
	status int
	size   int
}

func (e *upstreamError) Error() string {
	return "upstream did not return a valid CSV"
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/csv", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if token == "" {
			http.Error(w, "service not configured", http.StatusServiceUnavailable)
			return
		}
		got := extractToken(r)
		if len(got) != len(token) || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.NotFound(w, r)
			return
		}
		body, err := fetchCSV()
		if err != nil {
			log.Printf("fetch failed: %v", err)
			http.Error(w, "upstream fetch failed", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=120")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	log.Printf("listening on 0.0.0.0:%s (token configured: %v)", port, token != "")
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, mux))
}
