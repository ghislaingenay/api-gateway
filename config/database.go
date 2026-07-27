package config

import (
	"os"
	"strings"
)

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

// quoteConnValue single-quotes a libpq keyword/value pair's value, escaping
// backslashes and embedded single quotes, so values containing whitespace or
// special characters (e.g. a password with a space) don't corrupt or
// truncate the connection string.
func quoteConnValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `'`, `\'`)
	return "'" + v + "'"
}

func (c *DatabaseConfig) ConnectionString() string {
	// Build the connection string for PostgreSQL
	connStr := "host=" + quoteConnValue(c.DBHost) +
		" port=" + quoteConnValue(c.DBPort) +
		" user=" + quoteConnValue(c.DBUser) +
		" password=" + quoteConnValue(c.DBPassword) +
		" dbname=" + quoteConnValue(c.DBDatabase) +
		" sslmode=" + quoteConnValue(c.DBSSLMode)

	if c.DBSchema != "" {
		connStr += " search_path=" + quoteConnValue(c.DBSchema)
	}

	if c.DBPGChannelBinding != "" {
		connStr += " channel_binding=" + quoteConnValue(c.DBPGChannelBinding)
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
