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
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitee.com/ivfzhou/gotools/v4"
)

type client struct {
	host string
	opt  Options
}

func NewClient(host string, opts ...Option) TusClient {
	c := &client{host: host}
	for _, fn := range opts {
		fn(&c.opt)
	}
	if c.opt.hc == nil {
		c.opt.hc = http.DefaultClient
	}
	if len(c.opt.schema) <= 0 {
		c.opt.schema = "http"
	}
	if c.opt.chunkSize <= 0 {
		c.opt.chunkSize = 8 * 1024 * 1024
	}
	return c
}

func (c *client) Options(ctx context.Context) (*OptionsResult, error) {
	// 判断ctx是否被关闭
	select {
	case <-ctx.Done():
		c.errorLog(ctx, ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// 发送请求
	req, err := http.NewRequest(http.MethodOptions, fmt.Sprintf("%s://%s/files", c.opt.schema, c.host), nil)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	c.debugLog(ctx, "seed options request")
	rsp, err := c.opt.hc.Do(req)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	defer c.closeBody(ctx, rsp.Body)

	// 处理响应数据
	res := &OptionsResult{
		HTTPStatus:           rsp.StatusCode,
		TusExtension:         strings.Split(rsp.Header.Get("Tus-Extension"), ","),
		TusResumable:         rsp.Header.Get("Tus-Resumable"),
		TusChecksumAlgorithm: strings.Split(rsp.Header.Get("Tus-Checksum-Algorithm"), ","),
		TusVersion:           strings.Split(rsp.Header.Get("Tus-Version"), ","),
	}
	res.TusMaxSize, _ = strconv.Atoi(rsp.Header.Get("Tus-Max-Size"))

	return res, nil
}

func (c *client) Post(ctx context.Context, pr *PostRequest) (*PostResult, error) {
	// 判断ctx是否被关闭
	select {
	case <-ctx.Done():
		c.errorLog(ctx, ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// 发送请求
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s://%s/files", c.opt.schema, c.host), bytes.NewReader(pr.Body))
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	if len(pr.TusResumable) <= 0 {
		pr.TusResumable = "1.0.0"
	}
	req.Header.Set("Tus-Resumable", pr.TusResumable)
	req.Header.Set("Content-Length", strconv.Itoa(len(pr.Body)))
	if pr.UploadDeferLength {
		req.Header.Set("Upload-Defer-Length", "1")
	} else {
		req.Header.Set("Upload-Length", strconv.Itoa(pr.UploadLength))
	}
	var meta []string
	for k, v := range pr.UploadMetadata {
		meta = append(meta, fmt.Sprintf("%s %s", url.QueryEscape(k), base64.StdEncoding.EncodeToString([]byte(v))))
	}
	if len(meta) > 0 {
		req.Header.Set("Upload-Metadata", strings.Join(meta, ","))
	}
	if len(pr.UploadConcat) > 0 {
		req.Header.Set("Upload-Concat", pr.UploadConcat)
	}
	c.debugLog(ctx, "seed post request")
	rsp, err := c.opt.hc.Do(req)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	defer c.closeBody(ctx, rsp.Body)

	// 处理响应数据
	res := &PostResult{
		HTTPStatus:   rsp.StatusCode,
		TusResumable: rsp.Header.Get("Tus-Resumable"),
	}
	res.UploadOffset, _ = strconv.Atoi(rsp.Header.Get("Upload-Offset"))
	u, err := url.Parse(rsp.Header.Get("Location"))
	if err != nil {
		return nil, err
	}
	paths := strings.Split(u.Path, "/")
	res.Location = paths[len(paths)-1]

	return res, nil
}

func (c *client) Head(ctx context.Context, hr *HeadRequest) (*HeadResult, error) {
	// 判断ctx是否被关闭
	select {
	case <-ctx.Done():
		c.errorLog(ctx, ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// 发送请求
	req, err := http.NewRequest(http.MethodHead, fmt.Sprintf("%s://%s/files/%s", c.opt.schema, c.host, hr.Location), nil)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	if len(hr.TusResumable) <= 0 {
		hr.TusResumable = "1.0.0"
	}
	c.debugLog(ctx, "seed head request")
	rsp, err := c.opt.hc.Do(req)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	defer c.closeBody(ctx, rsp.Body)

	// 处理响应数据
	res := &HeadResult{
		HTTPStatus:   rsp.StatusCode,
		TusResumable: rsp.Header.Get("Tus-Resumable"),
	}
	res.UploadOffset, _ = strconv.Atoi(rsp.Header.Get("Upload-Offset"))
	res.UploadLength, _ = strconv.Atoi(rsp.Header.Get("Upload-Length"))
	res.UploadDeferLength = rsp.Header.Get("Upload-Defer-Length") == "1"
	meta := rsp.Header.Get("Upload-Metadata")
	kv := strings.Split(meta, ",")
	res.UploadMetadata = make(map[string]string, len(kv))
	uc := rsp.Header.Get("Upload-Concat")
	if len(uc) > 0 && strings.HasPrefix(uc, "final;") {
		for _, v := range strings.Split(uc, " ") {
			arr := strings.Split(v, "/")
			res.UploadConcat = append(res.UploadConcat, arr[len(arr)-1])
		}
	}
	for _, v := range kv {
		pair := strings.Split(v, " ")
		if len(pair) >= 2 {
			key, err := url.QueryUnescape(pair[0])
			if err == nil {
				value, err := base64.StdEncoding.DecodeString(pair[1])
				if err == nil {
					res.UploadMetadata[key] = string(value)
				}
			}
		}
	}

	return res, nil
}

func (c *client) Patch(ctx context.Context, pr *PatchRequest) (*PatchResult, error) {
	// 判断ctx是否被关闭
	select {
	case <-ctx.Done():
		c.errorLog(ctx, ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// 发送请求
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s://%s/files/%s", c.opt.schema, c.host, pr.Location),
		bytes.NewReader(pr.Body))
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	if len(pr.TusResumable) <= 0 {
		pr.TusResumable = "1.0.0"
	}
	req.Header.Set("Tus-Resumable", pr.TusResumable)
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Content-Length", strconv.Itoa(len(pr.Body)))
	req.Header.Set("Upload-Offset", strconv.Itoa(pr.UploadOffset))
	if len(pr.UploadChecksum) > 0 && len(pr.UploadChecksumAlgorithm) > 0 {
		req.Header.Set("Upload-Checksum", fmt.Sprintf("%s %s",
			pr.UploadChecksumAlgorithm, base64.StdEncoding.EncodeToString([]byte(pr.UploadChecksum))))
	}
	c.debugLog(ctx, "seed patch request")
	rsp, err := c.opt.hc.Do(req)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	defer c.closeBody(ctx, rsp.Body)

	// 处理响应数据
	res := &PatchResult{
		HTTPStatus:   rsp.StatusCode,
		TusResumable: rsp.Header.Get("Tus-Resumable"),
	}
	res.UploadOffset, _ = strconv.Atoi(rsp.Header.Get("Upload-Offset"))
	res.UploadExpires, _ = time.Parse("Mon, 02 Jan 2006 15:04:05 GMT", rsp.Header.Get("Upload-Expires"))

	return res, nil
}

func (c *client) PatchByIO(ctx context.Context, pr *PatchByIORequest) (*PatchResult, error) {
	// 判断ctx是否被关闭
	select {
	case <-ctx.Done():
		c.errorLog(ctx, ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// 发送请求
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s://%s/files/%s", c.opt.schema, c.host, pr.Location),
		nil)
	if err != nil {
		return nil, err
	}
	if len(pr.TusResumable) <= 0 {
		pr.TusResumable = "1.0.0"
	}
	req.Header.Set("Tus-Resumable", pr.TusResumable)
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Content-Length", strconv.Itoa(pr.BodySize))
	req.Header.Set("Upload-Offset", strconv.Itoa(pr.UploadOffset))
	if len(pr.UploadChecksum) > 0 && len(pr.UploadChecksumAlgorithm) > 0 {
		req.Header.Set("Upload-Checksum", fmt.Sprintf("%s %s",
			pr.UploadChecksumAlgorithm, base64.StdEncoding.EncodeToString([]byte(pr.UploadChecksum))))
	}
	req.ContentLength = int64(pr.BodySize)
	req.Body = pr.Body
	c.debugLog(ctx, "seed patch request")
	rsp, err := c.opt.hc.Do(req)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	defer c.closeBody(ctx, rsp.Body)

	// 处理响应数据
	res := &PatchResult{
		HTTPStatus:   rsp.StatusCode,
		TusResumable: rsp.Header.Get("Tus-Resumable"),
	}
	res.UploadOffset, _ = strconv.Atoi(rsp.Header.Get("Upload-Offset"))
	res.UploadExpires, _ = time.Parse("Mon, 02 Jan 2006 15:04:05 GMT", rsp.Header.Get("Upload-Expires"))

	return res, nil
}

func (c *client) Delete(ctx context.Context, dr *DeleteRequest) (*DeleteResult, error) {
	// 判断ctx是否被关闭
	select {
	case <-ctx.Done():
		c.errorLog(ctx, ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// 发送请求
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s://%s/files/%s", c.opt.schema, c.host, dr.Location), nil)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	if len(dr.TusResumable) <= 0 {
		dr.TusResumable = "1.0.0"
	}
	req.Header.Set("Tus-Resumable", dr.TusResumable)
	c.debugLog(ctx, "seed delete request")
	rsp, err := c.opt.hc.Do(req)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	defer c.closeBody(ctx, rsp.Body)

	// 处理响应数据
	res := &DeleteResult{
		HTTPStatus:   rsp.StatusCode,
		TusResumable: rsp.Header.Get("Tus-Resumable"),
	}

	return res, nil
}

func (c *client) Get(ctx context.Context, gr *GetRequest) (*GetResult, error) {
	// 判断ctx是否被关闭
	select {
	case <-ctx.Done():
		c.errorLog(ctx, ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// 发送请求
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s://%s/files/%s", c.opt.schema, c.host, gr.Location), nil)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}
	if len(gr.TusResumable) <= 0 {
		gr.TusResumable = "1.0.0"
	}
	req.Header.Set("Tus-Resumable", gr.TusResumable)
	c.debugLog(ctx, "seed get request")
	rsp, err := c.opt.hc.Do(req)
	if err != nil {
		c.errorLog(ctx, err)
		return nil, err
	}

	// 处理响应数据
	res := &GetResult{
		HTTPStatus:   rsp.StatusCode,
		TusResumable: rsp.Header.Get("Tus-Resumable"),
		Body:         rsp.Body,
	}
	res.ContentLength, _ = strconv.Atoi(rsp.Header.Get("Content-Length"))

	return res, nil
}

func (c *client) MultipleUploadFromFile(ctx context.Context, filePath string) (location string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		c.errorLog(ctx, err)
		return "", err
	}
	defer c.closeBody(ctx, file)
	return c.MultipleUploadFromReader(ctx, file)
}

func (c *client) MultipleUploadFromReader(ctx context.Context, r io.Reader) (location string, err error) {
	// 判断ctx是否被关闭
	select {
	case <-ctx.Done():
		c.errorLog(ctx, ctx.Err())
		return "", ctx.Err()
	default:
	}

	// 并行上传
	type data struct {
		index int
		body  []byte
	}
	m := sync.Map{}
	runner, wait := gotools.NewRunner(ctx, 0, func(ctx context.Context, t *data) error {
		postResult, err := c.Post(ctx, &PostRequest{UploadConcat: "partial", UploadLength: len(t.body)})
		if err != nil {
			return err
		}
		if postResult.HTTPStatus != http.StatusCreated {
			return fmt.Errorf("POST partial error: %d %s", postResult.HTTPStatus, http.StatusText(postResult.HTTPStatus))
		}
		patchResult, err := c.Patch(ctx, &PatchRequest{Location: postResult.Location, Body: t.body})
		if err != nil {
			return err
		}
		if patchResult.HTTPStatus != http.StatusNoContent {
			return fmt.Errorf("PATCH partial error: %d %s", patchResult.HTTPStatus, http.StatusText(patchResult.HTTPStatus))
		}
		m.Store(t.index, postResult.Location)
		return nil
	})

	// 边读边上传
	index := 0
	for {
		index++
		buf := make([]byte, c.opt.chunkSize)
		n, err := io.ReadFull(r, buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			if err = runner(&data{index, buf[:n]}, false); err != nil {
				return "", err
			}
			break
		}
		if err != nil {
			return "", err
		}
		if err = runner(&data{index, buf}, false); err != nil {
			c.errorLog(ctx, err)
			return "", err
		}
	}

	// 等待处理完毕
	err = wait(true)
	if err != nil {
		c.errorLog(ctx, err)
		return "", err
	}

	// 处理分片
	indexes := make([]int, 0, index)
	m.Range(func(key, value any) bool {
		indexes = append(indexes, key.(int))
		return true
	})
	slices.Sort(indexes)
	partLocationIds := make([]string, 0, len(indexes))
	uc := make([]string, 0, len(indexes))
	for _, v := range indexes {
		value, _ := m.Load(v)
		s := value.(string)
		uc = append(uc, "/files/"+s)
		partLocationIds = append(partLocationIds, s)
	}

	// 请求合并分片
	postResult, err := c.Post(ctx, &PostRequest{UploadConcat: "final;" + strings.Join(uc, " ")})
	if err != nil {
		c.errorLog(ctx, err)
		return "", err
	}
	if postResult.HTTPStatus != http.StatusCreated {
		return "", fmt.Errorf("POST partial error: %d %s", postResult.HTTPStatus, http.StatusText(postResult.HTTPStatus))
	}

	// 删除分片
	for _, v := range partLocationIds {
		_, err = c.Delete(ctx, &DeleteRequest{Location: v})
		if err != nil {
			c.errorLog(ctx, err)
		}
	}

	return postResult.Location, nil
}

func (c *client) DownloadToWriter(ctx context.Context, location string, w io.Writer) error {
	// 判断ctx是否被关闭
	select {
	case <-ctx.Done():
		c.errorLog(ctx, ctx.Err())
		return ctx.Err()
	default:
	}

	// 请求下载
	result, err := c.Get(ctx, &GetRequest{Location: location})
	if err != nil {
		c.errorLog(ctx, err)
		return err
	}
	if result.HTTPStatus != http.StatusOK {
		return fmt.Errorf("%d %s", result.HTTPStatus, http.StatusText(result.HTTPStatus))
	}
	defer c.closeBody(ctx, result.Body)

	// 写入
	written, err := io.Copy(w, result.Body)
	if err != nil {
		c.errorLog(ctx, err)
		return err
	}
	if written != int64(result.ContentLength) {
		return fmt.Errorf(
			"the number of bytes [%d] written to the file does not equal the number of bytes [%d] downloaded from the data",
			written, result.ContentLength)
	}

	return nil
}

func (c *client) DownloadToFile(ctx context.Context, location, dest string) error {
	// 创建文件夹
	pdir := filepath.Dir(dest)
	if err := os.MkdirAll(pdir, 0755); err != nil {
		c.errorLog(ctx, err)
		return err
	}
	file, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		c.errorLog(ctx, err)
		return err
	}
	defer c.closeBody(ctx, file)

	return c.DownloadToWriter(ctx, location, file)
}

func (c *client) UploadPart(ctx context.Context, data []byte) (location string, err error) {
	postResult, err := c.Post(ctx, &PostRequest{UploadConcat: "partial", UploadLength: len(data)})
	if err != nil {
		return "", err
	}
	if postResult.HTTPStatus != http.StatusCreated {
		return "", fmt.Errorf("POST partial error: %d %s", postResult.HTTPStatus, http.StatusText(postResult.HTTPStatus))
	}

	patchResult, err := c.Patch(ctx, &PatchRequest{Location: postResult.Location, Body: data})
	if err != nil {
		return "", err
	}
	if patchResult.HTTPStatus != http.StatusNoContent {
		return "", fmt.Errorf("PATCH partial error: %d %s", patchResult.HTTPStatus, http.StatusText(patchResult.HTTPStatus))
	}

	return postResult.Location, nil
}

func (c *client) UploadPartByIO(ctx context.Context, data io.ReadCloser, length int) (location string, err error) {
	postResult, err := c.Post(ctx, &PostRequest{UploadConcat: "partial", UploadLength: length})
	if err != nil {
		return "", err
	}
	if postResult.HTTPStatus != http.StatusCreated {
		return "", fmt.Errorf("POST partial error: %d %s", postResult.HTTPStatus, http.StatusText(postResult.HTTPStatus))
	}

	patchResult, err := c.PatchByIO(ctx, &PatchByIORequest{Location: postResult.Location, Body: data, BodySize: length})
	if err != nil {
		return "", err
	}
	if patchResult.HTTPStatus != http.StatusNoContent {
		return "", fmt.Errorf("PATCH partial error: %d %s", patchResult.HTTPStatus, http.StatusText(patchResult.HTTPStatus))
	}

	return postResult.Location, nil
}

func (c *client) MergeParts(ctx context.Context, parts []string) (location string, err error) {
	uc := make([]string, 0, len(parts))
	for _, v := range parts {
		uc = append(uc, "/files/"+v)
	}
	postResult, err := c.Post(ctx, &PostRequest{UploadConcat: "final;" + strings.Join(uc, " ")})
	if err != nil {
		return "", err
	}
	if postResult.HTTPStatus != http.StatusCreated {
		return "", fmt.Errorf("POST partial error: %d %s", postResult.HTTPStatus, http.StatusText(postResult.HTTPStatus))
	}

	// 删除分片
	for _, v := range parts {
		_, err = c.Delete(ctx, &DeleteRequest{Location: v})
		if err != nil {
			c.errorLog(ctx, err)
		}
	}

	return postResult.Location, nil
}

func (c *client) DiscardParts(ctx context.Context, parts []string) error {
	var errs []error
	for _, v := range parts {
		_, err := c.Delete(ctx, &DeleteRequest{Location: v})
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (c *client) errorLog(ctx context.Context, msg any) {
	if c.opt.logger != nil {
		c.opt.logger.Error(ctx, fmt.Sprintf("%v", ctx))
	}
}

func (c *client) warnLog(ctx context.Context, msg any) {
	if c.opt.logger != nil {
		c.opt.logger.Warn(ctx, fmt.Sprintf("%v", ctx))
	}
}

func (c *client) infoLog(ctx context.Context, msg any) {
	if c.opt.logger != nil {
		c.opt.logger.Info(ctx, fmt.Sprintf("%v", ctx))
	}
}

func (c *client) debugLog(ctx context.Context, msg any) {
	if c.opt.logger != nil {
		c.opt.logger.Debug(ctx, fmt.Sprintf("%v", ctx))
	}
}

func (c *client) closeBody(ctx context.Context, r io.Closer) {
	if r != nil {
		err := r.Close()
		if err != nil {
			c.errorLog(ctx, err)
		}
	}
}
