# 一、说明

Tus v1.0.0 协议 Go 客户端实现，参考自：[tus resumable upload protocol](https://github.com/tus/tus-resumable-upload-protocol/blob/main/protocol.md)

# 二、特性

- 完整实现 Tus v1.0.0 核心协议（`POST`、`HEAD`、`PATCH`、`DELETE`、`OPTIONS`、`GET`）
- 支持并发分片上传（`Upload-Concat: partial` / `final`）
- 支持从文件或 `io.Reader` 流式上传
- 支持文件下载到本地文件或 `io.Writer`
- 支持自定义 HTTP Client、分片大小、日志等配置
- 内置结构化日志，支持多级别输出

# 三、安装

```shell
go get gitee.com/ivfzhou/tus_client/v2@latest
```

# 四、快速开始

```golang
package main

import (
    "context"
    "fmt"
    tus "gitee.com/ivfzhou/tus_client/v2"
)

func main() {
    ctx := context.Background()

    // 创建客户端（默认使用 HTTP 协议）
    client := tus.NewClient("your_host")

    // 方式一：并行分片上传（推荐大文件使用）
    fileId, err := client.MultipleUploadFromFile(ctx, "your_file_path")
    if err != nil {
        panic(err)
    }
    fmt.Println("上传成功，文件ID:", fileId)

    // 下载文件到本地
    err = client.DownloadToFile(ctx, fileId, "to_file_path")
    if err != nil {
        panic(err)
    }
}
```

# 五、配置选项

通过可选函数 `Option` 灵活配置客户端：

| 函数 | 类型 | 说明 | 默认值 |
|---|---|---|---|
| `WithHTTPClient(c *http.Client)` | `*http.Client` | 自定义 HTTP 客户端 | `http.DefaultClient` |
| `WithSchema(schema string)` | `string` | 请求协议（`"http"` 或 `"https"`） | `"http"` |
| `WithChunkSize(size int)` | `int` | 分片上传时的切片大小（字节） | `8 * 1024 * 1024` (8MB) |
| `WithLogger(logger Logger)` | `Logger` | 自定义日志实现 | 无（不输出日志） |
| `WithLogLevel(level int)` | `int` | 日志级别 | `Level_Info` |

### 使用示例

```golang
import (
    "net/http"
    tus "gitee.com/ivfzhou/tus_client/v2"
)

// 使用 HTTPS + 自定义 HTTP Client + 自定义分片大小
client := tus.NewClient("your_host",
    tus.WithSchema("https"),
    tus.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
    tus.WithChunkSize(4 * 1024 * 1024), // 4MB
    tus.WithLogger(yourLogger),
    tus.WithLogLevel(tus.Level_Debug),
)
```

### 日志接口

实现 `Logger` 接口即可接入自定义日志：

```golang
type Logger interface {
    Printf(ctx context.Context, level int, format string, args ...any)
}
```

内置日志级别常量：

| 常量 | 值 | 说明 |
|---|---|---|
| `Level_Debug` | 1 | 调试信息 |
| `Level_Info` | 2 | 一般信息（默认） |
| `Level_Warning` | 3 | 警告信息 |
| `Level_Error` | 4 | 错误信息 |
| `Level_Silent` | 5 | 静默（关闭所有日志） |

# 六、API 参考

### 高级便捷方法

#### 文件上传

| 方法 | 说明 |
|---|---|
| `MultipleUploadFromFile(ctx, filePath)` | 从文件路径读取，自动分片并发上传，返回服务端文件标识（location） |
| `MultipleUploadFromReader(ctx, reader io.Reader)` | 从 `io.Reader` 读取直到 EOF，自动分片并发上传 |
| `UploadFile(ctx, data []byte)` | 上传字节数据（单次完整上传，不分片） |

#### 文件下载

| 方法 | 说明 |
|---|---|
| `DownloadToFile(ctx, location, destPath)` | 下载文件到本地指定路径（自动创建目录） |
| `DownloadToWriter(ctx, location, writer io.Writer)` | 下载文件数据写入 `io.Writer` |

#### 分片管理（手动控制分片流程）

| 方法 | 说明 |
|---|---|
| `UploadPart(ctx, data []byte) (location, error)` | 上传单个分片（字节数组），返回分片的 location |
| `UploadPartByIO(ctx, data io.ReadCloser, length int) (location, error)` | 上传单个分片（IO 流），返回分片的 location |
| `MergeParts(ctx, parts []string) (location, error)` | 按顺序合并多个分片（parts 为各分片的 location 列表），返回合并后文件的 location。**合并后会自动删除原分片。** |
| `DiscardParts(ctx, parts []string) error` | 批量丢弃/删除已上传的分片 |

#### 文件管理

| 方法 | 说明 |
|---|---|
| `DeleteFile(ctx, location) error` | 根据文件标识删除服务端文件 |

### 底层 Tus 协议方法

完整映射 Tus v1.0.0 协议的 HTTP 接口：

| 方法 | 对应 HTTP 方法 | 说明 |
|---|---|---|
| `Options(ctx) (*OptionsResult, error)` | OPTIONS | 获取服务器能力（支持的扩展、版本、最大文件大小、校验算法等） |
| `Post(ctx, *PostRequest) (*PostResult, error)` | POST | 创建上传资源（支持 partial/final concat） |
| `Head(ctx, *HeadRequest) (*HeadResult, error)` | HEAD | 查询上传状态（偏移量、长度、元信息等） |
| `Patch(ctx, *PatchRequest) (*PatchResult, error)` | PATCH | 上传文件数据块（字节切片） |
| `PatchByIO(ctx, *PatchByIORequest) (*PatchResult, error)` | PATCH | 上传文件数据块（IO 流） |
| `Delete(ctx, *DeleteRequest) (*DeleteResult, error)` | DELETE | 删除上传资源 |
| `Get(ctx, *GetRequest) (*GetResult, error)` | GET | 下载文件数据 |

### 请求 / 响应结构体

#### PostRequest

创建上传资源的请求参数。

| 字段 | 类型 | 说明 | 默认值 |
|---|---|---|---|
| `TusResumable` | `string` | Tus 协议版本 | `"1.0.0"` |
| `UploadMetadata` | `map[string]string` | 文件元信息 | - |
| `UploadLength` | `int` | 文件总大小 | - |
| `UploadDeferLength` | `bool` | 延迟确定文件大小 | `false` |
| `Body` | `[]byte` | 文件数据（非分片场景使用） | - |
| `UploadConcat` | `string` | 分片模式：`"partial"` 或 `"final;/files/a /files/b ..."` | - |

#### PatchRequest / PatchByIORequest

续传数据块的请求参数。

| 字段 | 类型 | 说明 | 默认值 |
|---|---|---|---|
| `Location` | `string` | 文件标识（必填） | - |
| `TusResumable` | `string` | Tus 协议版本 | `"1.0.0"` |
| `UploadOffset` | `int` | 当前偏移量 | - |
| `Body` / `Body` | `[]byte` / `io.ReadCloser` | 数据内容 | - |
| `BodySize` | `int` | 数据大小（仅 `PatchByIORequest`） | - |
| `UploadChecksum` | `string` | 数据校验和 | - |
| `UploadChecksumAlgorithm` | `string` | 校验和算法 | - |

#### OptionsResult

服务器能力响应。

| 字段 | 说明 |
|---|---|
| `HTTPStatus` | HTTP 响应码 |
| `TusExtension` | 服务器支持的扩展列表 |
| `TusResumable` | 服务端运行的 Tus 协议版本 |
| `TusVersion` | 服务器支持的协议版本列表 |
| `TusMaxSize` | 单次允许的最大文件大小 |
| `TusChecksumAlgorithm` | 支持的校验和算法列表 |

#### HeadResult

文件状态查询响应。

| 字段 | 说明 |
|---|---|
| `HTTPStatus` | HTTP 响应码 |
| `TusResumable` | 服务端 Tus 协议版本 |
| `UploadOffset` | 当前已上传偏移量 |
| `UploadLength` | 文件总大小 |
| `UploadMetadata` | 文件元信息 |
| `UploadDeferLength` | 是否延迟确定大小 |
| `UploadConcat` | 分片信息列表（final 合并文件时有效） |

# 七、典型使用场景

### 场景一：简单文件上传 + 下载

```golang
client := tus.NewClient("example.com")

// 上传
fileId, err := client.UploadFile(ctx, fileData)
if err != nil { /* 处理错误 */ }

// 下载
err = client.DownloadToFile(ctx, fileId, "./downloads/output.dat")
```

### 场景二：大文件并行分片上传（自动分片）

```golang
client := tus.NewClient("example.com",
    tus.WithChunkSize(10 * 1024 * 1024), // 10MB 每片
    tus.WithSchema("https"),
)

// 自动读取文件、分片、并发上传、合并、清理分片
fileId, err := client.MultipleUploadFromFile(ctx, "/path/to/large_file.zip")
```

### 场景三：手动分片上传（完全控制）

```golang
// 1. 上传分片
part1, _ := client.UploadPart(ctx, chunk1)
part2, _ := client.UploadPart(ctx, chunk2)
part3, _ := client.UploadPart(ctx, chunk3)

// 2. 合并分片（按传入顺序合并，并自动删除原分片）
fileId, err := client.MergeParts(ctx, []string{part1, part2, part3})

// 如果需要取消上传，丢弃所有分片
_ = client.DiscardParts(ctx, []string{part1, part2, part3})
```

### 场景四：查询服务器能力

```golang
result, err := client.Options(ctx)
fmt.Println("Tus Version:", result.TusVersion)
fmt.Println("Max File Size:", result.TusMaxSize)
fmt.Println("Extensions:", result.TusExtension)
```
