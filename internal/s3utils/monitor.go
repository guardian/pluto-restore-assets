package s3utils

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func MonitorObjectRestoreStatus(ctx context.Context, client *s3.Client) ([]S3Entry, error) {
	keys, err := readManifestFile("/tmp/manifest.csv")
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %v", err)
	}
	// Remove keys that are directories and have the "/" suffix
	keys = removeDirectories(keys)

	log.Printf("Monitoring %d objects", len(keys))
	remainingKeys := keys
	for len(remainingKeys) > 0 {
		var stillRestoring []S3Entry
		for _, key := range remainingKeys {
			restored, err := checkRestoreStatus(ctx, client, key.Bucket, key.Key)
			if err != nil {
				log.Printf("Error checking restore status for %s/%s: %v", key.Bucket, key.Key, err)
				stillRestoring = append(stillRestoring, key)
				continue
			}
			if !restored {
				stillRestoring = append(stillRestoring, key)
			} else {
				log.Printf("Object %s/%s has been restored", key.Bucket, key.Key)
			}
		}

		if len(stillRestoring) == 0 {
			log.Println("All objects restored successfully")
			return keys, nil
		}

		remainingKeys = stillRestoring
		sleepDuration := time.Duration(15+rand.Intn(30)) * time.Minute
		log.Printf("%d objects still restoring. Waiting %v before next check...", len(remainingKeys), sleepDuration)
		log.Printf("Remaining keys: %v", remainingKeys)
		time.Sleep(sleepDuration)
	}
	return nil, nil
}

type S3Entry struct {
	Bucket string
	Key    string
}

func removeDirectories(keys []S3Entry) []S3Entry {
	var filteredKeys []S3Entry
	for _, key := range keys {
		if strings.HasSuffix(key.Key, "/") {
			log.Printf("Ignoring directory %s/%s", key.Bucket, key.Key)
		} else {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return filteredKeys
}
func readManifestFile(filepath string) ([]S3Entry, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []S3Entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ",")
		if len(parts) == 2 {
			entries = append(entries, S3Entry{Bucket: parts[0], Key: parts[1]})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func checkRestoreStatus(ctx context.Context, client S3Client, bucket, key string) (bool, error) {
	output, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, fmt.Errorf("head object failed: %w", err)
	}

	// If the object is not in Glacier, it's already available
	if output.StorageClass == "" || output.StorageClass == "STANDARD" {
		log.Printf("Object %s/%s is already in STANDARD storage", bucket, key)
		return true, nil
	}

	// Check if object is restored
	if output.Restore != nil {
		if strings.Contains(*output.Restore, "ongoing-request=\"false\"") {
			return true, nil
		}
	}

	return false, nil
}
