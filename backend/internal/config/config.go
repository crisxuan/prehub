package config

import "os"

type Config struct {
	Port             string
	DatabaseURL      string
	RedisAddr        string
	GitHubToken      string
	GitHubAPIVersion string
	InternalAPIToken string
	TimeZone         string
	ClickHouseURL    string
	ClickHouseUser   string
	ClickHousePass   string
}

func Load() Config {
	return Config{
		Port:             env("PORT", "8080"),
		DatabaseURL:      env("DATABASE_URL", ""),
		RedisAddr:        env("REDIS_ADDR", "localhost:6379"),
		GitHubToken:      env("GITHUB_TOKEN", ""),
		GitHubAPIVersion: env("GITHUB_API_VERSION", "2026-03-10"),
		InternalAPIToken: env("INTERNAL_API_TOKEN", ""),
		TimeZone:         env("PREHUB_TIMEZONE", "Asia/Shanghai"),
		ClickHouseURL:    env("PREHUB_CLICKHOUSE_URL", "https://sql-clickhouse.clickhouse.com/"),
		ClickHouseUser:   env("PREHUB_CLICKHOUSE_USER", "demo"),
		ClickHousePass:   env("PREHUB_CLICKHOUSE_PASSWORD", ""),
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
