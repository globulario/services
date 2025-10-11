
# File Service (Globular)

A gRPC-based File Service that provides secure, streaming-friendly filesystem operations for Globular clusters and apps. It exposes directory listing, file upload/download, metadata querying, thumbnail generation, PDF rendering from HTML, archive creation, and discovery of public directories.

> This README documents how to **use** the service from clients. It assumes a File Service server is already running and registered in your Globular environment.

---

## Features

- **Directory operations**
  - `ReadDir` — list directory contents with optional recursion and on-the-fly thumbnail generation.
  - `CreateDir`, `Rename` (dirs), `DeleteDir`.
- **File operations**
  - `ReadFile` (server-streamed bytes), `SaveFile` (client-streamed upload), `DeleteFile`.
  - `GetFileInfo` — retrieve metadata for a single path.
- **Media & utilities**
  - `GetThumbnails` — recursively enumerate images and return a JSON thumbnail index (as bytes).
  - `HtmlToPdf` — render HTML to a PDF (bytes).
  - `CreateArchive` — create a server-side archive (.zip/.tar…) from a list of paths and get its server path/URL.
- **Discovery**
  - `GetPublicDirs` — list shared/public directories exposed by the service.
- **Admin**
  - `Stop` — request the service to stop (admin only).

All long payloads (dir listings, file contents, thumbnails) are streamed for scalability.

---

## Client Library (Go)

The `file_client` package wraps the gRPC stubs and handles connection, context, and token propagation.

### Installation

```bash
go get github.com/globulario/services/golang/file/file_client@latest
```

(You also need the common `globular_client`, `security`, and `utility` packages which are pulled transitively.)

### Constructing a Client

```go
import fileclient "github.com/globulario/services/golang/file/file_client"

c, err := fileclient.NewFileService_Client("globular.io", "file.FileService")
if err != nil { panic(err) }
defer c.Close()
```

- **address**: DNS name (or host:port via service discovery) where your Globular node can resolve the FileService.
- **id**: service identifier, typically `"file.FileService"`.

### Authentication

If you authenticate via the Authentication service, pass the resulting JWT in calls that accept a `token` parameter. The client also tries to inject a **local token** (for service-to-service calls) automatically; override by passing your explicit `token` string.

```go
auth, _ := authentication_client.NewAuthenticationService_Client("globular.io", "authentication.AuthenticationService")
token, _ := auth.Authenticate("sa", "adminadmin") // example only
```

---

## Common Workflows

### 1) Upload (Save) a File

```go
// Upload a local file to the server path using MoveFile (wrapper over SaveFile stream)
err := c.MoveFile(token, "/local/tmp/hello.txt", "/docs/hello.txt")
```

`MoveFile` opens the local path and streams bytes to the server using `SaveFile`. The destination parent directories should exist; create them if needed:

```go
_ = c.CreateDir(token, "/", "docs") // idempotent on most servers
```

### 2) Download (Read) a File

```go
data, err := c.ReadFile(token, "/docs/hello.txt")
// data is []byte. Persist or process as needed.
```

### 3) File Metadata

```go
info, err := c.GetFileInfo(token, "/docs/hello.txt", false, 0, 0)
fmt.Println(info.GetPath(), info.GetIsDir(), info.GetSize())
```

### 4) List a Directory

```go
entries, err := c.ReadDir("/media", false, 64, 64) // no token required for listing public dirs
for _, e := range entries {
  fmt.Println(e.GetName(), e.GetPath(), e.GetIsDir())
}
```

- `recursive`: `true` to traverse subdirectories.
- `thumbnailHeight/Width`: optional hints for image previews in responses that support thumbnails.

### 5) Thumbnails Index (Gallery)

Produce a JSON index of images beneath a path; you can unmarshal it client-side:

```go
js, err := c.GetThumbnails("/media/photos", true, 128, 128)
var any interface{}
_ = json.Unmarshal([]byte(js), &any)
```

### 6) Delete Files/Directories

```go
_ = c.DeleteFile(token, "/docs/hello.txt")
_ = c.DeleteDir(token, "/docs")
```

### 7) HTML to PDF

```go
pdf, err := c.HtmlToPdf("<html><body><h1>Hi</h1></body></html>")
// pdf starts with "%PDF" and can be written to disk.
```

### 8) Create an Archive

```go
zipPath, err := c.CreateArchive(token, []string{"/media/a.txt", "/media/img/pic.png"}, "bundle.zip")
// zipPath is a server-side path or URL to the created archive.
```

### 9) Public Directories

```go
dirs, err := c.GetPublicDirs()
for _, d := range dirs { fmt.Println(d) }
```

---

## Error Handling

- RPCs return gRPC errors. For existence checks, test for “not found” messages according to your server implementation.
- Upload/download are streamed; partial writes may fail mid-stream — handle errors and retries appropriately.
- Some operations require authentication and/or RBAC authorization; unauthorized calls will return `PermissionDenied` or similar errors.

---

## Testing

End-to-end tests are provided and assume a running server and an authentication service with a known admin account:

- **Save → Read → Delete** file round-trip
- `ReadDir` and `GetThumbnails` sanity checks
- `HtmlToPdf` returns valid PDF bytes
- Basic negative cases (`/nope`, missing files)

Run:

```bash
go test ./file/file_client -v
```

Set your server to accept `sa/adminadmin` (or adjust the test harness to your credentials).

---

## Notes & Tips

- For large uploads, `MoveFile` uses a fixed chunk size (5 KB by default) — tune server-side flow control accordingly.
- `ReadDir` and `ReadFile` stream results; be mindful of client backpressure and timeouts for very large trees/files.
- Prefer RBAC-managed **public roots** for anonymous listing and reading; use tokens for private spaces.
- The client automatically reconnects with exponential retries on transient failures.

---

## API Summary (selected wrappers)

- `ReadDir(path, recursive, thumbH, thumbW) ([]*filepb.FileInfo, error)`
- `CreateDir(token, parent, name) error`
- `ReadFile(token, path) ([]byte, error)`
- `RenameDir(token, path, oldName, newName) error`
- `DeleteDir(token, path) error`
- `GetFileInfo(token, path, rec, thumbH, thumbW) (*filepb.FileInfo, error)`
- `MoveFile(token, localPath, destPath) error`  *(upload)*
- `DeleteFile(token, path) error`
- `GetThumbnails(path, rec, thumbH, thumbW) (string, error)` *(JSON string)*
- `HtmlToPdf(html string) ([]byte, error)`
- `CreateArchive(token string, paths []string, name string) (string, error)`
- `GetPublicDirs() ([]string, error)`
- `StopService()` *(admin)*

---

## License

Part of the Globular project. See repository license for details.
