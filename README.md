# 一、说明

Tus v1.0.0 协议 Go 客户端实现，参考自：[tus resumable upload protocol](https://github.com/tus/tus-resumable-upload-protocol/blob/main/protocol.md)

# 二、特性

- 完整实现 Tus v1.0.0 核心协议（`POST`、`HEAD`、`PATCH`、`DELETE`、`OPTIONS`、`GET`）
- 支持并发分片上传（`Upload-Concat: partial` / `final`）
- 支持从文件或 `io.Reader` 流式上传
- 支持文件下载到本地文件或 `io.Writer`
- 支持自定义 HTTP Client、分片大小、日志等配置
- 内置结构化日志，支持多级别输出


# 三、快速开始

```shell
go get gitee.com/ivfzhou/tus_client/v2@latest
```

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

# 四、典型使用场景

## 4.1 简单文件上传 + 下载

```golang
client := tus.NewClient("example.com")

// 上传
fileId, err := client.UploadFile(ctx, fileData)
if err != nil { /* 处理错误 */ }

// 下载
err = client.DownloadToFile(ctx, fileId, "./downloads/output.dat")
```

## 4.2 大文件并行分片上传（自动分片）

```golang
client := tus.NewClient("example.com",
    tus.WithChunkSize(10 * 1024 * 1024), // 10MB 每片
    tus.WithSchema("https"),
)

// 自动读取文件、分片、并发上传、合并、清理分片
fileId, err := client.MultipleUploadFromFile(ctx, "/path/to/large_file.zip")
```

## 4.3 手动分片上传（完全控制）

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

## 4.4 查询服务器能力

```golang
result, err := client.Options(ctx)
fmt.Println("Tus Version:", result.TusVersion)
fmt.Println("Max File Size:", result.TusMaxSize)
fmt.Println("Extensions:", result.TusExtension)
```
