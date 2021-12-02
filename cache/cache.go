package cache

import (
	"time"
)

type Cache interface {
	LockRun(key string, d time.Duration, fn func() error) error
	Get(key string, result interface{}) error
	Set(key string, d time.Duration, create func() (interface{}, error)) error
	GetOrSet(key string, result interface{}, d time.Duration, create func() (interface{}, error)) error
	Remove(key string)
}
