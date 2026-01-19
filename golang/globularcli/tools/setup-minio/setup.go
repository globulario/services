package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	endpoint := flag.String("endpoint", "localhost:9000", "MinIO endpoint")
	accessKey := flag.String("access-key", "minioadmin", "MinIO access key")
	secretKey := flag.String("secret-key", "minioadmin", "MinIO secret key")
	useSSL := flag.Bool("ssl", false, "Use SSL")
	flag.Parse()

	// Initialize MinIO client
	minioClient, err := minio.New(*endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(*accessKey, *secretKey, ""),
		Secure: *useSSL,
	})
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}

	ctx := context.Background()

	// Buckets to create
	buckets := []string{"webroot", "users"}

	// Create buckets
	for _, bucket := range buckets {
		exists, err := minioClient.BucketExists(ctx, bucket)
		if err != nil {
			log.Fatalf("Failed to check if bucket %s exists: %v", bucket, err)
		}

		if exists {
			fmt.Printf("Bucket '%s' already exists\n", bucket)
		} else {
			err = minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
			if err != nil {
				log.Fatalf("Failed to create bucket %s: %v", bucket, err)
			}
			fmt.Printf("Created bucket '%s'\n", bucket)
		}

		// Set bucket policy to public read for webroot
		if bucket == "webroot" {
			policy := `{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Principal": {"AWS": ["*"]},
						"Action": ["s3:GetObject"],
						"Resource": ["arn:aws:s3:::webroot/*"]
					}
				]
			}`
			err = minioClient.SetBucketPolicy(ctx, bucket, policy)
			if err != nil {
				log.Printf("Warning: Failed to set bucket policy for %s: %v", bucket, err)
			} else {
				fmt.Printf("Set public read policy for bucket '%s'\n", bucket)
			}
		}
	}

	// Upload files to webroot
	files := map[string]string{
		"/home/dave/Documents/tmp/index.html": "index.html",
		"/home/dave/Documents/tmp/logo.png":   "logo.png",
	}

	for localPath, objectName := range files {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			log.Printf("Warning: File not found: %s", localPath)
			continue
		}

		contentType := "text/html"
		if filepath.Ext(objectName) == ".png" {
			contentType = "image/png"
		}

		_, err = minioClient.FPutObject(ctx, "webroot", objectName, localPath, minio.PutObjectOptions{
			ContentType: contentType,
		})
		if err != nil {
			log.Fatalf("Failed to upload %s: %v", objectName, err)
		}
		fmt.Printf("Uploaded '%s' to webroot bucket as '%s'\n", localPath, objectName)
	}

	fmt.Println("\nMinIO setup completed successfully!")
	fmt.Printf("Access the welcome page at: http://%s/webroot/index.html\n", *endpoint)
}
