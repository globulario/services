# File Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The File Service provides comprehensive file system operations and document management capabilities.

## Overview

This service handles all file-related operations including reading, writing, directory management, archive creation, thumbnail generation, and document conversion.

## Features

- **File Operations** - Read, write, copy, move, delete
- **Directory Management** - Create, list, recursive operations
- **Archive Creation** - ZIP file generation
- **Thumbnail Generation** - Image preview creation
- **Document Conversion** - HTML to PDF, Excel export
- **Metadata Extraction** - File information and attributes

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        File Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  File Operations                           │ │
│  │                                                            │ │
│  │  ReadFile │ SaveFile │ DeleteFile │ Copy │ Move │ Rename  │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                Directory Operations                        │ │
│  │                                                            │ │
│  │  ReadDir │ CreateDir │ DeleteDir │ ListRecursive          │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │               Advanced Operations                          │ │
│  │                                                            │ │
│  │  CreateArchive │ GetThumbnails │ HtmlToPdf │ WriteExcel   │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### File Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `ReadFile` | Read file contents | `path` |
| `SaveFile` | Write file contents | `path`, `data` |
| `DeleteFile` | Remove file | `path` |
| `GetFileInfo` | Get file metadata | `path` |
| `GetFileMetadata` | Get extended metadata | `path` |

### Directory Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `ReadDir` | List directory contents | `path`, `recursive` |
| `CreateDir` | Create directory | `path` |
| `DeleteDir` | Remove directory | `path` |
| `Rename` | Rename file/directory | `oldPath`, `newPath` |
| `Move` | Move file/directory | `source`, `dest` |
| `Copy` | Copy file/directory | `source`, `dest` |

### Advanced Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateArchive` | Create ZIP archive | `paths[]`, `output` |
| `GetThumbnails` | Generate image thumbnails | `path`, `sizes[]` |
| `HtmlToPdf` | Convert HTML to PDF | `html`, `output` |
| `WriteExcelFile` | Create Excel document | `data`, `output` |
| `CreateLnk` | Create shortcut/link | `target`, `link` |
| `UploadFile` | Download from URL | `url`, `dest` |

## Usage Examples

### Go Client

```go
import (
    file "github.com/globulario/services/golang/file/file_client"
)

client, _ := file.NewFileService_Client("localhost:10103", "file.FileService")
defer client.Close()

// Read file
content, err := client.ReadFile("/path/to/file.txt")
fmt.Printf("Content: %s\n", string(content))

// Save file
err = client.SaveFile("/path/to/new-file.txt", []byte("Hello, World!"))

// List directory
entries, err := client.ReadDir("/path/to/directory", false)
for _, entry := range entries {
    fmt.Printf("%s - %d bytes\n", entry.Name, entry.Size)
}

// Create directory
err = client.CreateDir("/path/to/new-directory")

// Copy file
err = client.Copy("/source/file.txt", "/dest/file.txt")

// Move file
err = client.Move("/old/path/file.txt", "/new/path/file.txt")

// Delete file
err = client.DeleteFile("/path/to/file.txt")

// Get file info
info, err := client.GetFileInfo("/path/to/file.txt")
fmt.Printf("Size: %d, Modified: %s\n", info.Size, info.ModTime)
```

### Create Archive

```go
// Create ZIP archive
files := []string{
    "/path/to/file1.txt",
    "/path/to/file2.txt",
    "/path/to/directory",
}
err := client.CreateArchive(files, "/output/archive.zip")
```

### Generate Thumbnails

```go
// Generate thumbnails at different sizes
sizes := []int{64, 128, 256}
thumbnails, err := client.GetThumbnails("/path/to/image.jpg", sizes)
for size, data := range thumbnails {
    fmt.Printf("Thumbnail %dx%d: %d bytes\n", size, size, len(data))
}
```

### HTML to PDF

```go
html := `
<!DOCTYPE html>
<html>
<body>
  <h1>Report</h1>
  <p>Generated document content...</p>
</body>
</html>
`
err := client.HtmlToPdf(html, "/output/report.pdf")
```

## Configuration

### Configuration File

```json
{
  "port": 10103,
  "rootPath": "/var/lib/globular/files",
  "publicPath": "/var/lib/globular/public",
  "maxFileSize": "100MB",
  "allowedExtensions": ["*"],
  "thumbnailSizes": [64, 128, 256, 512]
}
```

## Security

- Path traversal protection
- File type validation
- Size limits enforcement
- Permission-based access (via RBAC)

## Integration

Used by:
- [Media Service](../media/README.md) - Media file storage
- [Repository Service](../repository/README.md) - Artifact storage
- [Blog Service](../blog/README.md) - Attachment handling

---

[Back to Services Overview](../README.md)
