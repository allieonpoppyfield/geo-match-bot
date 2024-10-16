package cache

import (
	"fmt"
	"strconv"

	"github.com/go-redis/redis/v8"
	"golang.org/x/net/context"
)

type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisClient создает новое подключение к Redis
func NewRedisClient(host, port string) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port),
	})

	return &RedisClient{
		client: rdb,
		ctx:    context.Background(),
	}
}

// AddUserLocation добавляет локацию пользователя в Redis (с использованием гео-функции)
func (r *RedisClient) AddUserLocation(userID int64, latitude, longitude float64) error {
	_, err := r.client.GeoAdd(r.ctx, "user_locations", &redis.GeoLocation{
		Longitude: longitude,
		Latitude:  latitude,
		Name:      strconv.FormatInt(userID, 10),
	}).Result()

	return err
}

// FindNearbyUsers ищет пользователей в радиусе вокруг заданных координат
func (r *RedisClient) FindNearbyUsers(latitude, longitude float64, radius float64) ([]string, error) {
	locations, err := r.client.GeoRadius(r.ctx, "user_locations", longitude, latitude, &redis.GeoRadiusQuery{
		Radius:      radius,
		Unit:        "km",
		WithCoord:   false,
		WithDist:    false,
		WithGeoHash: false,
	}).Result()

	if err != nil {
		return nil, err
	}

	// Извлекаем имена (userID) из результата
	var nearbyUsers []string
	for _, location := range locations {
		nearbyUsers = append(nearbyUsers, location.Name)
	}

	return nearbyUsers, nil
}