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
	Options(context.Context) (*OptionsResult, error)
	Post(context.Context, *PostRequest) (*PostResult, error)
	Head(context.Context, *HeadRequest) (*HeadResult, error)
	Patch(context.Context, *PatchRequest) (*PatchResult, error)
	Delete(context.Context, *DeleteRequest) (*DeleteResult, error)
	Get(context.Context, *GetRequest) (*GetResult, error)
	MultipleUploadFromFile(ctx context.Context, filePath string) (location string, err error)
	MultipleUploadFromReader(ctx context.Context, r io.Reader) (location string, err error)
	DownloadToFile(ctx context.Context, location, dest string) error
	DownloadToWriter(ctx context.Context, location string, w io.Writer) error
}

type OptionsResult struct {
	HTTPStatus           int
	TusExtension         []string
	TusResumable         string
	TusVersion           []string
	TusMaxSize           int
	TusChecksumAlgorithm []string
}

type PostRequest struct {
	TusResumable      string
	UploadMetadata    map[string]string
	UploadLength      int
	UploadDeferLength bool
	Body              []byte
	UploadConcat      string
}

type PostResult struct {
	HTTPStatus   int
	TusResumable string
	Location     string
	UploadOffset int
}

type HeadRequest struct {
	TusResumable string
	Location     string
}

type HeadResult struct {
	HTTPStatus        int
	TusResumable      string
	UploadOffset      int
	UploadLength      int
	UploadMetadata    map[string]string
	UploadDeferLength bool
	UploadConcat      []string
}

type PatchRequest struct {
	Location                string
	TusResumable            string
	UploadOffset            int
	Body                    []byte
	UploadChecksum          string
	UploadChecksumAlgorithm string
}

type PatchResult struct {
	HTTPStatus    int
	TusResumable  string
	UploadOffset  int
	UploadExpires time.Time
}

type DeleteRequest struct {
	TusResumable string
	Location     string
}

type DeleteResult struct {
	HTTPStatus   int
	TusResumable string
}

type GetRequest struct {
	TusResumable string
	Location     string
}

type GetResult struct {
	HTTPStatus    int
	TusResumable  string
	Body          io.ReadCloser
	ContentLength int
}
