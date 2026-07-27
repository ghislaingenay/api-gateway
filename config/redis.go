package config

import "os"

type RedisConfig struct {
	RedisURL string
}

func LoadRedisConfig() *RedisConfig {
	return &RedisConfig{
		RedisURL: os.Getenv("REDIS_URL"),
	}
}
