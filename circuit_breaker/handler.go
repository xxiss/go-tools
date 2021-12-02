package circuit_breaker

import "time"

type Handler interface {
	Handle(*Breaker, func() (interface{}, error)) (bool, interface{}, error)
}

type TimeoutHandler struct {
	Timeout time.Duration
}

func (h *TimeoutHandler) SetTimeout(d time.Duration) *TimeoutHandler {
	h.Timeout = d
	return h
}

func (h *TimeoutHandler) Handle(c *Breaker, fn func() (interface{}, error)) (bool, interface{}, error) {
	if h.Timeout == 0 {
		h.Timeout = time.Second * 5
	}
	before := time.Now()
	result, err := fn()
	after := time.Now()
	return after.Before(before.Add(h.Timeout)), result, err
}
