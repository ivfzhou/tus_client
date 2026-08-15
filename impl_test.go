/*
 * Copyright (c) 2026 ivfzhou
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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestClient 根据 httptest.Server 构建指向该服务器的客户端。
func newTestClient(s *httptest.Server, opts ...Option) TusClient {
	host := strings.TrimPrefix(s.URL, "http://")
	return NewClient(host, append([]Option{WithSchema("http")}, opts...)...)
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("example.com").(*client)
	if c.host != "example.com" {
		t.Fatalf("host = %q, want %q", c.host, "example.com")
	}
	if c.opt.schema != "http" {
		t.Fatalf("schema = %q, want %q", c.opt.schema, "http")
	}
	if c.opt.chunkSize != 8*1024*1024 {
		t.Fatalf("chunkSize = %d, want %d", c.opt.chunkSize, 8*1024*1024)
	}
	if c.opt.logLevel != Level_Info {
		t.Fatalf("logLevel = %d, want %d", c.opt.logLevel, Level_Info)
	}
	if c.opt.hc != http.DefaultClient {
		t.Fatalf("hc should be http.DefaultClient")
	}
}

func TestNewClientChunkSizeGuard(t *testing.T) {
	for _, size := range []int{0, -1, -1024} {
		c := NewClient("example.com", WithChunkSize(size)).(*client)
		if c.opt.chunkSize != 8*1024*1024 {
			t.Fatalf("chunkSize = %d for input %d, want default", c.opt.chunkSize, size)
		}
	}
}

func TestOptions(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodOptions {
			t.Errorf("method = %s, want OPTIONS", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Errorf("path = %s, want /files", r.URL.Path)
		}
		w.Header().Set("Tus-Extension", "creation,creation-with-upload,termination")
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.Header().Set("Tus-Version", "1.0.0,0.2.2")
		w.Header().Set("Tus-Max-Size", "1073741824")
		w.Header().Set("Tus-Checksum-Algorithm", "sha1,md5")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()

	res, err := newTestClient(s).Options(context.Background())
	if err != nil {
		t.Fatalf("Options error: %v", err)
	}
	if res.HTTPStatus != http.StatusNoContent {
		t.Fatalf("HTTPStatus = %d, want %d", res.HTTPStatus, http.StatusNoContent)
	}
	if res.TusResumable != "1.0.0" {
		t.Fatalf("TusResumable = %q, want 1.0.0", res.TusResumable)
	}
	wantExt := []string{"creation", "creation-with-upload", "termination"}
	if len(res.TusExtension) != len(wantExt) {
		t.Fatalf("TusExtension = %v, want %v", res.TusExtension, wantExt)
	}
	for i := range wantExt {
		if res.TusExtension[i] != wantExt[i] {
			t.Fatalf("TusExtension = %v, want %v", res.TusExtension, wantExt)
		}
	}
	if res.TusMaxSize != 1073741824 {
		t.Fatalf("TusMaxSize = %d, want 1073741824", res.TusMaxSize)
	}
	if len(res.TusChecksumAlgorithm) != 2 || res.TusChecksumAlgorithm[0] != "sha1" || res.TusChecksumAlgorithm[1] != "md5" {
		t.Fatalf("TusChecksumAlgorithm = %v, want [sha1 md5]", res.TusChecksumAlgorithm)
	}
	if len(res.TusVersion) != 2 || res.TusVersion[0] != "1.0.0" || res.TusVersion[1] != "0.2.2" {
		t.Fatalf("TusVersion = %v, want [1.0.0 0.2.2]", res.TusVersion)
	}
}

func TestOptionsMissingHeaders(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()

	res, err := newTestClient(s).Options(context.Background())
	if err != nil {
		t.Fatalf("Options error: %v", err)
	}
	if len(res.TusExtension) != 0 {
		t.Fatalf("TusExtension = %v, want empty", res.TusExtension)
	}
	if len(res.TusVersion) != 0 {
		t.Fatalf("TusVersion = %v, want empty", res.TusVersion)
	}
	if len(res.TusChecksumAlgorithm) != 0 {
		t.Fatalf("TusChecksumAlgorithm = %v, want empty", res.TusChecksumAlgorithm)
	}
	if res.TusMaxSize != 0 {
		t.Fatalf("TusMaxSize = %d, want 0", res.TusMaxSize)
	}
}

func TestOptionsInvalidMaxSize(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Tus-Max-Size", "not-a-number")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()

	res, err := newTestClient(s).Options(context.Background())
	if err != nil {
		t.Fatalf("Options error: %v", err)
	}
	if res.TusMaxSize != 0 {
		t.Fatalf("TusMaxSize = %d, want 0", res.TusMaxSize)
	}
}

func TestPost(t *testing.T) {
	var gotMeta string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Errorf("path = %s, want /files", r.URL.Path)
		}
		if r.Header.Get("Tus-Resumable") != "1.0.0" {
			t.Errorf("Tus-Resumable = %q, want 1.0.0", r.Header.Get("Tus-Resumable"))
		}
		if r.Header.Get("Upload-Length") != "3" {
			t.Errorf("Upload-Length = %q, want 3", r.Header.Get("Upload-Length"))
		}
		if r.Header.Get("Upload-Defer-Length") != "" {
			t.Errorf("Upload-Defer-Length should be empty")
		}
		gotMeta = r.Header.Get("Upload-Metadata")
		body, _ := io.ReadAll(r.Body)
		if string(body) != "abc" {
			t.Errorf("body = %q, want abc", body)
		}
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.Header().Set("Upload-Offset", "0")
		w.Header().Set("Location", "/files/123")
		w.WriteHeader(http.StatusCreated)
	}))
	defer s.Close()

	res, err := newTestClient(s).Post(context.Background(), &PostRequest{
		UploadLength:   3,
		Body:           []byte("abc"),
		UploadMetadata: map[string]string{"filename": "a b.txt"},
	})
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if res.HTTPStatus != http.StatusCreated {
		t.Fatalf("HTTPStatus = %d, want %d", res.HTTPStatus, http.StatusCreated)
	}
	if res.Location != "123" {
		t.Fatalf("Location = %q, want 123", res.Location)
	}
	if res.UploadOffset != 0 {
		t.Fatalf("UploadOffset = %d, want 0", res.UploadOffset)
	}
	wantMeta := "filename " + base64.StdEncoding.EncodeToString([]byte("a b.txt"))
	if gotMeta != wantMeta {
		t.Fatalf("Upload-Metadata = %q, want %q", gotMeta, wantMeta)
	}
}

func TestPostAbsoluteLocation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://example.com/files/abc123")
		w.WriteHeader(http.StatusCreated)
	}))
	defer s.Close()

	res, err := newTestClient(s).Post(context.Background(), &PostRequest{UploadLength: 0})
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if res.Location != "abc123" {
		t.Fatalf("Location = %q, want abc123", res.Location)
	}
}

func TestPostDefaultResumable(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Tus-Resumable") != "1.0.0" {
			t.Errorf("Tus-Resumable = %q, want 1.0.0", r.Header.Get("Tus-Resumable"))
		}
		w.Header().Set("Location", "/files/x")
		w.WriteHeader(http.StatusCreated)
	}))
	defer s.Close()

	if _, err := newTestClient(s).Post(context.Background(), &PostRequest{}); err != nil {
		t.Fatalf("Post error: %v", err)
	}
}

func TestPostDeferLength(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upload-Defer-Length") != "1" {
			t.Errorf("Upload-Defer-Length = %q, want 1", r.Header.Get("Upload-Defer-Length"))
		}
		if r.Header.Get("Upload-Length") != "" {
			t.Errorf("Upload-Length should be empty when defer")
		}
		w.Header().Set("Location", "/files/x")
		w.WriteHeader(http.StatusCreated)
	}))
	defer s.Close()

	if _, err := newTestClient(s).Post(context.Background(), &PostRequest{UploadDeferLength: true}); err != nil {
		t.Fatalf("Post error: %v", err)
	}
}

func TestHead(t *testing.T) {
	meta := "filename " + base64.StdEncoding.EncodeToString([]byte("a b.txt")) +
		",type " + base64.StdEncoding.EncodeToString([]byte("image/png"))
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s, want HEAD", r.Method)
		}
		if r.URL.Path != "/files/xyz" {
			t.Errorf("path = %s, want /files/xyz", r.URL.Path)
		}
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.Header().Set("Upload-Offset", "10")
		w.Header().Set("Upload-Length", "100")
		w.Header().Set("Upload-Metadata", meta)
		w.Header().Set("Upload-Concat", "final;/files/part1 /files/part2")
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	res, err := newTestClient(s).Head(context.Background(), &HeadRequest{Location: "xyz"})
	if err != nil {
		t.Fatalf("Head error: %v", err)
	}
	if res.HTTPStatus != http.StatusOK {
		t.Fatalf("HTTPStatus = %d, want 200", res.HTTPStatus)
	}
	if res.UploadOffset != 10 {
		t.Fatalf("UploadOffset = %d, want 10", res.UploadOffset)
	}
	if res.UploadLength != 100 {
		t.Fatalf("UploadLength = %d, want 100", res.UploadLength)
	}
	if res.UploadDeferLength {
		t.Fatalf("UploadDeferLength should be false")
	}
	if res.UploadMetadata["filename"] != "a b.txt" {
		t.Fatalf("metadata filename = %q, want a b.txt", res.UploadMetadata["filename"])
	}
	if res.UploadMetadata["type"] != "image/png" {
		t.Fatalf("metadata type = %q, want image/png", res.UploadMetadata["type"])
	}
	wantConcat := []string{"part1", "part2"}
	if len(res.UploadConcat) != 2 || res.UploadConcat[0] != wantConcat[0] || res.UploadConcat[1] != wantConcat[1] {
		t.Fatalf("UploadConcat = %v, want %v", res.UploadConcat, wantConcat)
	}
}

func TestHeadPartialConcat(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Upload-Offset", "0")
		w.Header().Set("Upload-Length", "5")
		w.Header().Set("Upload-Concat", "partial")
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	res, err := newTestClient(s).Head(context.Background(), &HeadRequest{Location: "x"})
	if err != nil {
		t.Fatalf("Head error: %v", err)
	}
	if len(res.UploadConcat) != 0 {
		t.Fatalf("UploadConcat = %v, want empty", res.UploadConcat)
	}
}

func TestHeadDeferLength(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Upload-Defer-Length", "1")
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	res, err := newTestClient(s).Head(context.Background(), &HeadRequest{Location: "x"})
	if err != nil {
		t.Fatalf("Head error: %v", err)
	}
	if !res.UploadDeferLength {
		t.Fatalf("UploadDeferLength should be true")
	}
}

func TestPatch(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/files/xyz" {
			t.Errorf("path = %s, want /files/xyz", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/offset+octet-stream" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Upload-Offset") != "5" {
			t.Errorf("Upload-Offset = %q, want 5", r.Header.Get("Upload-Offset"))
		}
		wantChecksum := "sha1 " + base64.StdEncoding.EncodeToString([]byte("deadbeef"))
		if r.Header.Get("Upload-Checksum") != wantChecksum {
			t.Errorf("Upload-Checksum = %q, want %q", r.Header.Get("Upload-Checksum"), wantChecksum)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "hello" {
			t.Errorf("body = %q, want hello", body)
		}
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.Header().Set("Upload-Offset", "10")
		w.Header().Set("Upload-Expires", "Mon, 01 Jan 2024 00:00:00 GMT")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()

	res, err := newTestClient(s).Patch(context.Background(), &PatchRequest{
		Location:                "xyz",
		UploadOffset:            5,
		Body:                    []byte("hello"),
		UploadChecksum:          "deadbeef",
		UploadChecksumAlgorithm: "sha1",
	})
	if err != nil {
		t.Fatalf("Patch error: %v", err)
	}
	if res.HTTPStatus != http.StatusNoContent {
		t.Fatalf("HTTPStatus = %d, want 204", res.HTTPStatus)
	}
	if res.UploadOffset != 10 {
		t.Fatalf("UploadOffset = %d, want 10", res.UploadOffset)
	}
	want := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if !res.UploadExpires.Equal(want) {
		t.Fatalf("UploadExpires = %v, want %v", res.UploadExpires, want)
	}
}

func TestPatchByIO(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.Header.Get("Content-Length") != "5" {
			t.Errorf("Content-Length = %q, want 5", r.Header.Get("Content-Length"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "hello" {
			t.Errorf("body = %q, want hello", body)
		}
		w.Header().Set("Upload-Offset", "5")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()

	res, err := newTestClient(s).PatchByIO(context.Background(), &PatchByIORequest{
		Location: "xyz",
		Body:     io.NopCloser(bytes.NewReader([]byte("hello"))),
		BodySize: 5,
	})
	if err != nil {
		t.Fatalf("PatchByIO error: %v", err)
	}
	if res.HTTPStatus != http.StatusNoContent {
		t.Fatalf("HTTPStatus = %d, want 204", res.HTTPStatus)
	}
	if res.UploadOffset != 5 {
		t.Fatalf("UploadOffset = %d, want 5", res.UploadOffset)
	}
}

func TestDelete(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/files/xyz" {
			t.Errorf("path = %s, want /files/xyz", r.URL.Path)
		}
		if r.Header.Get("Tus-Resumable") != "1.0.0" {
			t.Errorf("Tus-Resumable = %q, want 1.0.0", r.Header.Get("Tus-Resumable"))
		}
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()

	res, err := newTestClient(s).Delete(context.Background(), &DeleteRequest{Location: "xyz"})
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if res.HTTPStatus != http.StatusNoContent {
		t.Fatalf("HTTPStatus = %d, want 204", res.HTTPStatus)
	}
}

func TestGet(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/files/xyz" {
			t.Errorf("path = %s, want /files/xyz", r.URL.Path)
		}
		w.Header().Set("Content-Length", "5")
		_, _ = io.WriteString(w, "hello")
	}))
	defer s.Close()

	res, err := newTestClient(s).Get(context.Background(), &GetRequest{Location: "xyz"})
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	defer res.Body.Close()
	if res.HTTPStatus != http.StatusOK {
		t.Fatalf("HTTPStatus = %d, want 200", res.HTTPStatus)
	}
	if res.ContentLength != 5 {
		t.Fatalf("ContentLength = %d, want 5", res.ContentLength)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != "hello" {
		t.Fatalf("body = %q, want hello", body)
	}
}

func TestGetNoContentLength(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "chunked-data")
		w.(http.Flusher).Flush()
	}))
	defer s.Close()

	res, err := newTestClient(s).Get(context.Background(), &GetRequest{Location: "xyz"})
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	defer res.Body.Close()
	if res.ContentLength != -1 {
		t.Fatalf("ContentLength = %d, want -1", res.ContentLength)
	}
}

func TestUploadFile(t *testing.T) {
	var patchBody string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if r.Header.Get("Upload-Concat") != "" {
				t.Errorf("Upload-Concat should be empty, got %q", r.Header.Get("Upload-Concat"))
			}
			w.Header().Set("Location", "/files/file-1")
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			b, _ := io.ReadAll(r.Body)
			patchBody = string(b)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer s.Close()

	loc, err := newTestClient(s).UploadFile(context.Background(), []byte("payload"))
	if err != nil {
		t.Fatalf("UploadFile error: %v", err)
	}
	if loc != "file-1" {
		t.Fatalf("location = %q, want file-1", loc)
	}
	if patchBody != "payload" {
		t.Fatalf("patch body = %q, want payload", patchBody)
	}
}

func TestDeleteFile(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()

	if err := newTestClient(s).DeleteFile(context.Background(), "xyz"); err != nil {
		t.Fatalf("DeleteFile error: %v", err)
	}
}

func TestDeleteFileError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	if err := newTestClient(s).DeleteFile(context.Background(), "xyz"); err == nil {
		t.Fatalf("DeleteFile should return error for non-204 status")
	}
}

func TestUploadPart(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if r.Header.Get("Upload-Concat") != "partial" {
				t.Errorf("Upload-Concat = %q, want partial", r.Header.Get("Upload-Concat"))
			}
			w.Header().Set("Location", "/files/part-1")
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer s.Close()

	loc, err := newTestClient(s).UploadPart(context.Background(), []byte("part-data"))
	if err != nil {
		t.Fatalf("UploadPart error: %v", err)
	}
	if loc != "part-1" {
		t.Fatalf("location = %q, want part-1", loc)
	}
}

func TestUploadPartByIO(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Location", "/files/part-io")
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			b, _ := io.ReadAll(r.Body)
			if string(b) != "io-data" {
				t.Errorf("body = %q, want io-data", b)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer s.Close()

	loc, err := newTestClient(s).UploadPartByIO(
		context.Background(), io.NopCloser(bytes.NewReader([]byte("io-data"))), 7)
	if err != nil {
		t.Fatalf("UploadPartByIO error: %v", err)
	}
	if loc != "part-io" {
		t.Fatalf("location = %q, want part-io", loc)
	}
}

func TestMergeParts(t *testing.T) {
	var finalConcat string
	var deleted []string
	var mu sync.Mutex
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			finalConcat = r.Header.Get("Upload-Concat")
			w.Header().Set("Location", "/files/merged")
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			mu.Lock()
			deleted = append(deleted, path.Base(r.URL.Path))
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer s.Close()

	loc, err := newTestClient(s).MergeParts(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("MergeParts error: %v", err)
	}
	if loc != "merged" {
		t.Fatalf("location = %q, want merged", loc)
	}
	if finalConcat != "final;/files/a /files/b /files/c" {
		t.Fatalf("Upload-Concat = %q", finalConcat)
	}
	if len(deleted) != 3 || deleted[0] != "a" || deleted[1] != "b" || deleted[2] != "c" {
		t.Fatalf("deleted = %v, want [a b c]", deleted)
	}
}

func TestDiscardParts(t *testing.T) {
	var mu sync.Mutex
	var deleted []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		deleted = append(deleted, path.Base(r.URL.Path))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()

	if err := newTestClient(s).DiscardParts(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatalf("DiscardParts error: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("deleted count = %d, want 2", len(deleted))
	}
}

func TestDiscardPartsError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	if err := newTestClient(s).DiscardParts(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatalf("DiscardParts should return error")
	}
}

func TestMultipleUploadFromReader(t *testing.T) {
	var (
		mu          sync.Mutex
		parts       = map[string][]byte{}
		nextID      int
		finalConcat string
		deleted     int
	)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			uc := r.Header.Get("Upload-Concat")
			switch {
			case strings.HasPrefix(uc, "final;"):
				finalConcat = uc
				w.Header().Set("Location", "/files/final")
			case uc == "partial":
				mu.Lock()
				nextID++
				id := strconv.Itoa(nextID)
				mu.Unlock()
				w.Header().Set("Location", "/files/"+id)
			default:
				w.Header().Set("Location", "/files/single")
			}
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			b, _ := io.ReadAll(r.Body)
			loc := path.Base(r.URL.Path)
			mu.Lock()
			parts[loc] = b
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			mu.Lock()
			deleted++
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer s.Close()

	c := newTestClient(s, WithChunkSize(4))
	data := []byte("0123456789") // 分片为 4、4、2
	loc, err := c.MultipleUploadFromReader(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("MultipleUploadFromReader error: %v", err)
	}
	if loc != "final" {
		t.Fatalf("location = %q, want final", loc)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}

	// 依据 final 拼接顺序还原数据，验证并发上传后的顺序正确。
	if !strings.HasPrefix(finalConcat, "final;") {
		t.Fatalf("Upload-Concat = %q", finalConcat)
	}
	ids := strings.Split(strings.TrimPrefix(finalConcat, "final;"), " ")
	var got []byte
	for _, id := range ids {
		got = append(got, parts[path.Base(id)]...)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("reconstructed = %q, want %q", got, data)
	}
}

func TestMultipleUploadFromReaderSingleChunk(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			uc := r.Header.Get("Upload-Concat")
			if strings.HasPrefix(uc, "final;") {
				if uc != "final;/files/part-1" {
					t.Errorf("Upload-Concat = %q, want final;/files/part-1", uc)
				}
				w.Header().Set("Location", "/files/final")
			} else {
				w.Header().Set("Location", "/files/part-1")
			}
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer s.Close()

	loc, err := newTestClient(s, WithChunkSize(1024)).
		MultipleUploadFromReader(context.Background(), strings.NewReader("small"))
	if err != nil {
		t.Fatalf("MultipleUploadFromReader error: %v", err)
	}
	if loc != "final" {
		t.Fatalf("location = %q, want final", loc)
	}
}

func TestMultipleUploadFromReaderEmpty(t *testing.T) {
	var sawPost bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			sawPost = true
			if r.Header.Get("Upload-Concat") != "" {
				t.Errorf("Upload-Concat should be empty for empty upload, got %q", r.Header.Get("Upload-Concat"))
			}
			w.Header().Set("Location", "/files/empty")
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer s.Close()

	loc, err := newTestClient(s, WithChunkSize(4)).
		MultipleUploadFromReader(context.Background(), strings.NewReader(""))
	if err != nil {
		t.Fatalf("MultipleUploadFromReader error: %v", err)
	}
	if loc != "empty" {
		t.Fatalf("location = %q, want empty", loc)
	}
	if !sawPost {
		t.Fatalf("expected a POST request")
	}
}

func TestMultipleUploadFromFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "upload.bin")
	if err := os.WriteFile(file, []byte("file-content-1234567890"), 0644); err != nil {
		t.Fatal(err)
	}

	var (
		mu      sync.Mutex
		parts   = map[string][]byte{}
		nextID  int
		finalUC string
	)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			uc := r.Header.Get("Upload-Concat")
			if strings.HasPrefix(uc, "final;") {
				finalUC = uc
				w.Header().Set("Location", "/files/final-file")
			} else {
				mu.Lock()
				nextID++
				id := strconv.Itoa(nextID)
				mu.Unlock()
				w.Header().Set("Location", "/files/"+id)
			}
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			parts[path.Base(r.URL.Path)] = b
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer s.Close()

	loc, err := newTestClient(s, WithChunkSize(5)).
		MultipleUploadFromFile(context.Background(), file)
	if err != nil {
		t.Fatalf("MultipleUploadFromFile error: %v", err)
	}
	if loc != "final-file" {
		t.Fatalf("location = %q, want final-file", loc)
	}

	ids := strings.Split(strings.TrimPrefix(finalUC, "final;"), " ")
	var got []byte
	for _, id := range ids {
		got = append(got, parts[path.Base(id)]...)
	}
	if !bytes.Equal(got, []byte("file-content-1234567890")) {
		t.Fatalf("reconstructed = %q", got)
	}
}

func TestMultipleUploadFromFileNotExist(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer s.Close()

	if _, err := newTestClient(s).MultipleUploadFromFile(context.Background(), "/no/such/file"); err == nil {
		t.Fatalf("should return error for non-existent file")
	}
}

func TestDownloadToWriter(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "11")
		_, _ = io.WriteString(w, "hello world")
	}))
	defer s.Close()

	var buf bytes.Buffer
	if err := newTestClient(s).DownloadToWriter(context.Background(), "xyz", &buf); err != nil {
		t.Fatalf("DownloadToWriter error: %v", err)
	}
	if buf.String() != "hello world" {
		t.Fatalf("content = %q, want hello world", buf.String())
	}
}

func TestDownloadToWriterNoContentLength(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "chunked content")
		w.(http.Flusher).Flush()
	}))
	defer s.Close()

	var buf bytes.Buffer
	if err := newTestClient(s).DownloadToWriter(context.Background(), "xyz", &buf); err != nil {
		t.Fatalf("DownloadToWriter error: %v", err)
	}
	if buf.String() != "chunked content" {
		t.Fatalf("content = %q, want chunked content", buf.String())
	}
}

func TestDownloadToWriterStatusError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	var buf bytes.Buffer
	if err := newTestClient(s).DownloadToWriter(context.Background(), "xyz", &buf); err == nil {
		t.Fatalf("DownloadToWriter should return error for non-200 status")
	}
}

func TestDownloadToFile(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "5")
		_, _ = io.WriteString(w, "data!")
	}))
	defer s.Close()

	dest := filepath.Join(t.TempDir(), "nested", "out.bin")
	if err := newTestClient(s).DownloadToFile(context.Background(), "xyz", dest); err != nil {
		t.Fatalf("DownloadToFile error: %v", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "data!" {
		t.Fatalf("content = %q, want data!", b)
	}
}

func TestContextCancelled(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called when ctx is cancelled")
	}))
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := newTestClient(s)

	if _, err := c.Options(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Options err = %v, want context.Canceled", err)
	}
	if _, err := c.Post(ctx, &PostRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Post err = %v, want context.Canceled", err)
	}
	if _, err := c.Head(ctx, &HeadRequest{Location: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Head err = %v, want context.Canceled", err)
	}
	if _, err := c.Patch(ctx, &PatchRequest{Location: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Patch err = %v, want context.Canceled", err)
	}
	if _, err := c.PatchByIO(ctx, &PatchByIORequest{Location: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("PatchByIO err = %v, want context.Canceled", err)
	}
	if _, err := c.Delete(ctx, &DeleteRequest{Location: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete err = %v, want context.Canceled", err)
	}
	if _, err := c.Get(ctx, &GetRequest{Location: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get err = %v, want context.Canceled", err)
	}
	if _, err := c.MultipleUploadFromReader(ctx, strings.NewReader("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("MultipleUploadFromReader err = %v, want context.Canceled", err)
	}
	if err := c.DownloadToWriter(ctx, "x", io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("DownloadToWriter err = %v, want context.Canceled", err)
	}
	if _, err := c.UploadPart(ctx, []byte("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("UploadPart err = %v, want context.Canceled", err)
	}
	if _, err := c.MergeParts(ctx, []string{"a"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("MergeParts err = %v, want context.Canceled", err)
	}
	if _, err := c.UploadFile(ctx, []byte("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("UploadFile err = %v, want context.Canceled", err)
	}
	if err := c.DeleteFile(ctx, "x"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DeleteFile err = %v, want context.Canceled", err)
	}
	if err := c.DiscardParts(ctx, []string{"a"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("DiscardParts err = %v, want context.Canceled", err)
	}
}
