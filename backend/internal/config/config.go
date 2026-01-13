package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env      string
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	AI       AIConfig
	Admin    AdminConfig
}

type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
}

type AuthConfig struct {
	JWTSecret          string
	JWTIssuer          string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	RateLimitPerMinute int
	RateLimitBurst     int
}

type AIConfig struct {
	Provider           string
	APIKey             string
	BaseURL            string
	Model              string
	Timeout            time.Duration
	RateLimitPerMinute int
	RateLimitBurst     int
	MaxOutputTokens    int
}

type AdminConfig struct {
	Emails []string
}

// Load загружает конфигурацию приложения из окружения и .env.
func Load() (Config, error) {
	cfg := Config{}

	if err := loadEnv(); err != nil {
		return cfg, err
	}

	cfg.Env = getEnv("APP_ENV", "local")

	serverPort, err := parseIntEnv("SERVER_PORT", 8080)
	if err != nil {
		return cfg, err
	}

	readTimeout, err := parseDurationEnv("SERVER_READ_TIMEOUT", 5*time.Second)
	if err != nil {
		return cfg, err
	}

	writeTimeout, err := parseDurationEnv("SERVER_WRITE_TIMEOUT", 10*time.Second)
	if err != nil {
		return cfg, err
	}

	idleTimeout, err := parseDurationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return cfg, err
	}

	cfg.Server = ServerConfig{
		Host:         getEnv("SERVER_HOST", "0.0.0.0"),
		Port:         serverPort,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	dbPort, err := parseIntEnv("DB_PORT", 5432)
	if err != nil {
		return cfg, err
	}

	maxOpenConns, err := parseIntEnv("DB_MAX_OPEN_CONNS", 10)
	if err != nil {
		return cfg, err
	}

	maxIdleConns, err := parseIntEnv("DB_MAX_IDLE_CONNS", 5)
	if err != nil {
		return cfg, err
	}

	connMaxIdleTime, err := parseDurationEnv("DB_CONN_MAX_IDLE_TIME", 5*time.Minute)
	if err != nil {
		return cfg, err
	}

	connMaxLifetime, err := parseDurationEnv("DB_CONN_MAX_LIFETIME", 30*time.Minute)
	if err != nil {
		return cfg, err
	}

	cfg.Database = DatabaseConfig{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            dbPort,
		User:            getEnv("DB_USER", "budget"),
		Password:        getEnv("DB_PASSWORD", "budget"),
		Name:            getEnv("DB_NAME", "budget_planner"),
		SSLMode:         getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxIdleTime: connMaxIdleTime,
		ConnMaxLifetime: connMaxLifetime,
	}

	accessTTL, err := parseDurationEnv("JWT_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return cfg, err
	}

	refreshTTL, err := parseDurationEnv("JWT_REFRESH_TTL", 7*24*time.Hour)
	if err != nil {
		return cfg, err
	}

	rateLimitPerMinute, err := parseIntEnv("AUTH_RATE_LIMIT_PER_MINUTE", 60)
	if err != nil {
		return cfg, err
	}

	rateLimitBurst, err := parseIntEnv("AUTH_RATE_LIMIT_BURST", 10)
	if err != nil {
		return cfg, err
	}

	cfg.Auth = AuthConfig{
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTIssuer:          getEnv("JWT_ISSUER", "budget-planner"),
		AccessTokenTTL:     accessTTL,
		RefreshTokenTTL:    refreshTTL,
		RateLimitPerMinute: rateLimitPerMinute,
		RateLimitBurst:     rateLimitBurst,
	}

	aiTimeout, err := parseDurationEnv("AI_TIMEOUT", 20*time.Second)
	if err != nil {
		return cfg, err
	}

	aiRateLimitPerMinute, err := parseIntEnv("AI_RATE_LIMIT_PER_MINUTE", 30)
	if err != nil {
		return cfg, err
	}

	aiRateLimitBurst, err := parseIntEnv("AI_RATE_LIMIT_BURST", 10)
	if err != nil {
		return cfg, err
	}

	aiMaxOutputTokens, err := parseIntEnv("AI_MAX_OUTPUT_TOKENS", 4096)
	if err != nil {
		return cfg, err
	}

	aiProvider := strings.ToLower(getEnv("AI_PROVIDER", "gemini"))
	defaultBaseURL := "https://api.groq.com/openai/v1"
	defaultModel := "llama-3.1-8b-instant"
	if aiProvider == "gemini" {
		defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
		defaultModel = "gemini-1.5-flash"
	}

	aiAPIKey := getEnv("AI_API_KEY", "")
	if aiAPIKey == "" && aiProvider == "gemini" {
		aiAPIKey = getEnv("GEMINI_API_KEY", "")
	}

	cfg.AI = AIConfig{
		Provider:           aiProvider,
		APIKey:             aiAPIKey,
		BaseURL:            getEnv("AI_BASE_URL", defaultBaseURL),
		Model:              getEnv("AI_MODEL", defaultModel),
		Timeout:            aiTimeout,
		RateLimitPerMinute: aiRateLimitPerMinute,
		RateLimitBurst:     aiRateLimitBurst,
		MaxOutputTokens:    aiMaxOutputTokens,
	}

	cfg.Admin = AdminConfig{
		Emails: parseCSVEnv("ADMIN_EMAILS"),
	}

	if err := cfg.validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// DSN возвращает строку подключения к базе данных.
func (c DatabaseConfig) DSN() string {
	user := url.UserPassword(c.User, c.Password)
	dsn := url.URL{
		Scheme: "postgres",
		User:   user,
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.Name,
	}

	query := url.Values{}
	query.Set("sslmode", c.SSLMode)
	return dsn.String() + "?" + query.Encode()
}

func (c Config) validate() error {
	if c.Server.Port <= 0 {
		return fmt.Errorf("SERVER_PORT must be greater than 0")
	}

	if c.Database.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}

	if c.Database.User == "" {
		return fmt.Errorf("DB_USER is required")
	}

	if c.Database.Name == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("DB_MAX_IDLE_CONNS cannot exceed DB_MAX_OPEN_CONNS")
	}

	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	if c.Auth.AccessTokenTTL <= 0 {
		return fmt.Errorf("JWT_ACCESS_TTL must be greater than 0")
	}

	if c.Auth.RefreshTokenTTL <= 0 {
		return fmt.Errorf("JWT_REFRESH_TTL must be greater than 0")
	}

	if c.Auth.RateLimitPerMinute <= 0 {
		return fmt.Errorf("AUTH_RATE_LIMIT_PER_MINUTE must be greater than 0")
	}

	if c.Auth.RateLimitBurst <= 0 {
		return fmt.Errorf("AUTH_RATE_LIMIT_BURST must be greater than 0")
	}

	if c.AI.RateLimitPerMinute <= 0 {
		return fmt.Errorf("AI_RATE_LIMIT_PER_MINUTE must be greater than 0")
	}

	if c.AI.RateLimitBurst <= 0 {
		return fmt.Errorf("AI_RATE_LIMIT_BURST must be greater than 0")
	}

	if c.AI.MaxOutputTokens <= 0 {
		return fmt.Errorf("AI_MAX_OUTPUT_TOKENS must be greater than 0")
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func parseIntEnv(key string, fallback int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", key)
	}

	return parsed, nil
}

func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", key)
	}

	return parsed, nil
}

func parseCSVEnv(key string) []string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(part))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func loadEnv() error {
	if envFile := os.Getenv("ENV_FILE"); envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			return fmt.Errorf("load env file %s: %w", envFile, err)
		}
		return nil
	}

	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}

	return nil
}
