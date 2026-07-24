package config

import (
	"os"
)

type AppConfig struct {
	AppEnv string
}

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

func validateAppEnv(env string) bool {
	return env == EnvDevelopment || env == EnvProduction
}

func (c *AppConfig) IsDevelopmentMode() bool {
	return c.AppEnv == EnvDevelopment
}

func LoadAppConfig() *AppConfig {
	return &AppConfig{
		AppEnv: os.Getenv("APP_ENV"),
	}
}
