package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunRejectsUnsupportedTransport(t *testing.T) {
	err := run(context.Background(), nil, "websocket", "", "")
	if err == nil || !strings.Contains(err.Error(), "unsupported transport") {
		t.Fatalf("run error = %v; want unsupported transport error", err)
	}
}

func TestRunHTTPRejectsEmptyAddress(t *testing.T) {
	err := runHTTP(context.Background(), nil, " ", "/mcp")
	if err == nil || !strings.Contains(err.Error(), "address must not be empty") {
		t.Fatalf("runHTTP error = %v; want empty address error", err)
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("DEPSDEVMCP_TEST_VALUE", "configured")
	if got := envOrDefault("DEPSDEVMCP_TEST_VALUE", "fallback"); got != "configured" {
		t.Fatalf("envOrDefault configured = %q; want configured", got)
	}
	t.Setenv("DEPSDEVMCP_TEST_VALUE", "")
	if got := envOrDefault("DEPSDEVMCP_TEST_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("envOrDefault empty = %q; want fallback", got)
	}
}
