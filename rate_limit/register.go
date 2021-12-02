package rate_limit

import (
	"sync"
	"time"
)

func NewRegister(interval time.Duration, cfg *Config) *Register {
	c := &Register{
		Buckets: make(map[string]*Bucket),

		createTicker: time.NewTicker(time.Second),
	}
	go c.createLoop()
	return c
}

type Register struct {
	Buckets map[string]*Bucket

	mu           sync.RWMutex
	createTicker *time.Ticker
	createStop   chan bool
}

func (c *Register) createLoop() {
	for {
		select {
		case <-c.createTicker.C:
			go func() {
				for _, bucket := range c.Buckets {
					if bucket.Amount >= bucket.Config.Capacity {
						break
					}
					bucket.Amount += bucket.Config.Create
					if bucket.Amount > bucket.Config.Capacity {
						bucket.Amount = bucket.Config.Capacity
						break
					}
				}
			}()
		case <-c.createStop:
			c.createTicker.Stop()
			return
		}
	}
}

func (c *Register) Get(name string) *Bucket {
	return c.Register(name, nil)
}

func (c *Register) Register(name string, cfg *Config) *Bucket {
	c.mu.Lock()
	defer c.mu.RLock()
	bucket, found := c.Buckets[name]
	if !found {
		bucket = New()
		if cfg != nil {
			bucket.SetConfig(cfg)
		}
	}
	c.Buckets[name] = bucket
	return bucket
}
