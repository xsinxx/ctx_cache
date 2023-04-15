package ctx_cache

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

const flagError = "error"

func randFunc(ctx context.Context, flag string) (int, error) {
	if flag == flagError {
		return 0, fmt.Errorf("%d", rand.Int())
	}
	return rand.Int(), nil
}

func RandFunc(ctx context.Context, flag string) (int, error) {
	key := "RandFunc:" + flag
	r, e := LoadFromCtxCache(ctx, key, func(ctx context.Context) (interface{}, error) {
		return randFunc(ctx, flag)
	})
	return r.(int), e
}

func TestLoadFromCtxCacheSuccess(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	var ret1, ret2 int
	var err1, err2 error
	const flag = ""

	// without cache, random return
	ret1, err1 = RandFunc(ctx, flag)
	assert.NoError(err1)
	ret2, err2 = RandFunc(ctx, flag)
	assert.NoError(err2)
	assert.NotEqual(ret1, ret2)

	// with cache, same return
	cacheCtx := WithCallCache(ctx)
	ret1, err1 = RandFunc(cacheCtx, flag)
	assert.NoError(err1)
	ret2, err2 = RandFunc(cacheCtx, flag)
	assert.NoError(err2)
	assert.Equal(ret1, ret2)
}

func TestLoadFromCtxCacheError(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	var err1, err2 error
	const flag = flagError

	// without cache, random error
	_, err1 = RandFunc(ctx, flag)
	assert.Error(err1)
	_, err2 = RandFunc(ctx, flag)
	assert.Error(err2)
	assert.NotEqual(err1.Error(), err2.Error())

	// with cache, same error
	cacheCtx := WithCallCache(ctx)
	_, err1 = RandFunc(cacheCtx, flag)
	assert.Error(err1)
	_, err2 = RandFunc(cacheCtx, flag)
	assert.Error(err2)
	assert.Equal(err1.Error(), err2.Error())
}

func withCallCacheOld(parent context.Context) context.Context {
	if v := parent.Value(callCacheKey); v != nil {
		return parent
	}
	return context.WithValue(parent, callCacheKey, new(sync.Map))
}

func loadFromCtxCacheOld(ctx context.Context, key string, loader loadFunc) (interface{}, error) {
	// get cache map of from ctx
	cache := ctx.Value(callCacheKey)
	if cache == nil {
		// cache not enabled, perform call without cache
		return loader(ctx)
	}

	sm := cache.(*sync.Map)
	v, exist := sm.Load(key)
	if !exist {
		v, _ = sm.LoadOrStore(key, newCacheItem())
	}

	// now that all routines hold references to the same cacheResult struct
	loadedResult := v.(*cacheItem)
	loadedResult.once.Do(func() {
		// sync.Once guarantees that only one routine will execute this, the others will wait till return
		loadedResult.ret, loadedResult.err = loader(ctx)
	})
	return loadedResult.ret, loadedResult.err
}

func funWithCache(ctx context.Context, id int64) (int64, error) {
	cacheKey := "funWithCache" + strconv.FormatInt(id, 10)
	ret, err := LoadFromCtxCache(ctx, cacheKey, func(ctx context.Context) (interface{}, error) {
		return id, nil
	})
	return ret.(int64), err
}

func funWithCacheOld(ctx context.Context, id int64) (int64, error) {
	cacheKey := "funWithCacheOld" + strconv.FormatInt(id, 10)
	ret, err := loadFromCtxCacheOld(ctx, cacheKey, func(ctx context.Context) (interface{}, error) {
		return id, nil
	})
	return ret.(int64), err
}

// BenchmarkLoadFromCtxCache/new-12				292658	      3797 ns/op	     932 B/op	      22 allocs/op
// BenchmarkLoadFromCtxCache/old-12				296752	      3999 ns/op	    1105 B/op	      27 allocs/op
func BenchmarkLoadFromCtxCache(b *testing.B) {
	id := rand.Int63()
	const goNum = 5
	b.Run("new", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			ctx := WithCallCache(context.Background())
			var wg sync.WaitGroup
			for i := 0; i < goNum; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, _ = funWithCache(ctx, id)
				}()
			}
			wg.Wait()
		}
	})
	b.Run("old", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			ctx := withCallCacheOld(context.Background())
			var wg sync.WaitGroup
			for i := 0; i < goNum; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, _ = funWithCacheOld(ctx, id)
				}()
			}
			wg.Wait()
		}
	})
}
