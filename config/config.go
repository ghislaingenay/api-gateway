package config

import (
	"os"
)

const (
 EnvDevelopment = "development"
 EnvProduction  = "production"
)

var  Env =  os.Getenv("APP_ENV")

func IsDevelopmentMode() bool {
	res := os.Getenv("APP_ENV")
	return res == EnvDevelopment
}

// Config holds all environment variables
type Config struct {
	AppEnv              string
	DBHost              string
	DBDatabase          string
	DBUser              string
	DBPort              string
	DBPassword          string
	DBSchema						string
	DBSSLMode           string
	DBPGChannelBinding  string
	RedisURL            string
	RedisToken          string
}

// Load reads from the environment and returns a Config struct.
// Call this from main() AFTER loading your .env file.
func Load() *Config {
	return &Config{
		AppEnv:             os.Getenv("APP_ENV"),
		DBHost:             os.Getenv("DB_HOST"),
		DBPort:             os.Getenv("DB_PORT"),
		DBDatabase:         os.Getenv("DB_DATABASE"),
		DBUser:             os.Getenv("DB_USER"),
		DBPassword:         os.Getenv("DB_PASSWORD"),
		DBSSLMode:          os.Getenv("DB_SSL_MODE"),
		DBPGChannelBinding: os.Getenv("DB_PG_CHANNELBINDING"),
		DBSchema:           os.Getenv("DB_SCHEMA"),
		RedisURL:           os.Getenv("REDIS_URL"),
		RedisToken:         os.Getenv("REDIS_TOKEN"),
	}
}