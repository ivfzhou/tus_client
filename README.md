# 1. 说明

Tus v1.0.0
协议客户端实现，参考自：[tus resumable upload protocol](https://github.com/tus/tus-resumable-upload-protocol/blob/main/protocol.md)

# 2. 使用

```shell
go get gitee.com/ivfzhou/tus_client@latest
```

```golang
client := tus_client.NewClient("host")

// 并行分片上传
fileId, err := client.MultipleUploadFromFile(ctx, "your_file_path")

// 下载
err := client.DownloadToFile(ctx, "fileId", "to_file_path")

```

# 3. 联系作者

电邮：ivfzhou@126.com
