package traffic

import (
	"math/rand"
	"net/http"
	"time"
)

// Common browser User-Agents
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
}

// Common languages
var languages = []string{
	"en-US,en;q=0.9",
	"en-GB,en;q=0.9",
	"zh-CN,zh;q=0.9,en;q=0.8",
	"ja,en-US;q=0.9,en;q=0.8",
	"de-DE,de;q=0.9,en;q=0.8",
	"fr-FR,fr;q=0.9,en;q=0.8",
	"es-ES,es;q=0.9,en;q=0.8",
}

// Common origins
var origins = []string{
	"https://www.google.com",
	"https://www.github.com",
	"https://www.stackoverflow.com",
	"https://www.reddit.com",
	"https://www.youtube.com",
	"https://www.facebook.com",
	"https://www.twitter.com",
}

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// GenerateRealisticHeaders generates realistic HTTP headers to mask traffic
func GenerateRealisticHeaders() http.Header {
	headers := make(http.Header)
	
	// User-Agent: Random browser
	headers.Set("User-Agent", userAgents[rng.Intn(len(userAgents))])
	
	// Accept: WebSocket related
	headers.Set("Accept", "*/*")
	
	// Accept-Language: Random language
	headers.Set("Accept-Language", languages[rng.Intn(len(languages))])
	
	// Accept-Encoding: Common encodings
	headers.Set("Accept-Encoding", "gzip, deflate, br")
	
	// Origin: Random common origin
	headers.Set("Origin", origins[rng.Intn(len(origins))])
	
	// Cache-Control: Common for WebSocket
	headers.Set("Cache-Control", "no-cache")
	
	// Pragma: Common for WebSocket
	headers.Set("Pragma", "no-cache")
	
	// Note: Connection header is NOT set here to avoid duplicate with WebSocket library
	// WebSocket library automatically adds "Connection: Upgrade" header
	
	// Upgrade: websocket (required for WebSocket)
	// Note: Upgrade header is NOT set here to avoid duplicate with WebSocket library
	// WebSocket library automatically adds "Upgrade: websocket" header
	
	return headers
}

// MergeHeaders merges user-provided headers with realistic headers
// User headers take precedence
func MergeHeaders(userHeaders, realisticHeaders http.Header) http.Header {
	result := make(http.Header)
	
	// First, copy realistic headers
	for k, v := range realisticHeaders {
		result[k] = v
	}
	
	// Then, override with user headers
	for k, v := range userHeaders {
		result[k] = v
	}
	
	return result
}

