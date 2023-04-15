package ctx_cache

import (
	"context"
)

const callCacheKey callCacheKeyT = "_zh_call_cache"

func (ci *cacheItem) doOnce(ctx context.Context, loader loadFunc) {
	ci.once.Do(func() {
		// sync.Once guarantees that only one routine will execute this, the others will wait till return
		ci.ret, ci.err = loader(ctx)
	})
}

// getOrCreateCacheItem 从callCache中获取指定key的cacheItem(不存在则创建一个)。保证并发安全
// 不会返回nil
func (cache *callCache) getOrCreateCacheItem(key string) *cacheItem {
	cache.lock.RLock()
	cr, ok := cache.m[key]
	cache.lock.RUnlock()
	if ok {
		return cr
	}

	cache.lock.Lock()
	defer cache.lock.Unlock()
	if cache.m == nil {
		cache.m = make(map[string]*cacheItem)
	} else {
		// 其他协程可能已经在m中将cacheItem写入
		cr, ok = cache.m[key]
	}
	if !ok {
		cr = newCacheItem()
		cache.m[key] = cr
	}
	return cr
}

// WithCallCache 用于构建context中的map
func WithCallCache(parent context.Context) context.Context {
	if parent.Value(callCacheKey) != nil {
		return parent
	}
	return context.WithValue(parent, callCacheKey, new(callCache))
}

// getOrCreateCacheItem 未启用cache才会返回nil
func getOrCreateCacheItem(ctx context.Context, key string) *cacheItem {
	if v := ctx.Value(callCacheKey); v != nil {
		return v.(*callCache).getOrCreateCacheItem(key)
	}
	return nil
}

// LoadFromCtxCache 从ctx中尝试获取key的缓存结果
// 如果不存在，调用loader;如果没有开启缓存，直接调用loader
func LoadFromCtxCache(ctx context.Context, key string, loader loadFunc) (interface{}, error) {
	cacheItem := getOrCreateCacheItem(ctx, key)
	if cacheItem == nil { // cache not enabled
		return loader(ctx)
	}

	// now that all routines hold references to the same cacheItem
	cacheItem.doOnce(ctx, loader)
	return cacheItem.ret, cacheItem.err
}
