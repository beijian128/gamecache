# gamecache
基于 gcache 和 gorm 实现的带进程内缓存的ORM，自动化标记脏数据并写入数据库。对于调用方而言，读写都在cache上完成，数据库是完全透明的。
以 

```go
	type User struct {
		ID   uint
		Name string
		Age  int
	}
```
为例，数据库中存在一条 {1,"张三",19} 的记录

读数据： user:=cache.Get(ID)  若缓存中没有该key，会尝试从数据库中加载
写数据： 
