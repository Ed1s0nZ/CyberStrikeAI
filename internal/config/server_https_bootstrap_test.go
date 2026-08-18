package config

import "testing"

func TestApplyPlainHTTPBootstrapDisablesConfiguredTLS(t *testing.T) {
	enabled := true
	cfg := &Config{
		Server: ServerConfig{
			TLSEnabled:      true,
			TLSAutoSelfSign: true,
			TLSCertPath:     "/tmp/server.crt",
			TLSKeyPath:      "/tmp/server.key",
			TLSHTTPRedirect: &enabled,
		},
	}

	ApplyPlainHTTPBootstrap(cfg)

	if MainWebUIUsesHTTPS(&cfg.Server) {
		t.Fatal("expected --http bootstrap to disable main web UI HTTPS")
	}
	if ServerHTTPRedirectEnabled(&cfg.Server) {
		t.Fatal("expected --http bootstrap to disable HTTP to HTTPS redirect")
	}
	if cfg.Server.TLSCertPath != "" || cfg.Server.TLSKeyPath != "" {
		t.Fatalf("expected TLS cert paths to be cleared, got cert=%q key=%q", cfg.Server.TLSCertPath, cfg.Server.TLSKeyPath)
	}
	if cfg.Server.TLSHTTPRedirect == nil || *cfg.Server.TLSHTTPRedirect {
		t.Fatal("expected TLSHTTPRedirect to be explicitly disabled")
	}
}
