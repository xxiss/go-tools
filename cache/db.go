package cache

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var tableName = "table_" + strconv.FormatInt(time.Now().UnixNano(), 10)

func NewDB(db *gorm.DB, tableName string) *DB {
	c := &DB{
		Memory:     NewMemory(),
		loadTicker: time.NewTicker(time.Minute * 30),
		loadStop:   make(chan bool),
		db:         db,
		tableName:  tableName,
	}
	c.initTable()
	if err := c.load(); err != nil {
		log.Println("db cache load:", err)
	}
	go c.loadLoop()
	return c
}

type dbItem struct {
	Key        string           `json:"key" gorm:"type:varchar(255);column:key;primaryKey;not null;comment:key"`
	Value      *json.RawMessage `json:"value" gorm:"type:json;column:value;comment:value"`
	Expiration int64            `json:"expiration" gorm:"type:bigint unsigned;column:expiration;not null;default:0;comment:expiration timestamp"`
	CreateTime time.Time        `json:"createTime" validate:"required" gorm:"type:datetime;column:create_time;autoCreateTime;not null;default:CURRENT_TIMESTAMP;comment:create time"`
	UpdateTime time.Time        `json:"updateTime" validate:"required" gorm:"type:datetime;column:update_time;autoUpdateTime;not null;default:CURRENT_TIMESTAMP;comment:update time"`
}

func (m *dbItem) TableName() string {
	return tableName
}

type DB struct {
	*Memory

	loadTicker *time.Ticker
	loadStop   chan bool
	db         *gorm.DB
	tableName  string
}

func (c *DB) initTable() {
	if !c.db.Migrator().HasTable(c.tableName) {
		c.db.Set("gorm:table_options", "ENGINE=InnoDB").Migrator().CreateTable(&dbItem{})
		c.db.Migrator().RenameTable(tableName, c.tableName)
	}
}

func (c *DB) loadLoop() {
	for {
		select {
		case <-c.loadTicker.C:
			if err := c.load(); err != nil {
				log.Println("db cache load:", err)
			}
		case <-c.loadStop:
			c.loadTicker.Stop()
			return
		}
	}
}

func (c *DB) load() (err error) {
	if err := c.db.Table(c.tableName).Where("`expiration` != 0 AND `expiration` < ?", time.Now().UnixNano()).Delete(dbItem{}).Error; err != nil {
		log.Println(err)
	}
	var rows []dbItem
	if err := c.db.Table(c.tableName).
		Where("`value` IS NOT NULL AND `expiration` = 0 OR `expiration` >= ?", time.Now().UnixNano()).
		Find(&rows).Error; err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.Storage = make(map[string]memoryItem)

	for _, row := range rows {
		c.Storage[row.Key] = memoryItem{Body: *row.Value, Expiration: row.Expiration}
	}
	return nil
}

func (c *DB) save(key string, mem memoryItem) error {
	body := json.RawMessage(mem.Body)
	row := dbItem{
		Key:        key,
		Value:      &body,
		Expiration: mem.Expiration,
	}
	if err := c.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "expiration"}),
	}).Table(c.tableName).Create(&row).Error; err != nil {
		return err
	}
	return nil
}

func (c *DB) delete(key string) error {
	return c.db.Table(c.tableName).Where("`key` = ? LIMIT 1", key).Delete(&dbItem{}).Error
}

func (c *DB) Set(key string, create func() (*Item, error)) error {
	if err := c.Memory.Set(key, create); err != nil {
		return err
	}

	if entry, found := c.Storage[key]; found {
		go c.save(key, entry)
	}
	return nil
}

func (c *DB) GetOrSet(key string, result interface{}, create func() (*Item, error)) error {
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

	go c.save(key, mem)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.Storage[key] = mem

	return json.Unmarshal(body, &result)
}

func (c *DB) Remove(key string) {
	c.Memory.Remove(key)
	go c.delete(key)
}
