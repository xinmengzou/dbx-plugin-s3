package main

import (
	"context"
	"strings"

	"github.com/minio/minio-go/v7"
)

const missingContinuationToken = "Truncated response should have continuation token set"

func listObjectsCompatible(context context.Context, client *minio.Client, bucket string, options minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	objects := make(chan minio.ObjectInfo, 1)
	go func() {
		defer close(objects)
		lastKey := options.StartAfter
		for {
			attempt := options
			attempt.StartAfter = lastKey
			retryWithV1 := false
			for object := range client.ListObjects(context, bucket, attempt) {
				if object.Err != nil {
					if !attempt.UseV1 && strings.Contains(object.Err.Error(), missingContinuationToken) {
						options.UseV1 = true
						retryWithV1 = true
						break
					}
					select {
					case objects <- object:
					case <-context.Done():
					}
					return
				}
				lastKey = object.Key
				select {
				case objects <- object:
				case <-context.Done():
					return
				}
			}
			if !retryWithV1 {
				return
			}
		}
	}()
	return objects
}
