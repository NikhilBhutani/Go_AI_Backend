package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Auth     AuthConfig
	LLM      LLMConfig
	Storage  StorageConfig
	STT      STTConfig
	TTS      TTSConfig
}

type ServerConfig struct {
	Host string
	Port int
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int
	MinConns        int
	MigrationsPath  string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AuthConfig struct {
	SupabaseURL    string
	SupabaseKey    string
	JWTSecret      string
	APIKeyHeader   string
}

type LLMConfig struct {
	OpenAIKey       string
	AnthropicKey    string
	OllamaURL       string
	DefaultProvider  string
	DefaultModel     string
	FallbackProvider string
	MaxRetries       int
}

type StorageConfig struct {
	SupabaseURL    string
	SupabaseKey    string
	Bucket         string
}

type STTConfig struct {
	Backend       string // "openai" or "local"
	OpenAIKey     string
	OpenAIBaseURL string
	OpenAIModel   string
	LocalBaseURL  string // default: "http://localhost:8178"
}

type TTSConfig struct {
	Backend       string // "openai" or "local"
	OpenAIKey     string
	OpenAIBaseURL string
	OpenAIModel   string
	LocalBinPath  string // default: "piper"
	LocalModel    string // required when backend=local
}

func Load() (*Config, error) {
	port, err := getEnvInt("SERVER_PORT", 8080)
	if err != nil {
		return nil, fmt.Errorf("invalid SERVER_PORT: %w", err)
	}

	maxConns, err := getEnvInt("DB_MAX_CONNS", 20)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MAX_CONNS: %w", err)
	}

	minConns, err := getEnvInt("DB_MIN_CONNS", 5)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MIN_CONNS: %w", err)
	}

	redisDB, err := getEnvInt("REDIS_DB", 0)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}

	maxRetries, err := getEnvInt("LLM_MAX_RETRIES", 3)
	if err != nil {
		return nil, fmt.Errorf("invalid LLM_MAX_RETRIES: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: port,
		},
		Database: DatabaseConfig{
			URL:            getEnv("DATABASE_URL", ""),
			MaxConns:       maxConns,
			MinConns:       minConns,
			MigrationsPath: getEnv("MIGRATIONS_PATH", "migrations"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		Auth: AuthConfig{
			SupabaseURL:  getEnv("SUPABASE_URL", ""),
			SupabaseKey:  getEnv("SUPABASE_ANON_KEY", ""),
			JWTSecret:    getEnv("SUPABASE_JWT_SECRET", ""),
			APIKeyHeader: getEnv("API_KEY_HEADER", "X-API-Key"),
		},
		LLM: LLMConfig{
			OpenAIKey:        getEnv("OPENAI_API_KEY", ""),
			AnthropicKey:     getEnv("ANTHROPIC_API_KEY", ""),
			OllamaURL:        getEnv("OLLAMA_URL", "http://localhost:11434"),
			DefaultProvider:  getEnv("LLM_DEFAULT_PROVIDER", "openai"),
			DefaultModel:     getEnv("LLM_DEFAULT_MODEL", "gpt-4"),
			FallbackProvider: getEnv("LLM_FALLBACK_PROVIDER", ""),
			MaxRetries:       maxRetries,
		},
		Storage: StorageConfig{
			SupabaseURL: getEnv("SUPABASE_URL", ""),
			SupabaseKey: getEnv("SUPABASE_SERVICE_KEY", ""),
			Bucket:      getEnv("STORAGE_BUCKET", "documents"),
		},
		STT: STTConfig{
			Backend:       getEnv("STT_BACKEND", "openai"),
			OpenAIKey:     getEnv("OPENAI_API_KEY", ""),
			OpenAIBaseURL: getEnv("STT_OPENAI_BASE_URL", ""),
			OpenAIModel:   getEnv("STT_OPENAI_MODEL", ""),
			LocalBaseURL:  getEnv("STT_LOCAL_BASE_URL", "http://localhost:8178"),
		},
		TTS: TTSConfig{
			Backend:       getEnv("TTS_BACKEND", "openai"),
			OpenAIKey:     getEnv("OPENAI_API_KEY", ""),
			OpenAIBaseURL: getEnv("TTS_OPENAI_BASE_URL", ""),
			OpenAIModel:   getEnv("TTS_OPENAI_MODEL", ""),
			LocalBinPath:  getEnv("TTS_LOCAL_PIPER_BIN", "piper"),
			LocalModel:    getEnv("TTS_LOCAL_PIPER_MODEL", ""),
		},
	}

	return cfg, nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func (c *Config) Validate() error {
	var missing []string
	if c.Database.URL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.Auth.JWTSecret == "" {
		missing = append(missing, "SUPABASE_JWT_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}
