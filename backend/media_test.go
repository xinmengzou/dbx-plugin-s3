package main

import (
	"encoding/base64"
	"net/url"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestParseMediaRange(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		size    int64
		start   int64
		end     int64
		partial bool
		valid   bool
	}{
		{name: "full small object", size: 128, start: 0, end: 127, valid: true},
		{name: "bounded open range", value: "bytes=10-", size: maxMediaChunkBytes + 100, start: 10, end: 10 + maxMediaChunkBytes - 1, partial: true, valid: true},
		{name: "explicit range", value: "bytes=4-11", size: 64, start: 4, end: 11, partial: true, valid: true},
		{name: "suffix", value: "bytes=-8", size: 64, start: 56, end: 63, partial: true, valid: true},
		{name: "past end", value: "bytes=64-", size: 64, valid: false},
		{name: "multiple", value: "bytes=0-1,4-5", size: 64, valid: false},
		{name: "empty", size: 0, start: 0, end: -1, valid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, valid := parseMediaRange(test.value, test.size)
			if valid != test.valid {
				t.Fatalf("valid=%v, expected %v", valid, test.valid)
			}
			if !valid {
				return
			}
			if result.start != test.start || result.end != test.end || result.partial != test.partial {
				t.Fatalf("unexpected range: %#v", result)
			}
		})
	}
}

func TestReadMediaRangeAgainstS3HTTPContract(t *testing.T) {
	const bucket = "example-bucket"
	fake, server := newFakeS3Server(bucket)
	defer server.Close()
	fake.objects["video.mp4"] = newFakeObject([]byte("0123456789abcdef"), "application/octet-stream")
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client, err := minio.New(endpoint.Host, &minio.Options{
		Creds:        credentials.NewStaticV4("access-key", "secret-key", ""),
		Secure:       false,
		Region:       "us-east-1",
		BucketLookup: minio.BucketLookupPath,
		MaxRetries:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance := &plugin{connections: map[string]*s3Connection{
		"connection-1": {bucket: bucket, client: client},
	}}
	result, pluginError := instance.readMediaRange(map[string]any{
		"connectionId": "connection-1",
		"uri":          "s3://example-bucket/video.mp4",
		"range":        "bytes=4-9",
	})
	if pluginError != nil {
		t.Fatal(pluginError.Message)
	}
	media := result.(map[string]any)
	data, err := base64.StdEncoding.DecodeString(media["dataBase64"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "456789" || media["offset"] != int64(4) || media["length"] != int64(6) || media["partial"] != true || media["contentType"] != "video/mp4" {
		t.Fatalf("unexpected media range: %#v data=%q", media, data)
	}
}
