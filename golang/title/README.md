# Title Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Title Service provides a media metadata database for managing video, movie, and TV show information.

## Overview

This service stores and retrieves metadata about media content including videos, actors, directors, publishers, and associated media like posters and previews.

## Features

- **Video Metadata** - Title, description, duration, rating
- **Person Database** - Actors, directors, writers
- **Publisher Info** - Production companies, distributors
- **Media Assets** - Posters, previews, thumbnails
- **Cast Relationships** - Person-to-video linkage
- **Full-Text Search** - Search across all metadata

## Core Entities

### Video

```
Video
├── id: unique identifier
├── title: display title
├── description: synopsis
├── publisher: production company
├── duration: runtime in seconds
├── rating: content rating
├── genre: category/genre
├── releaseDate: publication date
├── views: view count
├── tags: search tags
├── cast: person references
├── posters: image references
└── previews: preview references
```

### Person

```
Person
├── id: unique identifier
├── fullname: display name
├── aliases: alternative names
├── biography: life story
├── birthDate: date of birth
├── birthPlace: location
├── status: active/retired/deceased
├── role: actor/director/writer
└── filmography: video references
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Title Service                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Video Manager                            │ │
│  │                                                            │ │
│  │  Video ◄──► Cast ◄──► Person                              │ │
│  │    │                                                       │ │
│  │    └──► Posters ◄──► Previews                             │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Person Manager                           │ │
│  │                                                            │ │
│  │  Person ◄──► Filmography ◄──► Videos                      │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Search Integration                       │ │
│  │                                                            │ │
│  │  Index videos + persons ──▶ Full-text search              │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Video Operations

| Method | Description |
|--------|-------------|
| `CreateVideo` | Add video metadata |
| `GetVideo` | Get video by ID |
| `UpdateVideo` | Update metadata |
| `DeleteVideo` | Remove video |
| `SearchVideos` | Search videos |

### Person Operations

| Method | Description |
|--------|-------------|
| `CreatePerson` | Add person |
| `GetPerson` | Get person by ID |
| `UpdatePerson` | Update person |
| `DeletePerson` | Remove person |
| `SearchPersons` | Search people |

### Relationship Operations

| Method | Description |
|--------|-------------|
| `AddCastMember` | Link person to video |
| `RemoveCastMember` | Unlink person |
| `GetFilmography` | Get person's videos |

## Usage Examples

### Go Client

```go
import (
    title "github.com/globulario/services/golang/title/title_client"
)

client, _ := title.NewTitleService_Client("localhost:10116", "title.TitleService")
defer client.Close()

// Create person
person := &titlepb.Person{
    Id:        "person-001",
    Fullname:  "Jane Doe",
    Biography: "Award-winning actress...",
    BirthDate: "1985-03-15",
    Role:      titlepb.PersonRole_ACTOR,
    Status:    titlepb.CareerStatus_ACTIVE,
}
err := client.CreatePerson(person)

// Create video
video := &titlepb.Video{
    Id:          "video-001",
    Title:       "The Great Adventure",
    Description: "An epic journey...",
    Duration:    7200, // 2 hours
    Genre:       "Adventure",
    Rating:      "PG-13",
    ReleaseDate: "2024-06-15",
    Tags:        []string{"adventure", "drama", "epic"},
}
err = client.CreateVideo(video)

// Add cast member
err = client.AddCastMember("video-001", "person-001", "Lead Role")

// Search videos
results, err := client.SearchVideos("adventure epic")
for _, v := range results {
    fmt.Printf("%s (%s)\n", v.Title, v.ReleaseDate)
}

// Get filmography
videos, err := client.GetFilmography("person-001")
```

## Configuration

```json
{
  "port": 10116,
  "database": "titles",
  "searchEnabled": true,
  "posterPath": "/var/lib/globular/posters",
  "previewPath": "/var/lib/globular/previews"
}
```

## Dependencies

- [Persistence Service](../persistence/README.md) - Metadata storage
- [Search Service](../search/README.md) - Full-text search
- [File Service](../file/README.md) - Media asset storage

---

[Back to Services Overview](../README.md)
