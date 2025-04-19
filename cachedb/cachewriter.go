package cachedb

import (
	"fmt"
	"reflect"
	"time"

	"github.com/bluele/gcache"
	"gorm.io/gorm"
)

// CacheDB 是一个带缓存的泛型数据库包装器
type CacheDB[T any] struct {
	db     *gorm.DB
	Cache  gcache.Cache
	copies map[interface{}]T // 保存深拷贝副本
}

// NewWithCache 创建一个新的带缓存的泛型DB实例
func NewWithCache[T any](db *gorm.DB, size int) *CacheDB[T] {
	c := &CacheDB[T]{
		db:     db,
		copies: make(map[interface{}]T),
	}

	c.Cache = gcache.New(size).
		LRU().
		Expiration(time.Second * 2).
		LoaderFunc(c.loadFromDB()).      // 缓存未命中时从数据库加载
		EvictedFunc(c.evictToDB()).      // 缓存淘汰时回写
		PurgeVisitorFunc(c.purgeToDB()). // 清空缓存时回写
		AddedFunc(c.logCacheAdd()).      // 可选的添加日志
		Build()

	return c
}

// loadFromDB 从数据库加载数据并保存副本
func (c *CacheDB[T]) loadFromDB() gcache.LoaderFunc {
	return func(key interface{}) (interface{}, error) {
		var entity T
		if err := c.db.First(&entity, key).Error; err != nil {
			return nil, fmt.Errorf("failed to load from DB: %w", err)
		}

		// 保存深拷贝副本
		copy := deepCopy(entity)
		c.copies[key] = copy

		return &entity, nil
	}
}

// evictToDB 缓存淘汰时的回写逻辑
func (c *CacheDB[T]) evictToDB() gcache.EvictedFunc {
	return func(key, value interface{}) {
		if err := c.saveIfModified(key, value); err != nil {
			fmt.Printf("Evict save failed: %v\n", err)
		}
		delete(c.copies, key) // 清理副本
		// 记录日志
		fmt.Printf("Evicted from cache: key=%v\n", key)
	}
}

// purgeToDB 清空缓存时的回写逻辑
func (c *CacheDB[T]) purgeToDB() gcache.PurgeVisitorFunc {
	return func(key, value interface{}) {
		if err := c.saveIfModified(key, value); err != nil {
			fmt.Printf("Purge save failed: %v\n", err)
		}
		delete(c.copies, key) // 清理副本
		// 记录日志
		fmt.Printf("Purged from cache: key=%v\n", key)
	}
}

// saveIfModified 比较新旧值并保存修改
func (c *CacheDB[T]) saveIfModified(key, newValue interface{}) error {
	// 获取保存的副本
	oldCopy, exists := c.copies[key]
	if !exists {
		return fmt.Errorf("no copy found for key %v", key)
	}

	// 类型断言
	newVal, ok := newValue.(*T)
	if !ok {
		return fmt.Errorf("invalid value type for key %v", key)
	}

	// 比较当前值与副本
	if !reflect.DeepEqual(oldCopy, *newVal) {
		if err := c.db.Model(&oldCopy).Updates(newVal).Error; err != nil {
			return fmt.Errorf("failed to update: %w", err)
		}
		fmt.Printf("Saved changes for key %v\n", key)
	}
	return nil
}

// logCacheAdd 可选的缓存添加日志
func (c *CacheDB[T]) logCacheAdd() func(key, value interface{}) {
	return func(key, value interface{}) {
		fmt.Printf("New cache added: key=%v\n", key)
	}
}

// deepCopy 创建深拷贝
func deepCopy[T any](src T) T {
	// 使用反射创建深拷贝
	original := reflect.ValueOf(src)
	cpy := reflect.New(original.Type()).Elem()

	// 递归拷贝
	copyRecursive(original, cpy)

	return cpy.Interface().(T)
}

// copyRecursive 递归拷贝结构体
func copyRecursive(original, cpy reflect.Value) {
	switch original.Kind() {
	case reflect.Ptr:
		// 解引用指针
		originalValue := original.Elem()
		if !originalValue.IsValid() {
			return
		}
		cpy.Set(reflect.New(originalValue.Type()))
		copyRecursive(originalValue, cpy.Elem())

	case reflect.Interface:
		// 解引用接口
		if original.IsNil() {
			return
		}
		originalValue := original.Elem()
		copyValue := reflect.New(originalValue.Type()).Elem()
		copyRecursive(originalValue, copyValue)
		cpy.Set(copyValue)

	case reflect.Struct:
		// 拷贝结构体字段
		for i := 0; i < original.NumField(); i++ {
			if original.Type().Field(i).PkgPath != "" {
				continue // 跳过未导出字段
			}
			copyRecursive(original.Field(i), cpy.Field(i))
		}

	case reflect.Slice:
		// 拷贝切片
		if original.IsNil() {
			return
		}
		cpy.Set(reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		for i := 0; i < original.Len(); i++ {
			copyRecursive(original.Index(i), cpy.Index(i))
		}

	case reflect.Map:
		// 拷贝map
		if original.IsNil() {
			return
		}
		cpy.Set(reflect.MakeMap(original.Type()))
		for _, key := range original.MapKeys() {
			originalValue := original.MapIndex(key)
			copyValue := reflect.New(originalValue.Type()).Elem()
			copyRecursive(originalValue, copyValue)
			cpy.SetMapIndex(key, copyValue)
		}

	default:
		// 直接设置基础类型
		cpy.Set(original)
	}
}

// Get 从缓存或数据库获取值
func (c *CacheDB[T]) Get(key interface{}) (*T, error) {
	val, err := c.Cache.Get(key)
	if err != nil {
		return nil, err
	}
	return val.(*T), nil
}

// Set 设置缓存值
func (c *CacheDB[T]) Set(key interface{}, value T) error {
	// 保存深拷贝副本
	copy := deepCopy(value)
	c.copies[key] = copy

	return c.Cache.Set(key, &value)
}
