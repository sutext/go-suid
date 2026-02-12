# suid

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25.0-blue.svg)](https://golang.org/)

suid 是一个高性能的全局唯一标识符生成库，提供了两种类型的唯一ID：SUID 和 GUID。

## 功能特性

### SUID (Snowflake Unique Identifier)
- 64位整数，结构为：符号(1)-组(3)-时间(34)-序列(19)-主机(7)
- 支持从整数和字符串创建
- 支持 JSON 序列化和反序列化
- 支持数据库驱动接口
- 支持 GORM 数据类型接口
- 线程安全，支持并发访问
- 最大支持到 2514-05-30 01:53:03 +0000
- 每秒最多支持 524288 个并发事务

### GUID (Globally Unique Identifier)
- 10字节数组，结构为：组ID(1字节)-时间戳(6字节)-序列号(2字节)-主机ID(1字节)
- 支持从字符串解析
- 支持 JSON 序列化和反序列化
- 支持数据库驱动接口
- 支持 GORM 数据类型接口

## 安装

```bash
go get sutext.github.io/suid
```

## 使用示例

### SUID 示例

```go
package main

import (
    "fmt"
    "sutext.github.io/suid"
)

func main() {
    // 创建默认组(0)的 SUID
    id := suid.New()
    fmt.Println("SUID:", id.String())
    fmt.Println("SUID Integer:", id.Integer())
    fmt.Println("SUID Description:", id.Description())
    
    // 创建指定组的 SUID
    id2 := suid.New(1)
    fmt.Println("SUID with group 1:", id2.String())
    
    // 从整数创建 SUID
    id3 := suid.FromInteger(123456789)
    fmt.Println("SUID from integer:", id3.String())
    
    // 从字符串解析 SUID
    id4, err := suid.FromString("abcdefghijklm")
    if err == nil {
        fmt.Println("SUID from string:", id4.String())
    }
    
    // 获取主机ID
    fmt.Println("Host ID:", suid.HostID())
}
```

### GUID 示例

```go
package main

import (
    "fmt"
    "sutext.github.io/suid"
)

func main() {
    // 创建默认组(0)的 GUID
    guid := suid.NewGUID()
    fmt.Println("GUID:", guid.String())
    fmt.Println("GUID Description:", guid.Description())
    
    // 创建指定组的 GUID
    guid2 := suid.NewGUID(1)
    fmt.Println("GUID with group 1:", guid2.String())
    
    // 从字符串解析 GUID
    guid3, err := suid.ParseGUID("abcdefghijklmnop")
    if err == nil {
        fmt.Println("GUID from string:", guid3.String())
    }
}
```

## 配置

### 主机ID配置

SUID 和 GUID 会自动尝试从以下环境变量获取主机ID：
1. `SUID_HOST_ID`：直接指定主机ID
2. `POD_NAME`：Kubernetes Pod名称（取最后一部分，以"-"分隔）
3. `HOSTNAME`：主机名（取最后一部分，以"-"分隔）

如果以上都未设置，会尝试使用本地IP地址的最后一部分。
如果仍然无法获取，则会随机生成一个主机ID。

### Kubernetes 环境

在 Kubernetes 环境中，建议使用 StatefulSet 来提供唯一的主机ID。

## 性能特性

- **SUID**：每秒最多支持 524288 个并发事务
- **GUID**：使用原子操作生成序列号，性能更高
- 两者都支持高并发场景，线程安全

## 注意事项

1. 确保在分布式环境中设置唯一的主机ID，以避免ID冲突
2. 时钟回拨会导致 panic，确保系统时钟正常运行
3. SUID 的最大日期为 2514-05-30 01:53:03 +0000
4. 建议将 SUID 用作数据库表的主键，以确保唯一性和性能

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request 来改进这个项目。