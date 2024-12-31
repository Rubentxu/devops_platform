package store

import (
	"dev.rubentxu.devops-platform/core/ports"
	"fmt"
	"log"
)

type StoreType string

const (
	MemoryStore StoreType = "memory"
	RedisStore  StoreType = "redis"
)

func NewStore(taskDbType string, name string) (ports.Store, error) {
	var s ports.Store
	var err error
	switch taskDbType {
	case "memory":
		s = NewInMemoryTaskStore()
	case "persistent":
		filename := fmt.Sprintf("%s_tasks.db", name)
		s, err = NewTaskStore(filename, 0600, "tasks")
	}
	if err != nil {
		log.Printf("Unable to create new task store: %v", err)
	}
	return s, nil
}
