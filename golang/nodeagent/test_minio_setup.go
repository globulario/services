package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("Globular MinIO ObjectStore Setup")
	fmt.Println("==================================================")
	fmt.Println()

	contractPath := os.Getenv("NODE_AGENT_MINIO_CONTRACT")
	if contractPath == "" {
		contractPath = "/tmp/globular-fix/objectstore/minio.json"
	}

	fmt.Printf("Loading contract from: %s\n", contractPath)

	f, err := os.Open(contractPath)
	if err != nil {
		log.Fatalf("Failed to open contract: %v", err)
	}
	defer f.Close()

	cfg, err := config.LoadMinioProxyConfigFrom(f)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("MinIO endpoint: %s (secure=%t)\n", cfg.Endpoint, cfg.Secure)
	fmt.Printf("Bucket: %s\n", cfg.Bucket)
	fmt.Printf("Auth mode: %s\n", cfg.Auth.Mode)
	fmt.Println()

	// Build credentials
	var creds *credentials.Credentials
	if cfg.Auth != nil && cfg.Auth.Mode == "file" && cfg.Auth.CredFile != "" {
		fmt.Printf("Reading credentials from: %s\n", cfg.Auth.CredFile)
		data, err := os.ReadFile(cfg.Auth.CredFile)
		if err != nil {
			log.Fatalf("Failed to read credentials: %v", err)
		}
		parts := strings.Split(strings.TrimSpace(string(data)), ":")
		if len(parts) != 2 {
			log.Fatalf("Invalid credential format (expected access:secret)")
		}
		creds = credentials.NewStaticV4(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), "")
		fmt.Printf("Using credentials from file\n")
	} else {
		fmt.Printf("Using default credentials (minioadmin)\n")
		creds = credentials.NewStaticV4("minioadmin", "minioadmin", "")
	}
	fmt.Println()

	// Create MinIO client
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Secure: cfg.Secure,
		Creds:  creds,
	})
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}

	ctx := context.Background()
	bucket := cfg.Bucket
	if bucket == "" {
		bucket = "globular"
	}

	// Check if bucket exists
	fmt.Printf("Checking bucket '%s'...\n", bucket)
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		log.Fatalf("Failed to check bucket: %v", err)
	}

	if exists {
		fmt.Printf("✓ Bucket '%s' already exists\n", bucket)
	} else {
		fmt.Printf("Creating bucket '%s'...\n", bucket)
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			log.Fatalf("Failed to create bucket: %v", err)
		}
		fmt.Printf("✓ Bucket '%s' created\n", bucket)
	}
	fmt.Println()

	// Create sentinels for domain
	domain := "localhost"
	usersSentinel := domain + "/users/.keep"
	webrootSentinel := domain + "/webroot/.keep"

	fmt.Printf("Creating sentinel: %s/%s\n", bucket, usersSentinel)
	if err := putSentinel(ctx, client, bucket, usersSentinel); err != nil {
		log.Fatalf("Failed to create users sentinel: %v", err)
	}
	fmt.Printf("✓ Users sentinel created\n")

	fmt.Printf("Creating sentinel: %s/%s\n", bucket, webrootSentinel)
	if err := putSentinel(ctx, client, bucket, webrootSentinel); err != nil {
		log.Fatalf("Failed to create webroot sentinel: %v", err)
	}
	fmt.Printf("✓ Webroot sentinel created\n")
	fmt.Println()

	// List buckets to verify
	fmt.Println("Verifying buckets...")
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		log.Printf("Warning: Could not list buckets: %v", err)
	} else {
		fmt.Printf("Found %d bucket(s):\n", len(buckets))
		for _, b := range buckets {
			fmt.Printf("  - %s (created: %s)\n", b.Name, b.CreationDate.Format("2006-01-02 15:04:05"))
		}
	}
	fmt.Println()

	fmt.Println("==================================================")
	fmt.Println("✓ ObjectStore layout successfully created!")
	fmt.Println("==================================================")
	fmt.Println()
	fmt.Println("You can now access the MinIO console at:")
	fmt.Println("  http://localhost:9001")
	fmt.Println()
	fmt.Println("Login with credentials:")
	fmt.Println("  Username: minioadmin")
	fmt.Println("  Password: minioadmin")
	fmt.Println()
}

func putSentinel(ctx context.Context, client *minio.Client, bucket, key string) error {
	_, err := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return nil // Already exists
	}
	reader := bytes.NewReader([]byte{})
	_, err = client.PutObject(ctx, bucket, key, reader, 0, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}
