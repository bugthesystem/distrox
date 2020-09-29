package main

import (
	"github.com/spf13/viper"
)

type AppConfig struct {
	host string
	port int
	// debug or release
	mode         string
	pprofEnabled bool
}

type CacheConfig struct {
	shards       int
	maxBytes     int
	ttlInSeconds int64
	statsEnabled bool

	maxKeySizeInBytes   int64
	maxValueSizeInBytes int64
}

type Config struct {
	app   AppConfig
	cache CacheConfig
}

func loadConfig() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var c Config

	// app
	c.app.host = v.GetString("app.hostname")
	c.app.port = v.GetInt("app.port")
	c.app.mode = v.GetString("app.mode")
	c.app.pprofEnabled = v.GetBool("app.pprof_enabled")

	// cache
	c.cache.shards = v.GetInt("cache.shards")
	c.cache.maxBytes = v.GetInt("cache.max_bytes")
	c.cache.ttlInSeconds = v.GetInt64("cache.ttl_in_seconds")
	c.cache.statsEnabled = v.GetBool("cache.stats_enabled")

	c.cache.maxKeySizeInBytes = v.GetInt64("cache.max_key_size_in_bytes")
	c.cache.maxValueSizeInBytes = v.GetInt64("cache.max_value_size_in_bytes")

	return &c, nil
}
