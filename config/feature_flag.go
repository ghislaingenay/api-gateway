package config

import "os"

type FeatureFlag struct {
	AllowAutoDBMigration bool
}

func LoadFeatureFlags() *FeatureFlag {
	return &FeatureFlag{
		AllowAutoDBMigration: os.Getenv("ALLOW_AUTO_DB_MIGRATION") == "true",
	}
}

