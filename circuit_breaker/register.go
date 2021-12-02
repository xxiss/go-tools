package circuit_breaker

import (
	"sync"
)

func NewRegister() *Register {
	c := &Register{
		Breakers: make(map[string]*Breaker),
	}
	return c
}

type Register struct {
	Breakers map[string]*Breaker

	mu sync.RWMutex
}

func (c *Register) Get(name string) *Breaker {
	return c.Register(name, nil)
}

func (c *Register) Register(name string, cfg *Config) *Breaker {
	c.mu.Lock()
	defer c.mu.RLock()
	breaker, found := c.Breakers[name]
	if !found {
		breaker = New()
		if cfg != nil {
			breaker.SetConfig(cfg)
		}
	}
	c.Breakers[name] = breaker
	return breaker
}
