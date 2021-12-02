package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type redisCache struct {
	*redis.Client
	mu  sync.RWMutex
	mus map[string]*sync.RWMutex
}

func NewRedis(host string, port int, password string, db int) (*redisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	})
	if _, err := client.Ping(context.TODO()).Result(); err != nil {
		return nil, err
	}
	return &redisCache{
		Client: client,
		mus:    make(map[string]*sync.RWMutex),
	}, nil
	// defer client.Close()
}

func (c *redisCache) gcRWMutex(key string) *sync.RWMutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mus[key] == nil {
		c.mus[key] = &sync.RWMutex{}
	}
	return c.mus[key]
}

func (c *redisCache) LockRun(id string, timeout time.Duration, fn func() error) error {
	if _, err := c.Client.SetNX(context.TODO(), id, "ok", timeout).Result(); err != nil {
		return fmt.Errorf("the system is busy. please try again later. id:%s", id)
	}
	defer c.Client.Del(context.TODO(), id)
	return fn()
}

func (c *redisCache) Get(key string, result interface{}) error {
	rel, err := c.Client.Get(context.TODO(), key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(rel), result)
}

func (c *redisCache) Set(key string, d time.Duration, create func() (interface{}, error)) error {
	mu := c.gcRWMutex(key)
	mu.Lock()
	defer mu.Unlock()

	value, err := create()
	if err != nil {
		return err
	}
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = c.Client.Set(context.TODO(), key, body, d).Result()
	return err
}

func (c *redisCache) GetOrSet(key string, result interface{}, d time.Duration, create func() (interface{}, error)) error {
	mu := c.gcRWMutex(key)
	mu.Lock()
	defer mu.Unlock()

	rel, err := c.Client.Get(context.TODO(), key).Result()
	if err == nil {
		return json.Unmarshal([]byte(rel), result)
	}

	value, err := create()
	if err != nil {
		return err
	}
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := c.Client.Set(context.TODO(), key, body, d).Result(); err != nil {
		return err
	}

	return json.Unmarshal(body, result)
}

func (c *redisCache) Remove(key string) {
	c.Client.Del(context.TODO(), key)
}
