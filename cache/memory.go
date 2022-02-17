package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

func NewMemory() *Memory {
	c := &Memory{
		Storage:  make(map[string]memoryItem),
		nx:       make(map[string]int64),
		mus:      make(map[string]*sync.RWMutex),
		gcTicker: time.NewTicker(time.Minute * 10),
		gcStop:   make(chan bool),
	}
	go c.gcLoop()
	return c
}

type memoryItem struct {
	Body       []byte
	Expiration int64
}

func (c memoryItem) Expired(unixNano int64) bool {
	if c.Expiration == 0 {
		return false
	}
	return unixNano > c.Expiration
}

type Memory struct {
	Storage map[string]memoryItem

	mu       sync.RWMutex
	mus      map[string]*sync.RWMutex
	nx       map[string]int64
	gcTicker *time.Ticker
	gcStop   chan bool
}

func (c *Memory) gcLoop() {
	for {
		select {
		case <-c.gcTicker.C:
			c.ClearExpired()
		case <-c.gcStop:
			c.gcTicker.Stop()
			return
		}
	}
}

func (c *Memory) gcRWMutex(key string) *sync.RWMutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mus[key] == nil {
		c.mus[key] = &sync.RWMutex{}
	}
	return c.mus[key]
}

func (c *Memory) ResetGC(d time.Duration) {
	c.gcTicker.Reset(d)
}

func (c *Memory) StopGC() {
	c.gcStop <- true
}

func (c *Memory) ClearExpired() {
	now := time.Now().UnixNano()
	for k, v := range c.Storage {
		if v.Expired(now) {
			c.Remove(k)
		}
	}
}

func (c *Memory) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Storage = make(map[string]memoryItem)
}

func (c *Memory) LockRun(key string, d time.Duration, fn func() error) error {
	c.mu.Lock()
	now := time.Now().UnixNano()
	if c.nx[key] != 0 && c.nx[key]+int64(d) > now {
		c.mu.Unlock()
		return fmt.Errorf("the system is busy. please try again later. key:%s", key)
	}
	c.nx[key] = now
	c.mu.Unlock()
	defer func() {
		if c.nx[key] == now {
			delete(c.nx, key)
		}
	}()
	return fn()
}

func (c *Memory) Get(key string, result interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.Storage[key]
	if !found || entry.Expired(time.Now().UnixNano()) {
		return fmt.Errorf("%s doesn't exist", key)
	}
	return json.Unmarshal(entry.Body, &result)
}

func (c *Memory) Set(key string, create func() (*Item, error)) error {
	mu := c.gcRWMutex(key)
	mu.Lock()
	defer mu.Unlock()

	item, err := create()
	if err != nil {
		return err
	}
	body, err := json.Marshal(item.Value)
	if err != nil {
		return err
	}
	mem := memoryItem{Body: body}
	if item.Duration != 0 {
		mem.Expiration = time.Now().Add(item.Duration).UnixNano()
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.Storage[key] = mem
	return nil
}

func (c *Memory) GetOrSet(key string, result interface{}, create func() (*Item, error)) error {
	mu := c.gcRWMutex(key)
	mu.Lock()
	defer mu.Unlock()

	entry, found := c.Storage[key]
	if found && !entry.Expired(time.Now().UnixNano()) {
		return json.Unmarshal(entry.Body, &result)
	}

	item, err := create()
	if err != nil {
		return err
	}
	body, err := json.Marshal(item.Value)
	if err != nil {
		return err
	}
	mem := memoryItem{Body: body}
	if item.Duration != 0 {
		mem.Expiration = time.Now().Add(item.Duration).UnixNano()
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.Storage[key] = mem

	return json.Unmarshal(body, &result)
}

func (c *Memory) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Storage, key)
}
