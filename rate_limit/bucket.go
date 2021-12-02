package rate_limit

import (
	"fmt"
	"time"
)

func New() *Bucket {
	return &Bucket{Config: &Config{
		Create:   1,
		Consume:  1,
		Capacity: 2,
		Timeout:  time.Second,
		Interval: time.Millisecond * 10,
	}}
}

type Config struct {
	Create    uint
	Consume   uint
	Capacity  uint
	Timeout   time.Duration
	Interval  time.Duration
	Callbacks []func(key string, bk Bucket)
}

type Bucket struct {
	Config *Config

	Amount uint
}

func (c *Bucket) SetConfig(cfg *Config) *Bucket {
	c.Config = cfg
	return c
}

func (c *Bucket) Run(key string, cfg *Config, fn func() (interface{}, error)) (interface{}, error) {
	var d time.Duration
	for {
		if c.Config.Consume <= c.Amount {
			break
		}
		if d >= c.Config.Timeout {
			if len(c.Config.Callbacks) > 0 {
				bk := *c
				go func() {
					for _, callback := range c.Config.Callbacks {
						callback(key, bk)
					}
				}()
			}
			return nil, fmt.Errorf("the system is busy. please try again later. key:%s", key)
		}

		d += c.Config.Interval
		time.Sleep(c.Config.Interval)
	}

	c.Amount -= c.Config.Consume
	return fn()
}
