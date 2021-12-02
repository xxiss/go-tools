package cache

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"
)

func NewFile(fp string) *File {
	c := &File{
		Memory:     NewMemory(),
		fp:         fp,
		saveTicker: time.NewTicker(time.Second * 30),
		saveStop:   make(chan bool),
	}
	dir := path.Dir(c.fp)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0777)
	}
	if err := c.load(); err != nil {
		log.Println("file cache load:", err)
	}
	go c.saveLoop()
	return c
}

type File struct {
	*Memory
	fp         string
	saveTicker *time.Ticker
	saveStop   chan bool
}

func (c *File) saveLoop() {
	for {
		select {
		case <-c.saveTicker.C:
			if err := c.save(); err != nil {
				log.Println("file cache save:", err)
			}
		case <-c.saveStop:
			c.saveTicker.Stop()
			return
		}
	}
}

func (c *File) StopSave() {
	c.saveStop <- true
}

func (c *File) save() error {
	f, err := os.Create(c.fp)
	if err != nil {
		return err
	}
	defer f.Close()

	return c.gobsave(f)
}

func (c *File) load() (err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("error registering item types with gob library")
		}
	}()

	f, err := os.Open(c.fp)
	if err != nil {
		return err
	}
	defer f.Close()

	err = c.gobload(f)
	return
}

func (c *File) gobsave(w io.Writer) (err error) {
	enc := gob.NewEncoder(w)
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("error registering item types with gob library")
		}
	}()
	c.mu.RLock()
	defer c.mu.RUnlock()
	// gob.Register(c.storage)
	err = enc.Encode(&c.Storage)
	return
}

func (c *File) gobload(r io.Reader) error {
	now := time.Now().UnixNano()
	dec := gob.NewDecoder(r)
	storage := map[string]memoryItem{}
	if err := dec.Decode(&storage); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range storage {
		ov, found := c.Storage[k]
		if !found || ov.Expired(now) {
			c.Storage[k] = v
		}
	}
	return nil
}
