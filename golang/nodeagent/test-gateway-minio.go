package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioConfig struct {
	Type   string `json:"type"`
	Endpoint string `json:"endpoint"`
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
	Secure bool `json:"secure"`
	Auth struct {
		Mode     string `json:"mode"`
		CredFile string `json:"credFile"`
	} `json:"auth"`
}

func main() {
	// Load contract
	contractPath := "/var/lib/globular/objectstore/minio.json"
	data, err := os.ReadFile(contractPath)
	if err != nil {
		log.Fatalf("Failed to read contract: %v", err)
	}

	var cfg MinioConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse contract: %v", err)
	}

	fmt.Printf("Contract loaded:\n")
	fmt.Printf("  Endpoint: %s\n", cfg.Endpoint)
	fmt.Printf("  Bucket: %s\n", cfg.Bucket)
	fmt.Printf("  Secure: %t\n", cfg.Secure)
	fmt.Printf("  CredFile: %s\n", cfg.Auth.CredFile)
	fmt.Println()

	// Load credentials
	credData, err := os.ReadFile(cfg.Auth.CredFile)
	if err != nil {
		log.Fatalf("Failed to read credentials: %v", err)
	}

	parts := strings.Split(strings.TrimSpace(string(credData)), ":")
	if len(parts) != 2 {
		log.Fatalf("Invalid credentials format")
	}

	accessKey := parts[0]
	secretKey := parts[1]

	fmt.Printf("Credentials loaded:\n")
	fmt.Printf("  Access Key: %s\n", accessKey)
	fmt.Printf("  Secret Key: %s***\n", secretKey[:3])
	fmt.Println()

	// Create MinIO client
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: cfg.Secure,
	})
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}

	ctx := context.Background()

	// Check bucket exists
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		log.Fatalf("Failed to check bucket: %v", err)
	}

	fmt.Printf("Bucket '%s' exists: %t\n", cfg.Bucket, exists)
	fmt.Println()

	// Try to fetch the test file
	objectKey := "localhost/webroot/index.html"
	fmt.Printf("Attempting to fetch: %s/%s\n", cfg.Bucket, objectKey)

	obj, err := client.GetObject(ctx, cfg.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		log.Fatalf("Failed to get object: %v", err)
	}

	stat, err := obj.Stat()
	if err != nil {
		log.Fatalf("Failed to stat object: %v", err)
	}

	fmt.Printf("✓ Object found!\n")
	fmt.Printf("  Size: %d bytes\n", stat.Size)
	fmt.Printf("  ContentType: %s\n", stat.ContentType)
	fmt.Printf("  LastModified: %s\n", stat.LastModified)
	fmt.Println()

	// Read content
	buf := make([]byte, stat.Size)
	n, err := obj.Read(buf)
	if err != nil && err.Error() != "EOF" {
		log.Fatalf("Failed to read object: %v", err)
	}

	fmt.Printf("Content preview:\n")
	fmt.Printf("%s\n", string(buf[:n]))
	fmt.Println()

	fmt.Println("✓✓✓ All tests passed! MinIO is accessible and working!")
	fmt.Println()
	fmt.Println("The gateway SHOULD be able to serve this file.")
	fmt.Println("If it's not working, there might be an issue with the gateway code or configuration.")
}
