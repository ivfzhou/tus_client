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
	"net/http"
)

type Logger interface {
	Error(ctx context.Context, msg string)
	Info(ctx context.Context, msg string)
	Warn(ctx context.Context, msg string)
	Debug(ctx context.Context, msg string)
}

type Options struct {
	hc        *http.Client
	schema    string
	chunkSize int
	logger    Logger
}

type Option func(*Options)

func WithHTTPClient(c *http.Client) Option {
	return func(o *Options) {
		o.hc = c
	}
}

func WithSchema(schema string) Option {
	return func(o *Options) {
		o.schema = schema
	}
}

func WithChunkSize(chunkSize int) Option {
	return func(o *Options) {
		o.chunkSize = chunkSize
	}
}

func WithLogger(logger Logger) Option {
	return func(o *Options) {
		o.logger = logger
	}
}
