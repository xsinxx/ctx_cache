package ctx_cache

import (
	"context"
	"sync"
)

type loadFunc func(context.Context) (interface{}, error)
type callCacheKeyT string

type cacheItem struct {
	ret  interface{}
	err  error
	once sync.Once
}

type callCache struct {
	m    map[string]*cacheItem
	lock sync.RWMutex
}

func newCacheItem() *cacheItem {
	return &cacheItem{
		once: sync.Once{},
	}
}
