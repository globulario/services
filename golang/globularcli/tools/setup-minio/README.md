# MinIO Setup for Globular

This tool sets up MinIO buckets and uploads the Globular welcome page.

## What it does

1. Creates two MinIO buckets:
   - `webroot` - For web content (HTML, images, etc.) with public read access
   - `users` - For user files (private access)

2. Uploads files to the `webroot` bucket:
   - `index.html` - Globular cluster status and welcome page
   - `logo.png` - Globular logo

## Prerequisites

- MinIO server running (default: localhost:9000)
- Go 1.20+ installed
- Access credentials for MinIO

## Usage

### Quick Start (with default credentials)

```bash
./setup.sh
```

### Custom MinIO Configuration

Set environment variables before running:

```bash
export MINIO_ENDPOINT="minio.example.com:9000"
export MINIO_ACCESS_KEY="your-access-key"
export MINIO_SECRET_KEY="your-secret-key"
export MINIO_USE_SSL="true"
./setup.sh
```

### Direct Go Program Usage

```bash
# Build
go build -o setup-minio setup.go

# Run with custom options
./setup-minio \
  -endpoint="localhost:9000" \
  -access-key="minioadmin" \
  -secret-key="minioadmin" \
  -ssl=false
```

## Starting MinIO (if not running)

### Using Docker

```bash
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  --name minio \
  minio/minio server /data --console-address :9001
```

Access MinIO Console at: http://localhost:9001

### Using Binary

```bash
# Download MinIO (Linux)
wget https://dl.min.io/server/minio/release/linux-amd64/minio
chmod +x minio

# Start MinIO
MINIO_ROOT_USER=minioadmin MINIO_ROOT_PASSWORD=minioadmin \
  ./minio server /tmp/minio-data --console-address :9001
```

## Files Created

### index.html

A beautiful welcome page showing:
- Globular logo (animated)
- Cluster version and platform info
- Configuration details (services, storage, mesh, etc.)
- Status indicators
- Quick links to health check, metrics, and file browser
- Responsive design with gradient styling

### Buckets

- **webroot**: Public read access - serves static web content
- **users**: Private access - stores user files

## Integration with Gateway

The Globular gateway reads HTML code from the `webroot` bucket when configured to use MinIO as the object store. Place your static web applications in this bucket to serve them through the gateway.

## Troubleshooting

**Connection Refused**
- Ensure MinIO is running on the specified endpoint
- Check firewall rules

**Access Denied**
- Verify your access key and secret key are correct
- Check MinIO user permissions

**Files Not Found**
- Ensure `/home/dave/Documents/tmp/index.html` and `logo.png` exist
- The setup will skip missing files with a warning

## Verifying Setup

After running the setup, verify by accessing:

```bash
# Check buckets exist
curl http://localhost:9000/webroot/

# View welcome page
curl http://localhost:9000/webroot/index.html

# View logo
curl http://localhost:9000/webroot/logo.png --output /tmp/test-logo.png
```

Or open in browser:
- http://localhost:9000/webroot/index.html
