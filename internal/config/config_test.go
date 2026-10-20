package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := LoadWithArgs([]string{}, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.Env != EnvDevelopment {
		t.Errorf("expected env development, got %s", cfg.Env)
	}
	if !cfg.SeedData {
		t.Errorf("expected SeedData true in development, got false")
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	env := []string{
		"AOSPHERE_PORT=9000",
		"AOSPHERE_ENV=production",
		"AOSPHERE_DB_PATH=/var/lib/aosphere.db",
		"AOSPHERE_MCP_STDIO=true",
		"AOSPHERE_LOG_LEVEL=warn",
	}

	cfg, err := LoadWithArgs([]string{}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Port)
	}
	if cfg.Env != EnvProduction {
		t.Errorf("expected env production, got %s", cfg.Env)
	}
	// In production, SeedData defaults to false unless explicitly overridden
	if cfg.SeedData {
		t.Errorf("expected SeedData false in production by default, got true")
	}
	if cfg.DBPath != "/var/lib/aosphere.db" {
		t.Errorf("expected db path /var/lib/aosphere.db, got %s", cfg.DBPath)
	}
	if !cfg.MCPStdio {
		t.Errorf("expected MCPStdio true, got false")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("expected log level warn, got %s", cfg.LogLevel)
	}
}

func TestFlagPrecedenceOverEnvironment(t *testing.T) {
	env := []string{
		"AOSPHERE_PORT=9000",
		"AOSPHERE_ENV=development",
	}
	args := []string{
		"-port", "8888",
		"-env", "production",
		"-seed=false",
	}

	cfg, err := LoadWithArgs(args, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 8888 {
		t.Errorf("expected flag port 8888 to override env 9000, got %d", cfg.Port)
	}
	if cfg.Env != EnvProduction {
		t.Errorf("expected flag env production to override env development, got %s", cfg.Env)
	}
	if cfg.SeedData {
		t.Errorf("expected SeedData false from flag, got true")
	}
}

func TestInvalidConfig(t *testing.T) {
	_, err := LoadWithArgs([]string{"-port", "99999"}, []string{})
	if err == nil {
		t.Error("expected error for invalid port > 65535, got nil")
	}

	_, err = LoadWithArgs([]string{"-env", "invalid_env"}, []string{})
	if err == nil {
		t.Error("expected error for invalid env name, got nil")
	}
}
