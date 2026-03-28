/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package blob

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Store struct {
	client *s3.Client
	bucket string
}

func newS3Store(cfg Config) (*s3Store, error) {
	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3 bucket name is required")
	}

	opts := func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		if cfg.S3Region != "" {
			o.Region = cfg.S3Region
		} else {
			o.Region = "us-east-1"
		}
		if cfg.S3AccessKeyID != "" {
			o.Credentials = credentials.NewStaticCredentialsProvider(
				cfg.S3AccessKeyID,
				cfg.S3SecretAccessKey,
				"",
			)
		}
		o.UsePathStyle = cfg.S3ForcePathStyle
	}

	client := s3.New(s3.Options{}, opts)

	return &s3Store{
		client: client,
		bucket: cfg.S3Bucket,
	}, nil
}

func (s *s3Store) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(contentType),
	}
	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("s3 put %q: %w", key, err)
	}
	return nil
}

func (s *s3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get %q: %w", key, err)
	}
	return out.Body, nil
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete %q: %w", key, err)
	}
	return nil
}
