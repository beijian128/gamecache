package cachedb

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewWithCache(t *testing.T) {
	type User struct {
		ID   uint
		Name string
		Age  int
	}

	// 使用内存数据库（":memory:"）
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// 自动迁移
	err = db.AutoMigrate(&User{})
	if err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// 创建一个新的用户
	user := User{Name: "John Doe", Age: 30}
	result := db.Create(&user)
	if result.Error != nil {
		t.Fatalf("failed to create user: %v", result.Error)
	}

	userCache := NewWithCache[User](db, 10)

	// 从缓存获取用户
	u, err := userCache.Cache.Get(user.ID)
	if err != nil {
		t.Fatalf("failed to get from cache: %v", err)
	}

	// 验证缓存中的用户信息
	if u.(*User).Name != "John Doe" {
		t.Errorf("expected name 'John Doe', got '%s'", u.(*User).Name)
	}

	// 更新用户信息
	u.(*User).Name = "Jane Doe"

	userCache.Cache.Purge()

	// 从数据库查询用户
	var dbUser User
	if err := db.First(&dbUser, user.ID).Error; err != nil {
		t.Fatalf("failed to query from db: %v", err)
	}

	// 验证数据库中的用户信息是否已更新
	if dbUser.Name != "Jane Doe" {
		t.Errorf("expected name 'Jane Doe' in db, got '%s'", dbUser.Name)
	}

}
