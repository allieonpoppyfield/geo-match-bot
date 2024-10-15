package cache

import (
	"geo_match_bot/internal/config"
	"log"

	"github.com/bradfitz/gomemcache/memcache"
)

type MemcacheClient struct {
	Client *memcache.Client
}

func NewMemcacheClient(cfg *config.Config) (*MemcacheClient, error) {
	client := memcache.New(cfg.MemcacheHost + ":" + cfg.MemcachePort) // Используем RedisHost и RedisPort для Memcached

	// Пробный запрос для проверки подключения
	err := client.Set(&memcache.Item{
		Key:   "ping",
		Value: []byte("pong"),
	})

	if err != nil {
		return nil, err
	}

	log.Println("Successfully connected to Memcached")

	return &MemcacheClient{Client: client}, nil
}

func (c *MemcacheClient) Get(key string) (string, error) {
	item, err := c.Client.Get(key)
	if err != nil {
		return "", err
	}
	return string(item.Value), nil
}

func (c *MemcacheClient) Set(key string, value string) error {
	err := c.Client.Set(&memcache.Item{
		Key:   key,
		Value: []byte(value),
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *MemcacheClient) Delete(key string) error {
	err := c.Client.Delete(key)
	if err != nil {
		return err
	}
	return nil
}
