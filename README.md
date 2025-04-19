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

读数据 user,_:=cache.Get(1)   若缓存中没有该key，会尝试从数据库中加载
写数据 user.(*User).Name = "李四"   , 对结构体字段直接赋值即可。 最终“李四”会被写入到数据库， 记录变为 {1,"李四",19}
