package config

import (
	"crypto/tls"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func NewRedisClient(config *viper.Viper, log *logrus.Logger) (*redis.Client, func()) {
	addr := fmt.Sprintf("%s:%d", config.GetString("REDIS_HOST"), config.GetInt("REDIS_PORT"))
	password := config.GetString("REDIS_PASSWORD")
	db := config.GetInt("REDIS_DB")
	enableTLS := config.GetBool("REDIS_TLS_ENABLED")

	opts := &redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	}

	if enableTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(opts)

	log.Info("Connected to Redis successfully")

	cleanup := func() {
		if err := client.Close(); err != nil {
			log.Errorf("failed to close redis client: %v", err)
		}
		log.Info("Redis connection closed")
	}

	return client, cleanup
}
