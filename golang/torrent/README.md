# Torrent Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Torrent Service provides BitTorrent download management capabilities.

## Overview

This service enables downloading content via the BitTorrent protocol with support for seeding, progress tracking, and download management.

## Features

- **Torrent Downloads** - Download files via BitTorrent
- **Seeding Support** - Optional seeding after download
- **Progress Tracking** - Real-time download progress
- **Download Rate** - Speed monitoring
- **File Selection** - Choose specific files from torrent

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Torrent Service                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Download Manager                          │ │
│  │                                                            │ │
│  │  Torrent ──▶ Peers ──▶ Download ──▶ Verify ──▶ Seed       │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Progress Tracker                          │ │
│  │                                                            │ │
│  │  Total Size │ Downloaded │ Rate │ Peers │ ETA             │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Seeding Manager                          │ │
│  │                                                            │ │
│  │  Completed ──▶ Seed (optional) ──▶ Upload to peers        │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Download Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `DownloadTorrent` | Start torrent download | `torrentFile`/`magnetUrl`, `destPath`, `seed` |
| `GetTorrentInfos` | Get download progress (streaming) | `torrentId` |
| `DropTorrent` | Stop/remove torrent | `torrentId` |
| `GetTorrentLnks` | List available torrents | - |

### Torrent Info Structure

```protobuf
message TorrentInfo {
    string id = 1;
    string name = 2;
    int64 totalSize = 3;      // Total bytes
    int64 downloaded = 4;     // Downloaded bytes
    float progress = 5;       // 0.0 - 1.0
    int64 downloadRate = 6;   // Bytes per second
    int64 uploadRate = 7;     // Bytes per second
    int32 numPeers = 8;       // Connected peers
    string status = 9;        // downloading, seeding, paused
    repeated FileInfo files = 10;
}
```

## Usage Examples

### Go Client

```go
import (
    torrent "github.com/globulario/services/golang/torrent/torrent_client"
)

client, _ := torrent.NewTorrentService_Client("localhost:10120", "torrent.TorrentService")
defer client.Close()

// Download from magnet link
err := client.DownloadTorrent(
    "magnet:?xt=urn:btih:...",
    "/downloads/",
    false, // don't seed after download
)

// Download from .torrent file
torrentData, _ := os.ReadFile("file.torrent")
err = client.DownloadTorrent(
    torrentData,
    "/downloads/",
    true, // seed after download
)

// Monitor progress
stream, err := client.GetTorrentInfos(torrentId)
for {
    info, err := stream.Recv()
    if err != nil {
        break
    }

    fmt.Printf("Name: %s\n", info.Name)
    fmt.Printf("Progress: %.1f%%\n", info.Progress*100)
    fmt.Printf("Speed: %.2f MB/s\n", float64(info.DownloadRate)/1024/1024)
    fmt.Printf("Peers: %d\n", info.NumPeers)
    fmt.Printf("Status: %s\n", info.Status)
}

// Stop torrent
err = client.DropTorrent(torrentId)

// List all torrents
torrents, err := client.GetTorrentLnks()
for _, t := range torrents {
    fmt.Printf("%s - %s\n", t.Name, t.Status)
}
```

## Configuration

```json
{
  "port": 10120,
  "downloadPath": "/var/lib/globular/downloads",
  "maxActiveTorrents": 5,
  "maxPeers": 50,
  "uploadLimit": 0,
  "downloadLimit": 0,
  "seedRatio": 0,
  "listenPort": 6881
}
```

## Dependencies

None - Uses embedded BitTorrent client.

---

[Back to Services Overview](../README.md)
