# suid

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25.0-blue.svg)](https://golang.org/)

suid is a high-performance globally unique identifier generation library that provides two types of unique IDs: SUID and GUID.

## Features

### SUID (Snowflake Unique Identifier)
- 64-bit integer with structure: sign(1)-group(3)-time(34)-seq(19)-host(7)
- Support creation from integer and string
- Support JSON serialization and deserialization
- Support database driver interface
- Support GORM data type interface
- Thread-safe and support concurrent access
- Maximum supported date is `2514-05-30 01:53:03 +0000`
- Maximum concurrent transactions is 524,288 per second. It will automatically wait for the next second

### GUID (Globally Unique Identifier)
- 10-byte array with structure: group ID(4 bit)-timestamp(7 bytes)-sequence number(12 bit)-host ID(1 byte)
- Support parsing from string
- Support JSON serialization and deserialization
- The maximum supported timestamp is `2540-11-08 07:35:09.481983 +0800 CST`

## Installation

```bash
go get sutext.github.io/suid
```

## Usage Examples

### SUID Example

```go
package main

import (
    "fmt"
    "sutext.github.io/suid"
)

func main() {
    // Create SUID with default group(0)
    id := suid.New()
    fmt.Println("SUID:", id.String())
    fmt.Println("SUID Integer:", id.Integer())
    fmt.Println("SUID Description:", id.Description())
    
    // Create SUID with specified group
    id2 := suid.New(1)
    fmt.Println("SUID with group 1:", id2.String())
    
    // Create SUID from integer
    id3 := suid.FromInteger(123456789)
    fmt.Println("SUID from integer:", id3.String())
    
    // Parse SUID from string
    id4, err := suid.FromString("abcdefghijklm")
    if err == nil {
        fmt.Println("SUID from string:", id4.String())
    }
    
    // Get host ID
    fmt.Println("Host ID:", suid.HostID())
}
```

### GUID Example

```go
package main

import (
    "fmt"
    "sutext.github.io/suid"
)

func main() {
    // Create GUID with default group(0)
    guid := suid.NewGUID()
    fmt.Println("GUID:", guid.String())
    fmt.Println("GUID Description:", guid.Description())
    
    // Create GUID with specified group
    guid2 := suid.NewGUID(1)
    fmt.Println("GUID with group 1:", guid2.String())
    
    // Parse GUID from string
    guid3, err := suid.ParseGUID("abcdefghijklmnop")
    if err == nil {
        fmt.Println("GUID from string:", guid3.String())
    }
}
```

## Configuration

### Host ID Configuration

SUID and GUID will automatically try to get host ID from the following environment variables:
1. `SUID_HOST_ID`: directly specify host ID
2. `POD_NAME`: Kubernetes Pod name (take the last part, separated by "-")
3. `HOSTNAME`: hostname (take the last part, separated by "-")

If none of the above is set, it will try to use the last part of the local IP address.
If still not available, it will randomly generate a host ID.

### Kubernetes Environment

In Kubernetes environment, it is recommended to use StatefulSet to provide a unique host ID.

## Performance Characteristics

- **SUID**: Maximum 524,288 concurrent transactions per second
- **GUID**: Uses atomic operations to generate sequence numbers, providing higher performance
- Both support high concurrency scenarios and are thread-safe

## Notes

1. Ensure to set a unique host ID in distributed environment to avoid ID conflicts
2. Clock skew will cause panic, ensure the system clock is running normally
3. The maximum date for SUID is `2514-05-30 01:53:03 +0000`
4. It is recommended to use SUID as the primary key for database tables to ensure uniqueness and performance

## License

MIT License

## Contribution

Welcome to submit Issues and Pull Requests to improve this project.