package cache

import (
	"time"
)

type Cache interface {
	LockRun(key string, d time.Duration, fn func() error) error
	Get(key string, result interface{}) error
	Set(key string, create func() (*Item, error)) error
	GetOrSet(key string, result interface{}, create func() (*Item, error)) error
	Remove(key string)
}

type Item struct {
	Value    interface{}
	Duration time.Duration
}
