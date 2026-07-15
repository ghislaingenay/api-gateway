package cache

import (
	"api-gateway/config"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient initializes and tests the Redis connection
func NewRedisClient(url string, token string) (*redis.Client, error) {
	// Parse the URL (e.g., "redis://user:password@localhost:6379/0")
	// If you aren't using a URL string, you can use redis.Options directly.
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	// Apply the token/password if provided separately
	if token != "" {
		opts.Password = token
	}

	client := redis.NewClient(opts)

	// Create a context with a timeout to prevent hanging forever on boot
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ping the database to ensure connection is valid
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return client, nil
}

func main() {
	// Assuming you loaded your config as discussed previously:
	cfg := config.Load()
	redisURL := cfg.RedisURL
	redisToken := cfg.RedisToken

	client, err := NewRedisClient(redisURL, redisToken)
	if err != nil {
		log.Fatalf("Redis initialization failed: %v", err)
	}
	defer client.Close()

	fmt.Println("Successfully connected to Redis!")

	// --- Example Usage ---
	ctx := context.Background()

	// Set a value
	err = client.Set(ctx, "my_key", "hello go-redis", 10*time.Minute).Err()
	if err != nil {
		log.Printf("Failed to set key: %v", err)
	}

	// Get a value
	val, err := client.Get(ctx, "my_key").Result()
	if err != nil {
		log.Printf("Failed to get key: %v", err)
	} else {
		fmt.Printf("my_key: %s\n", val)
	}
}