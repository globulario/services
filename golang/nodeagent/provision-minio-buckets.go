package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	// Read credentials from single source of truth
	credFile := "/var/lib/globular/minio/credentials"
	data, err := os.ReadFile(credFile)
	if err != nil {
		log.Fatalf("Failed to read credentials from %s: %v", credFile, err)
	}

	parts := strings.Split(strings.TrimSpace(string(data)), ":")
	if len(parts) != 2 {
		log.Fatalf("Invalid credentials format in %s", credFile)
	}

	accessKey := strings.TrimSpace(parts[0])
	secretKey := strings.TrimSpace(parts[1])
	endpoint := "127.0.0.1:9000"
	bucketName := "globular"
	domain := "localhost"

	fmt.Printf("=== MinIO Bucket Provisioning ===\n")
	fmt.Printf("Endpoint: %s\n", endpoint)
	fmt.Printf("Access Key: %s\n", accessKey)
	fmt.Printf("Bucket: %s\n", bucketName)
	fmt.Printf("Domain: %s\n\n", domain)

	// Create MinIO client
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}

	ctx := context.Background()

	// Check if bucket exists
	fmt.Printf("Checking if bucket '%s' exists...\n", bucketName)
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		log.Fatalf("Failed to check bucket: %v", err)
	}

	if !exists {
		fmt.Printf("Creating bucket '%s'...\n", bucketName)
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Fatalf("Failed to create bucket: %v", err)
		}
		fmt.Printf("✓ Bucket created\n")
	} else {
		fmt.Printf("✓ Bucket already exists\n")
	}

	// Create sentinel files for domain-scoped prefixes
	sentinels := []string{
		fmt.Sprintf("%s/users/.keep", domain),
		fmt.Sprintf("%s/webroot/.keep", domain),
	}

	fmt.Printf("\nCreating sentinel files...\n")
	for _, key := range sentinels {
		fmt.Printf("  Creating: %s/%s\n", bucketName, key)

		reader := strings.NewReader("")
		_, err := client.PutObject(ctx, bucketName, key, reader, 0, minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		})
		if err != nil {
			log.Fatalf("Failed to create sentinel %s: %v", key, err)
		}
		fmt.Printf("  ✓ Created\n")
	}

	// Upload local webroot files if they exist
	webrootLocal := "/var/lib/globular/webroot"
	if stat, err := os.Stat(webrootLocal); err == nil && stat.IsDir() {
		fmt.Printf("\nUploading webroot files from %s...\n", webrootLocal)

		entries, err := os.ReadDir(webrootLocal)
		if err != nil {
			log.Printf("Warning: Failed to read webroot directory: %v", err)
		} else {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				localPath := webrootLocal + "/" + entry.Name()
				objectKey := fmt.Sprintf("%s/webroot/%s", domain, entry.Name())

				fmt.Printf("  Uploading: %s -> %s/%s\n", entry.Name(), bucketName, objectKey)

				_, err := client.FPutObject(ctx, bucketName, objectKey, localPath, minio.PutObjectOptions{
					ContentType: "application/octet-stream",
				})
				if err != nil {
					log.Printf("  Warning: Failed to upload %s: %v", entry.Name(), err)
				} else {
					fmt.Printf("  ✓ Uploaded\n")
				}
			}
		}
	}

	fmt.Printf("\n=== Verification ===\n")

	// List objects in bucket
	fmt.Printf("Objects in bucket '%s':\n", bucketName)
	objectCh := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	count := 0
	for object := range objectCh {
		if object.Err != nil {
			log.Fatalf("Error listing objects: %v", object.Err)
		}
		fmt.Printf("  - %s (%d bytes, %s)\n", object.Key, object.Size, object.LastModified.Format(time.RFC3339))
		count++
	}

	if count == 0 {
		fmt.Printf("  (no objects found)\n")
	}

	fmt.Printf("\n=== SUCCESS ===\n")
	fmt.Printf("MinIO bucket provisioned successfully!\n")
	fmt.Printf("  Bucket: %s\n", bucketName)
	fmt.Printf("  Objects: %d\n", count)
	fmt.Printf("\nYou can now:\n")
	fmt.Printf("  1. Login to MinIO console: http://localhost:9001\n")
	fmt.Printf("  2. Username: %s\n", accessKey)
	fmt.Printf("  3. Password: %s\n", secretKey)
	fmt.Printf("  4. Browse bucket: %s\n", bucketName)
}
