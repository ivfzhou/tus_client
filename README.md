# 说明

Tus v1.0.0 协议客户端实现，参考自：[tus resumable upload protocol](https://github.com/tus/tus-resumable-upload-protocol/blob/main/protocol.md)

# 使用

```shell
go get gitee.com/ivfzhou/tus_client/v2@latest
```

```golang
client := tus_client.NewClient("your_host")

// 并行分片上传
fileId, err := client.MultipleUploadFromFile(ctx, "your_file_path")

// 下载
err := client.DownloadToFile(ctx, "fileId", "to_file_path")
```
