package main

import (
	"os"
	"testing"
)

func TestLoadEnvConfigDefaults(t *testing.T) {
	clearEnv(t)
	os.Setenv("FGTECH_INGRESS_FQDN", "apps.example.com")
	os.Setenv("FGTECH_INGRESS_CLASSNAME", "nginx")

	cfg, err := loadEnvConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IngressHost != "apps.example.com" {
		t.Fatalf("IngressHost = %s, want apps.example.com", cfg.IngressHost)
	}
	if cfg.IngressTLSSecret != "fgtech-tls" {
		t.Fatalf("IngressTLSSecret = %s, want fgtech-tls", cfg.IngressTLSSecret)
	}
	if cfg.IngressClassName != "nginx" {
		t.Fatalf("IngressClassName = %s, want nginx", cfg.IngressClassName)
	}
	if cfg.DefaultServiceAccount != "default" {
		t.Fatalf("DefaultServiceAccount = %s, want default", cfg.DefaultServiceAccount)
	}
	if cfg.DefaultTTLSeconds != 3600 {
		t.Fatalf("DefaultTTLSeconds = %d, want 3600", cfg.DefaultTTLSeconds)
	}
	if cfg.PodPort != 8080 {
		t.Fatalf("PodPort = %d, want 8080", cfg.PodPort)
	}
}

func TestLoadEnvConfigOverrides(t *testing.T) {
	clearEnv(t)
	os.Setenv("FGTECH_INGRESS_FQDN", "apps.example.com")
	os.Setenv("FGTECH_INGRESS_TLS_SECRET", "custom-tls")
	os.Setenv("FGTECH_INGRESS_CLASSNAME", "nginx-custom")
	os.Setenv("FGTECH_DEFAULT_TTL_SECONDS", "7200")
	os.Setenv("FGTECH_POD_SERVICEACCOUNT", "sa-custom")
	os.Setenv("FGTECH_POD_PORT", "9090")

	cfg, err := loadEnvConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IngressTLSSecret != "custom-tls" {
		t.Fatalf("IngressTLSSecret = %s, want custom-tls", cfg.IngressTLSSecret)
	}
	if cfg.DefaultTTLSeconds != 7200 {
		t.Fatalf("DefaultTTLSeconds = %d, want 7200", cfg.DefaultTTLSeconds)
	}
	if cfg.DefaultServiceAccount != "sa-custom" {
		t.Fatalf("DefaultServiceAccount = %s, want sa-custom", cfg.DefaultServiceAccount)
	}
	if cfg.PodPort != 9090 {
		t.Fatalf("PodPort = %d, want 9090", cfg.PodPort)
	}
	if cfg.IngressClassName != "nginx-custom" {
		t.Fatalf("IngressClassName = %s, want nginx-custom", cfg.IngressClassName)
	}
}

func TestLoadEnvConfigMissingHost(t *testing.T) {
	clearEnv(t)
	if _, err := loadEnvConfig(); err == nil {
		t.Fatalf("expected error for missing FGTECH_INGRESS_FQDN")
	}
}

func TestLoadEnvConfigMissingClass(t *testing.T) {
	clearEnv(t)
	os.Setenv("FGTECH_INGRESS_FQDN", "apps.example.com")
	if _, err := loadEnvConfig(); err == nil {
		t.Fatalf("expected error for missing FGTECH_INGRESS_CLASSNAME")
	}
}

func TestLoadEnvConfigBadTTL(t *testing.T) {
	clearEnv(t)
	os.Setenv("FGTECH_INGRESS_FQDN", "apps.example.com")
	os.Setenv("FGTECH_DEFAULT_TTL_SECONDS", "-1")
	if _, err := loadEnvConfig(); err == nil {
		t.Fatalf("expected error for invalid FGTECH_DEFAULT_TTL_SECONDS")
	}
}

func TestLoadEnvConfigBadPort(t *testing.T) {
	clearEnv(t)
	os.Setenv("FGTECH_INGRESS_FQDN", "apps.example.com")
	os.Setenv("FGTECH_POD_PORT", "0")
	if _, err := loadEnvConfig(); err == nil {
		t.Fatalf("expected error for invalid FGTECH_POD_PORT")
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	os.Unsetenv("FGTECH_INGRESS_FQDN")
	os.Unsetenv("FGTECH_INGRESS_TLS_SECRET")
	os.Unsetenv("FGTECH_INGRESS_CLASSNAME")
	os.Unsetenv("FGTECH_DEFAULT_TTL_SECONDS")
	os.Unsetenv("FGTECH_POD_SERVICEACCOUNT")
	os.Unsetenv("FGTECH_POD_PORT")
}
