package config

import "os"

type DatabaseConfig struct {
	DBHost             string
	DBDatabase         string
	DBUser             string
	DBPort             string
	DBPassword         string
	DBSchema           string
	DBSSLMode          string
	DBPGChannelBinding string
}

const (
	DefaultDbPort             = 5432
	DefaultDbSSLMode          = "disable"
	DefaultDbPGChannelBinding = "prefer"
	DefaultDbSchema           = "public"
)

func (c *DatabaseConfig) ConnectionString() string {
	// Build the connection string for PostgreSQL
	connStr := "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBDatabase +
		" sslmode=" + c.DBSSLMode

	if c.DBSchema != "" {
		connStr += " search_path=" + c.DBSchema
	}

	if c.DBPGChannelBinding != "" {
		connStr += " channel_binding=" + c.DBPGChannelBinding
	}

	return connStr
}

func LoadDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		DBHost:             os.Getenv("DB_HOST"),
		DBPort:             os.Getenv("DB_PORT"),
		DBDatabase:         os.Getenv("DB_DATABASE"),
		DBUser:             os.Getenv("DB_USER"),
		DBPassword:         os.Getenv("DB_PASSWORD"),
		DBSSLMode:          os.Getenv("DB_SSL_MODE"),
		DBPGChannelBinding: os.Getenv("DB_PG_CHANNELBINDING"),
		DBSchema:           os.Getenv("DB_SCHEMA"),
	}
}
