#!/bin/bash
set -euo pipefail

# MinIO Setup Script for Globular
# Creates webroot and users buckets, uploads index.html and logo

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENDPOINT="${MINIO_ENDPOINT:-localhost:9000}"
ACCESS_KEY="${MINIO_ACCESS_KEY:-minioadmin}"
SECRET_KEY="${MINIO_SECRET_KEY:-minioadmin}"
USE_SSL="${MINIO_USE_SSL:-false}"

echo "=================================================="
echo "Globular MinIO Setup"
echo "=================================================="
echo "Endpoint: $ENDPOINT"
echo "Access Key: $ACCESS_KEY"
echo "Using SSL: $USE_SSL"
echo ""

# Check if MinIO is accessible
if ! nc -z -w5 ${ENDPOINT%%:*} ${ENDPOINT##*:} 2>/dev/null; then
    echo "ERROR: Cannot connect to MinIO at $ENDPOINT"
    echo ""
    echo "Please ensure MinIO is running. You can start it with:"
    echo "  docker run -d -p 9000:9000 -p 9001:9001 \\"
    echo "    -e MINIO_ROOT_USER=minioadmin \\"
    echo "    -e MINIO_ROOT_PASSWORD=minioadmin \\"
    echo "    minio/minio server /data --console-address :9001"
    echo ""
    exit 1
fi

# Build the setup tool if needed
if [ ! -f "$SCRIPT_DIR/setup-minio" ]; then
    echo "Building setup tool..."
    cd "$SCRIPT_DIR"
    go build -o setup-minio setup.go
fi

# Run the setup
echo "Creating buckets and uploading files..."
"$SCRIPT_DIR/setup-minio" \
    -endpoint="$ENDPOINT" \
    -access-key="$ACCESS_KEY" \
    -secret-key="$SECRET_KEY" \
    -ssl="$USE_SSL"

echo ""
echo "=================================================="
echo "Setup completed successfully!"
echo "=================================================="
echo ""
echo "You can now access:"
echo "  - Welcome page: http://$ENDPOINT/webroot/index.html"
echo "  - Logo: http://$ENDPOINT/webroot/logo.png"
echo ""
echo "Buckets created:"
echo "  - webroot (public read)"
echo "  - users (private)"
echo ""
