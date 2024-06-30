/*
 * Copyright (c) 2023 ivfzhou
 * tus_client is licensed under Mulan PSL v2.
 * You can use this software according to the terms and conditions of the Mulan PSL v2.
 * You may obtain a copy of Mulan PSL v2 at:
 *          http://license.coscl.org.cn/MulanPSL2
 * THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
 * EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
 * MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
 * See the Mulan PSL v2 for more details.
 */

package tus_client

import (
	"context"
	"io"
	"time"
)

type TusClient interface {
	// Options 发送 HTTP OPTIONS 请求
	Options(context.Context) (*OptionsResult, error)

	// Post 发送 HTTP POST 请求
	Post(context.Context, *PostRequest) (*PostResult, error)

	// Head 发送 HTTP HEAD 请求
	Head(context.Context, *HeadRequest) (*HeadResult, error)

	// Patch 发送 HTTP PATCH 请求
	Patch(context.Context, *PatchRequest) (*PatchResult, error)

	// Delete 发送 HTTP DELETE 请求
	Delete(context.Context, *DeleteRequest) (*DeleteResult, error)

	// Get 发送 HTTP GET 请求
	Get(context.Context, *GetRequest) (*GetResult, error)

	// MultipleUploadFromFile 读取文件并发上传到服务器，返回文件在服务器标识
	MultipleUploadFromFile(ctx context.Context, filePath string) (location string, err error)

	// MultipleUploadFromReader 读取 IO 流直到 EOF，并发上传到服务器，返回文件在服务器标识
	MultipleUploadFromReader(ctx context.Context, r io.Reader) (location string, err error)

	// DownloadToFile 从服务器下载文件到本地
	DownloadToFile(ctx context.Context, location, dest string) error

	// DownloadToWriter 从服务器下载文件到指定的 IO 流中
	DownloadToWriter(ctx context.Context, location string, w io.Writer) error

	// UploadPart 上传分片
	UploadPart(ctx context.Context, data []byte) (location string, err error)

	// MergeParts 合并分片，切片顺序决定顺序合并顺序
	MergeParts(ctx context.Context, parts []string) (location string, err error)

	// DiscardParts 丢弃上传的分片
	DiscardParts(ctx context.Context, parts []string) error

	// UploadPartByIO 上传分片
	UploadPartByIO(ctx context.Context, data io.ReadCloser, length int) (location string, err error)

	// PatchByIO 发送 HTTP PATCH 请求
	PatchByIO(ctx context.Context, pr *PatchByIORequest) (*PatchResult, error)
}

type OptionsResult struct {
	// HTTPStatus 响应码
	HTTPStatus int
	// TusExtension 服务器支持的扩展
	TusExtension []string
	// TusResumable 服务器运行的Tus协议版本
	TusResumable string
	// TusVersion 服务器支持的协议版本
	TusVersion []string
	// TusMaxSize 服务器允许的单次最大文件上传大小
	TusMaxSize int
	// TusChecksumAlgorithm 服务器支持的文件校验和算法
	TusChecksumAlgorithm []string
}

type PostRequest struct {
	// TusResumable 客户端使用Tus协议版本，默认1.0.0
	TusResumable string
	// UploadMetadata 文件元信息
	UploadMetadata map[string]string
	// UploadLength 文件上传大小
	UploadLength int
	// UploadDeferLength 文件大小在完成上传后确定
	UploadDeferLength bool
	// Body 文件数据
	Body []byte
	// UploadConcat 分片上传
	UploadConcat string
}

type PostResult struct {
	// HTTPStatus 响应码
	HTTPStatus int
	// TusResumable 服务器运行的Tus协议版本
	TusResumable string
	// Location 文件标识
	Location string
	// UploadOffset 文件大小偏移量
	UploadOffset int
}

type HeadRequest struct {
	// TusResumable 客户端使用Tus协议版本，默认1.0.0
	TusResumable string
	// Location 文件标识
	Location string
}

type HeadResult struct {
	// HTTPStatus 响应码
	HTTPStatus int
	// TusResumable 服务器运行的Tus协议版本
	TusResumable string
	// UploadOffset 文件大小偏移量
	UploadOffset int
	// UploadLength 文件大小
	UploadLength int
	// UploadMetadata 文件元信息
	UploadMetadata map[string]string
	// UploadDeferLength 文件大小在完成上传后确定
	UploadDeferLength bool
	// UploadConcat 文件的分片信息
	UploadConcat []string
}

type PatchRequest struct {
	// Location 文件标识
	Location string
	// TusResumable 客户端使用Tus协议版本，默认1.0.0
	TusResumable string
	// UploadOffset 文件大小偏移量
	UploadOffset int
	// Body 文件数据
	Body []byte
	// UploadChecksum 数据校验和
	UploadChecksum string
	// UploadChecksumAlgorithm 校验和算法
	UploadChecksumAlgorithm string
}

type PatchByIORequest struct {
	// Location 文件标识
	Location string
	// TusResumable 客户端使用Tus协议版本，默认1.0.0
	TusResumable string
	// UploadOffset 文件大小偏移量
	UploadOffset int
	// Body 文件数据
	Body io.ReadCloser
	// BodySize 大小
	BodySize int
	// UploadChecksum 数据校验和
	UploadChecksum string
	// UploadChecksumAlgorithm 校验和算法
	UploadChecksumAlgorithm string
}

type PatchResult struct {
	// HTTPStatus 响应码
	HTTPStatus int
	// TusResumable 服务器运行的Tus协议版本
	TusResumable string
	// UploadOffset 文件大小偏移量
	UploadOffset int
	// UploadExpires 文件过期时间
	UploadExpires time.Time
}

type DeleteRequest struct {
	// TusResumable 客户端使用Tus协议版本，默认1.0.0
	TusResumable string
	// Location 文件标识
	Location string
}

type DeleteResult struct {
	// HTTPStatus 响应码
	HTTPStatus int
	// TusResumable 服务器运行的Tus协议版本
	TusResumable string
}

type GetRequest struct {
	// TusResumable 客户端使用Tus协议版本，默认1.0.0
	TusResumable string
	// Location 文件标识
	Location string
}

type GetResult struct {
	// HTTPStatus 响应码
	HTTPStatus int
	// TusResumable 服务器运行的Tus协议版本
	TusResumable string
	// Body 文件数据IO流
	Body io.ReadCloser
	// ContentLength 文件大小
	ContentLength int
}
