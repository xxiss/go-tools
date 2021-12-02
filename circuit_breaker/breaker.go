package circuit_breaker

import (
	"math/rand"
	"time"
)

func New() *Breaker {
	return &Breaker{
		Config: &Config{
			Duration:       time.Second * 10,
			FailCounter:    10,
			SuccessCounter: 10,
			Failback: func() (interface{}, error) {
				return nil, nil
			},
			Handler: new(TimeoutHandler).SetTimeout(time.Second * 5),
		},
	}
}

type State string

const (
	StateClose    State = "CLOSE"
	StateHalfOpen State = "HALF_OPEN"
	StateOpen     State = "OPEN"
)

type Config struct {
	Duration       time.Duration
	FailCounter    uint
	SuccessCounter uint
	Failback       func() (interface{}, error)
	Handler        Handler
}

type Breaker struct {
	*Config
	FailCounter    uint
	SuccessCounter uint
	State          State
	Timestamp      int64
}

func (c *Breaker) init() {
	c.Timestamp = time.Now().UnixNano()
	c.FailCounter = 0
	c.SuccessCounter = 0
}

func (c *Breaker) duration() time.Duration {
	return time.Duration(time.Now().UnixNano() - c.Timestamp)
}

func (c *Breaker) aswitch() {
	switch c.State {
	case StateOpen:
		if c.duration() >= c.Config.Duration {
			c.init()
			c.State = StateHalfOpen
		}
	case StateClose:
		if c.duration() >= c.Config.Duration {
			c.init()
			return
		}
		fallthrough
	case StateHalfOpen:
		if c.FailCounter >= c.Config.FailCounter {
			c.init()
			c.State = StateOpen
		}
		if c.SuccessCounter >= c.Config.SuccessCounter {
			c.init()
			c.State = StateClose
		}
	}
}

func (c *Breaker) call(fn func() (interface{}, error)) (interface{}, error) {
	ok, result, err := c.Config.Handler.Handle(c, fn)
	if ok {
		c.SuccessCounter++
	} else {
		c.FailCounter++
	}
	c.aswitch()
	return result, err
}

func (c *Breaker) attemptCall(fn func() (interface{}, error)) (interface{}, error) {
	if rand.New(rand.NewSource(time.Now().UnixNano())).Intn(10) >= 5 {
		return c.call(fn)
	}
	return c.Config.Failback()
}

func (c *Breaker) SetConfig(cfg *Config) *Breaker {
	c.Config = cfg
	return c
}

func (c *Breaker) Run(fn func() (interface{}, error)) (interface{}, error) {
	c.aswitch()
	if c.State == StateOpen {
		return c.Failback()
	}
	if c.State == StateHalfOpen {
		return c.attemptCall(fn)
	}
	return c.call(fn)
}
