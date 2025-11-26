package traffic

import (
	"net/http"
	"testing"
)

func TestGenerateRealisticHeaders(t *testing.T) {
	headers := GenerateRealisticHeaders()
	
	// Check that required headers are present
	requiredHeaders := []string{
		"User-Agent",
		"Accept",
		"Accept-Language",
		"Accept-Encoding",
		"Origin",
		"Connection",
		"Upgrade",
	}
	
	for _, header := range requiredHeaders {
		if headers.Get(header) == "" {
			t.Errorf("Required header %s is missing", header)
		}
	}
	
	// Verify User-Agent is from our list
	userAgent := headers.Get("User-Agent")
	found := false
	for _, ua := range userAgents {
		if ua == userAgent {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("User-Agent %s is not in the expected list", userAgent)
	}
	
	// Verify Upgrade header
	if headers.Get("Upgrade") != "websocket" {
		t.Errorf("Upgrade header should be 'websocket', got '%s'", headers.Get("Upgrade"))
	}
}

func TestMergeHeaders(t *testing.T) {
	realisticHeaders := GenerateRealisticHeaders()
	userHeaders := make(http.Header)
	userHeaders.Set("User-Agent", "Custom-Agent/1.0")
	userHeaders.Set("X-Custom-Header", "custom-value")
	
	merged := MergeHeaders(userHeaders, realisticHeaders)
	
	// User headers should take precedence
	if merged.Get("User-Agent") != "Custom-Agent/1.0" {
		t.Errorf("User header should take precedence, got '%s'", merged.Get("User-Agent"))
	}
	
	// Custom header should be present
	if merged.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Custom header should be present, got '%s'", merged.Get("X-Custom-Header"))
	}
	
	// Realistic headers should still be present
	if merged.Get("Accept") == "" {
		t.Error("Realistic headers should still be present")
	}
}



