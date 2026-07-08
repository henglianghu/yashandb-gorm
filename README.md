# gorm-yasdb|崖山数据库GORM驱动

gorm-yasdb 是 YashanDB 的 GORM 驱动，基于 GORM 框架实现，依赖崖山数据库 Go 驱动。

## 特性

- ✅ 要求 GORM **v1.31.1** 及以上（`gorm.io/gorm`）
- ✅ 自动迁移（AutoMigrate）
- ✅ 完整的 CRUD 操作支持
- ✅ 事务支持
- ✅ 关联关系（HasOne, HasMany, BelongsTo, Many2Many）
- ✅ 钩子与回调
- ✅ 连接池支持
- ✅ 保留关键字自动引号处理
- ✅ 大小写敏感命名模式

## 快速开始

```go
import (
    "gorm.io/gorm"
    yasdb "github.com/yashan-technologies/yashandb-gorm"
)

// 默认模式
db, err := gorm.Open(yasdb.Open("sys/password@host:port"), &gorm.Config{})

// 大小写敏感模式（所有标识符统一加双引号）
db, err := gorm.Open(yasdb.New(yasdb.Config{
    DSN:                  "sys/password@host:port",
    NamingCaseSensitive:  true,
}), &gorm.Config{})
```

## 大小写敏感模式

默认情况下，GORM 生成的 SQL 不会对普通列名加引号（保留字除外）。开启 `NamingCaseSensitive` 后：

| 模式 | 小写列名 `user_name` | 大写列名 `NAME` | 保留字 `ORDER` | 手动加引号 `"name"` |
| --- | --- | --- | --- | --- |
| 关闭（默认） | `user_name`（原样输出，数据库转大写） | `NAME`（原样输出） | `"ORDER"`（转大写加引号） | `"name"`（原样输出） |
| 开启 | `"user_name"`（原样加引号） | `"NAME"`（加引号） | `"ORDER"`（加引号） | `"name"`（原样输出） |

### 小写列名手动加引号

如果需要在数据库中保留精确的小写列名，可在 gorm tag 中手动加引号：

```go
type Model struct {
    ID   uint   `gorm:"primaryKey"`
    Name string `gorm:"column:\"name\""`  // 精确小写 "name"
}
```

这种情况下，引号标识符在所有模式下都原样输出，不做任何转换：

```go
// 任何模式下
UPDATE models SET "name"=:1 WHERE id = :3
```

```go
// 关闭时（默认）
UPDATE users SET user_name=:1, "DATE"=:2 WHERE id = :3

// 开启后
UPDATE "users" SET "user_name"=:1,"DATE"=:2 WHERE id = :3
```

## 保留字处理

内置 520 个 YashanDB 保留关键字（来自 `V$RESERVED_WORDS` 视图），DDL 和 DML 生成时自动处理引号。当列名或表名与保留字冲突时，会自动加上双引号避免语法错误。

```go
type Model struct {
    Date  string `gorm:"column:DATE"`  // DATE 是保留字，自动转为 "DATE"
    Order string `gorm:"column:ORDER"` // ORDER 是保留字，自动转为 "ORDER"
    Name  string `gorm:"column:NAME"`  // NAME 非保留字，保持原样
}
```

## 兼容性说明

| 组件 | 版本要求 |
| ---- | -------- |
| Go | 1.21 及以上 |
| GORM | v1.31.1 及以上（`gorm.io/gorm`） |
| yashandb-go | v1.4.3 |

| 驱动版本 | 版本发布时间 | 新特性     |
| -------- | ------------ | ---------- |
| 1.0.2    | 2026.3.18    | 首版本支持 |