package cache

import "errors"

var (
	ErrParseRedisURL = errors.New("failed to parse redis url")
	ErrConnectRedis  = errors.New("failed to connect to redis")
	ErrCacheGet      = errors.New("cache: get failed")
	ErrCacheSet      = errors.New("cache: set failed")
	ErrCacheMarshal  = errors.New("cache: marshal response failed")
)
