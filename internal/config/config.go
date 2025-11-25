package config

import (
	"log"
	"os"
	"strconv"
)

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Addr        string
	Password    string
	DB          int
	MaxRetries  int
	DialTimeout int
	Timeout     int
	Prefix      string
}

type AppConfig struct {
	Port     string
	Postgres PostgresConfig
	Redis    RedisConfig
	// Local export storage directory (where generated files will be written)
	ExportDir string
	// Public URL prefix where files will be served (e.g. /files)
	FilesPublicPrefix string
	// ExternalURL — optional absolute URL used when generating file urls (e.g. https://example.com:8060)
	ExternalURL  string
	ExportPrefix string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustAtoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("invalid int value %q: %v", s, err)
	}
	return i
}

func mustBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		log.Fatalf("invalid bool value %q: %v", s, err)
	}
	return b
}

func Load() AppConfig {
	return AppConfig{
		Port: getenv("APP_PORT", "8010"),
		Postgres: PostgresConfig{
			Host:     getenv("PG_HOST", "127.0.0.1"),
			Port:     mustAtoi(getenv("PG_PORT", "5432")),
			User:     getenv("PG_USER", "root"),
			Password: getenv("PG_PASSWORD", "hello-world"),
			DBName:   getenv("PG_DB", "debtster"),
			SSLMode:  getenv("PG_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:        getenv("REDIS_ADDR", "127.0.0.1:6379"),
			Password:    getenv("REDIS_PASSWORD", "hello-world"),
			DB:          mustAtoi(getenv("REDIS_DB", "0")),
			MaxRetries:  mustAtoi(getenv("REDIS_MAX_RETRIES", "5")),
			DialTimeout: mustAtoi(getenv("REDIS_DIAL_TIMEOUT", "10")),
			Timeout:     mustAtoi(getenv("REDIS_TIMEOUT", "5")),
			Prefix:      getenv("REDIS_PREFIX", "debtster_database"),
		},
		ExportDir:         getenv("EXPORT_DIR", "./exports"),
		FilesPublicPrefix: getenv("EXPORT_PUBLIC_PREFIX", "/files"),
		ExternalURL:       getenv("EXTERNAL_URL", ""),
		ExportPrefix:      getenv("EXPORT_CACHE_PREFIX", "pkb_database_cache"),
	}
}
