package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	endpoint := flag.String("endpoint", "", "MinIO endpoint (required, e.g. host:9000)")
	accessKey := flag.String("access-key", "minioadmin", "MinIO access key")
	secretKey := flag.String("secret-key", "minioadmin", "MinIO secret key")
	useSSL := flag.Bool("ssl", false, "Use SSL")
	caCert := flag.String("ca-cert", "", "Path to CA certificate for TLS verification")
	workflowDir := flag.String("workflow-dir", "", "Directory containing workflow definition YAML files to upload to globular-config/workflows/")
	flag.Parse()

	if *endpoint == "" {
		log.Fatal("--endpoint is required (e.g. --endpoint host:9000)")
	}

	// Initialize MinIO client
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(*accessKey, *secretKey, ""),
		Secure: *useSSL,
	}
	if *caCert != "" {
		caCertData, err := os.ReadFile(*caCert)
		if err != nil {
			log.Fatalf("Failed to read CA cert %s: %v", *caCert, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCertData) {
			log.Fatalf("Failed to parse CA cert %s", *caCert)
		}
		opts.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		}
	}
	minioClient, err := minio.New(*endpoint, opts)
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}

	ctx := context.Background()

	// Buckets to create
	buckets := []string{"webroot", "users", "globular-packages", "globular-config", "globular-backups"}

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

	// Upload workflow definitions to globular-config/workflows/
	if *workflowDir != "" {
		entries, err := os.ReadDir(*workflowDir)
		if err != nil {
			log.Fatalf("Failed to read workflow dir %s: %v", *workflowDir, err)
		}
		uploaded := 0
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
				continue
			}
			localPath := filepath.Join(*workflowDir, e.Name())
			objectName := "workflows/" + e.Name()
			_, err := minioClient.FPutObject(ctx, "globular-config", objectName, localPath, minio.PutObjectOptions{
				ContentType: "application/x-yaml",
			})
			if err != nil {
				log.Printf("Warning: Failed to upload %s: %v", objectName, err)
				continue
			}
			fmt.Printf("Uploaded workflow definition '%s' to globular-config/%s\n", e.Name(), objectName)
			uploaded++
		}
		fmt.Printf("Uploaded %d workflow definitions to globular-config/workflows/\n", uploaded)
	}

	fmt.Println("\nMinIO setup completed successfully!")
	fmt.Printf("Access the welcome page at: http://%s/webroot/index.html\n", *endpoint)
}
