package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Environment represents the deployment stage
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvTest        Environment = "test"
	EnvProduction  Environment = "production"
)

// Config holds all configuration options for AOSphere BOS
type Config struct {
	Env          Environment
	Port         int
	DBPath       string
	SeedData     bool
	MCPStdio     bool
	LogLevel     string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Load loads configuration from command line flags, environment variables, and defaults.
// Precedence: Command-line flags > Environment variables > Defaults.
func Load() (*Config, error) {
	return LoadWithArgs(os.Args[1:], os.Environ())
}

// LoadWithArgs allows isolated testing with custom CLI args and environment slices.
func LoadWithArgs(args []string, envPairs []string) (*Config, error) {
	envMap := make(map[string]string)
	for _, pair := range envPairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// 1. Initial defaults
	cfg := &Config{
		Env:          EnvDevelopment,
		Port:         8080,
		DBPath:       filepath.Join("data", "aosphere.db"),
		SeedData:     true,
		MCPStdio:     false,
		LogLevel:     "info",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// 2. Override with environment variables
	if val, ok := envMap["AOSPHERE_ENV"]; ok && val != "" {
		cfg.Env = Environment(strings.ToLower(val))
	}
	if cfg.Env == EnvProduction {
		cfg.SeedData = false // Default to no seeding in production
	}

	if val, ok := envMap["AOSPHERE_PORT"]; ok && val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			cfg.Port = p
		}
	}

	if val, ok := envMap["AOSPHERE_DB_PATH"]; ok && val != "" {
		cfg.DBPath = val
	}

	if val, ok := envMap["AOSPHERE_SEED"]; ok && val != "" {
		if s, err := strconv.ParseBool(val); err == nil {
			cfg.SeedData = s
		}
	}

	if val, ok := envMap["AOSPHERE_MCP_STDIO"]; ok && val != "" {
		if m, err := strconv.ParseBool(val); err == nil {
			cfg.MCPStdio = m
		}
	}

	if val, ok := envMap["AOSPHERE_LOG_LEVEL"]; ok && val != "" {
		cfg.LogLevel = strings.ToLower(val)
	}

	// 3. Override with command-line flags
	fs := flag.NewFlagSet("aosphere", flag.ContinueOnError)
	portFlag := fs.Int("port", cfg.Port, "HTTP server port (AOSPHERE_PORT)")
	dbPathFlag := fs.String("db", cfg.DBPath, "Path to SQLite database file (AOSPHERE_DB_PATH)")
	envFlag := fs.String("env", string(cfg.Env), "Environment: development, test, production (AOSPHERE_ENV)")
	seedFlag := fs.Bool("seed", cfg.SeedData, "Seed starter demo data on initialization (AOSPHERE_SEED)")
	mcpStdioFlag := fs.Bool("mcp-stdio", cfg.MCPStdio, "Run as an MCP server over stdio (AOSPHERE_MCP_STDIO)")
	logLevelFlag := fs.String("log-level", cfg.LogLevel, "Log level: debug, info, warn, error (AOSPHERE_LOG_LEVEL)")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	cfg.Port = *portFlag
	cfg.DBPath = *dbPathFlag
	cfg.Env = Environment(strings.ToLower(*envFlag))
	cfg.SeedData = *seedFlag
	cfg.MCPStdio = *mcpStdioFlag
	cfg.LogLevel = strings.ToLower(*logLevelFlag)

	// Validate config
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("invalid port %d: must be between 1 and 65535", cfg.Port)
	}
	if cfg.Env != EnvDevelopment && cfg.Env != EnvTest && cfg.Env != EnvProduction {
		return nil, fmt.Errorf("invalid environment %q: must be development, test, or production", cfg.Env)
	}

	return cfg, nil
}
