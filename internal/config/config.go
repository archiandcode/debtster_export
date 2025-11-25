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

type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
	Region          string
	Prefix          string
}

type AppConfig struct {
	Port         string
	Postgres     PostgresConfig
	Redis        RedisConfig
	S3           S3Config
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
		S3: S3Config{
			Endpoint:        getenv("S3_ENDPOINT", "localhost:9000"),
			AccessKeyID:     getenv("S3_ACCESS_KEY", "minio"),
			SecretAccessKey: getenv("S3_SECRET_KEY", "minio123"),
			Bucket:          getenv("S3_BUCKET", "exports"),
			Region:          getenv("S3_REGION", "us-east-1"),
			UseSSL:          mustBool(getenv("S3_USE_SSL", "false")),
			Prefix:          getenv("S3_PREFIX", ""),
		},
		ExportPrefix: getenv("EXPORT_CACHE_PREFIX", "pkb_database_cache"),
	}
}
