package config

import (
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

const AuthPrefix = "auth:session:"

func NewRedisClient(config *viper.Viper) *redis.Client {
	addr := fmt.Sprintf("%s:%d", config.GetString("REDIS_HOST"), config.GetInt("REDIS_PORT"))
	password := config.GetString("REDIS_PASSWORD")
	db := config.GetInt("REDIS_DB")

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return client
}