# Media Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Media Service provides video and audio processing, streaming, and integration with external video platforms.

## Overview

This service handles media file processing including video conversion, HLS streaming, thumbnail generation, and downloading content from external platforms via yt-dlp integration.

## Features

- **Video Conversion** - Convert to MP4/H.264, HLS
- **Preview Generation** - Video thumbnails and timelines
- **Audio Processing** - Audio extraction and conversion
- **External Downloads** - YouTube, Vimeo, and other platforms
- **HLS Streaming** - Adaptive bitrate streaming
- **Playlist Management** - Channel and playlist sync

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Media Service                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Video Processor                          │ │
│  │                                                            │ │
│  │  Input ──▶ FFmpeg ──▶ Output (MP4, HLS, Preview)          │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Download Manager                          │ │
│  │                                                            │ │
│  │  URL ──▶ yt-dlp ──▶ Download ──▶ Process ──▶ Store        │ │
│  │                                                            │ │
│  │  Supported: YouTube, Vimeo, Dailymotion, Hulu, etc.       │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                 Channel/Playlist Sync                      │ │
│  │                                                            │ │
│  │  Playlist URL ──▶ Fetch Metadata ──▶ Download Videos      │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Video Processing

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateVideoPreview` | Generate preview image | `videoPath` |
| `CreateVideoTimeLine` | Generate timeline thumbnails | `videoPath`, `count` |
| `ConvertVideoToMpeg4H264` | Convert to MP4 | `input`, `output` |
| `ConvertVideoToHls` | Convert to HLS | `input`, `output` |
| `StartProcessVideo` | Start processing job | `videoPath` |
| `StopProcessVideo` | Cancel processing | `videoPath` |
| `IsProcessVideo` | Check if processing | `videoPath` |

### Audio Processing

| Method | Description | Parameters |
|--------|-------------|------------|
| `StartProcessAudio` | Process audio file | `audioPath` |

### External Downloads

| Method | Description | Parameters |
|--------|-------------|------------|
| `UploadVideo` | Download from URL | `url`, `destPath`, `options` |
| `SyncChannelFromPlaylist` | Sync playlist | `playlistUrl` |
| `GetChannel` | Get channel info | `channelId` |
| `ListChannels` | List all channels | - |
| `GetTorrentLnks` | Get available torrents | - |

### Playlist Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `GeneratePlaylist` | Create playlist file | `videos[]`, `output` |
| `CreateVttFile` | Generate subtitles | `videoPath` |
| `ListMediaFiles` | List all media | - |

## Usage Examples

### Go Client

```go
import (
    media "github.com/globulario/services/golang/media/media_client"
)

client, _ := media.NewMediaService_Client("localhost:10114", "media.MediaService")
defer client.Close()

// Create video preview
preview, err := client.CreateVideoPreview("/videos/movie.mp4")
// preview contains thumbnail image data

// Generate timeline (10 thumbnails)
timeline, err := client.CreateVideoTimeLine("/videos/movie.mp4", 10)
// timeline contains array of thumbnail images

// Convert to HLS for streaming
err = client.ConvertVideoToHls(
    "/videos/movie.mp4",
    "/hls/movie/playlist.m3u8",
)

// Download from YouTube
err = client.UploadVideo(
    "https://youtube.com/watch?v=xxxxx",
    "/downloads/video.mp4",
    &mediapb.DownloadOptions{
        Format:   "bestvideo[height<=1080]+bestaudio",
        Subtitle: true,
    },
)

// Sync YouTube playlist
err = client.SyncChannelFromPlaylist(
    "https://youtube.com/playlist?list=xxxxx",
)

// List downloaded media
files, err := client.ListMediaFiles()
for file := range files {
    fmt.Printf("%s - %d MB\n", file.Name, file.Size/1024/1024)
}
```

### Batch Processing

```go
// Start video processing
err := client.StartProcessVideo("/videos/raw/video1.mp4")

// Check processing status
processing, err := client.IsProcessVideo("/videos/raw/video1.mp4")
if processing {
    fmt.Println("Still processing...")
}

// Stop processing if needed
err = client.StopProcessVideo("/videos/raw/video1.mp4")
```

## Configuration

### Configuration File

```json
{
  "port": 10114,
  "ffmpegPath": "/usr/bin/ffmpeg",
  "ytdlpPath": "/usr/local/bin/yt-dlp",
  "mediaPath": "/var/lib/globular/media",
  "hlsSegmentDuration": 10,
  "previewSize": "320x180",
  "maxConcurrentJobs": 4,
  "downloadQuality": "bestvideo[height<=1080]+bestaudio"
}
```

## External Dependencies

- **FFmpeg** - Video/audio processing
- **yt-dlp** - External video downloads

## Integration

- [File Service](../file/README.md) - File storage
- [Title Service](../title/README.md) - Media metadata

---

[Back to Services Overview](../README.md)
