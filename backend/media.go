package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	pathpkg "path"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	dbxpluginsdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
)

type mediaByteRange struct {
	start   int64
	end     int64
	partial bool
}

func parseMediaRange(value string, size int64) (mediaByteRange, bool) {
	if size < 0 {
		return mediaByteRange{}, false
	}
	if size == 0 {
		return mediaByteRange{end: -1}, strings.TrimSpace(value) == ""
	}
	value = strings.TrimSpace(value)
	if value == "" {
		end := minInt64(size-1, maxMediaChunkBytes-1)
		return mediaByteRange{start: 0, end: end, partial: end+1 < size}, true
	}
	if !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return mediaByteRange{}, false
	}
	parts := strings.SplitN(strings.TrimPrefix(value, "bytes="), "-", 2)
	if len(parts) != 2 {
		return mediaByteRange{}, false
	}
	if parts[0] == "" {
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffix <= 0 {
			return mediaByteRange{}, false
		}
		if suffix > maxMediaChunkBytes {
			suffix = maxMediaChunkBytes
		}
		if suffix > size {
			suffix = size
		}
		return mediaByteRange{start: size - suffix, end: size - 1, partial: true}, true
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return mediaByteRange{}, false
	}
	end := size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return mediaByteRange{}, false
		}
		if end >= size {
			end = size - 1
		}
	}
	if end-start+1 > maxMediaChunkBytes {
		end = start + maxMediaChunkBytes - 1
	}
	return mediaByteRange{start: start, end: end, partial: true}, true
}

func (plugin *plugin) readMediaRange(values map[string]any) (any, *dbxpluginsdk.PluginError) {
	requestContext, cancel := operationContext()
	defer cancel()
	connection, pluginError := plugin.connectionFor(values)
	if pluginError != nil {
		return nil, pluginError
	}
	path, pluginError := parseObjectPath(stringValue(values["uri"]), connection)
	if pluginError != nil || path.key == "" || strings.HasSuffix(path.key, "/") {
		return nil, invalidParams("S3 media read requires a file URI")
	}
	remotePath := connection.remotePath(path)
	object, err := connection.client.GetObject(requestContext, remotePath.bucket, remotePath.key, minio.GetObjectOptions{})
	if err != nil {
		return nil, remoteError("S3 media read failed: " + err.Error())
	}
	defer object.Close()
	metadata, err := object.Stat()
	if err != nil {
		return nil, remoteError("S3 media read failed: " + err.Error())
	}
	contentType := metadata.ContentType
	declaredType := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	if declaredType == "" || strings.EqualFold(declaredType, "application/octet-stream") {
		if inferredType := mime.TypeByExtension(pathpkg.Ext(path.key)); inferredType != "" {
			contentType = inferredType
		}
	}
	result := map[string]any{
		"size":         metadata.Size,
		"contentType":  contentType,
		"etag":         metadata.ETag,
		"lastModified": metadata.LastModified.UTC().Format(time.RFC3339),
	}
	if boolValue(values["head"]) {
		return result, nil
	}
	byteRange, ok := parseMediaRange(stringValue(values["range"]), metadata.Size)
	if !ok {
		result["rangeNotSatisfiable"] = true
		return result, nil
	}
	if metadata.Size == 0 {
		result["dataBase64"] = ""
		result["offset"] = int64(0)
		result["length"] = int64(0)
		result["partial"] = false
		return result, nil
	}
	if _, err := object.Seek(byteRange.start, io.SeekStart); err != nil {
		return nil, remoteError("S3 media seek failed: " + err.Error())
	}
	length := byteRange.end - byteRange.start + 1
	data, err := io.ReadAll(io.LimitReader(object, length))
	if err != nil {
		return nil, remoteError("S3 media read failed: " + err.Error())
	}
	if int64(len(data)) != length {
		return nil, remoteError(fmt.Sprintf("S3 media read returned %d bytes, expected %d", len(data), length))
	}
	result["dataBase64"] = base64.StdEncoding.EncodeToString(data)
	result["offset"] = byteRange.start
	result["length"] = int64(len(data))
	result["partial"] = byteRange.partial
	return result, nil
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
