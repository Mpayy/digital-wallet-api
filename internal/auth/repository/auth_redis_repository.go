package repository

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_auth_redis_repository.go
type AuthRedisRepository interface {
	SaveSession(ctx context.Context, token string, authData []byte, ttl time.Duration) error
	DeleteSession(ctx context.Context, token string) error
	SessionExists(ctx context.Context, token string) (bool, error)
}

type authRedisRepositoryImpl struct {
	client *redis.Client
}

func NewAuthRedisRepository(client *redis.Client) AuthRedisRepository {
	return &authRedisRepositoryImpl{client: client}
}

func (r *authRedisRepositoryImpl) SaveSession(ctx context.Context, token string, authData []byte, ttl time.Duration) error {
	err := r.client.Set(ctx, token, authData, ttl).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *authRedisRepositoryImpl) DeleteSession(ctx context.Context, token string) error {
	err := r.client.Del(ctx, token).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *authRedisRepositoryImpl) SessionExists(ctx context.Context, token string) (bool, error) {
	exists, err := r.client.Exists(ctx, token).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}
