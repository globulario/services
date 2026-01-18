# Blog Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Blog Service provides a complete blogging platform with content management, comments, reactions, and full-text search.

## Overview

This service enables building blog and content management features with support for multi-author publishing, categorization, commenting, and engagement tracking.

## Features

- **Post Management** - Create, edit, publish blog posts
- **Multi-Author Support** - Multiple authors per blog
- **Comment System** - Nested comment threads
- **Reactions** - Emoji-based engagement
- **Full-Text Search** - Search posts by content
- **Multi-Language** - Localized content support
- **Status Workflow** - Draft, Published, Archived

## Post Structure

```
BlogPost
├── id: unique identifier
├── title: post title
├── subtitle: optional subtitle
├── content: markdown/HTML content
├── author: author account ID
├── keywords: search tags
├── language: content language
├── status: DRAFT | PUBLISHED | ARCHIVED
├── publishedAt: publication date
├── comments: comment threads
├── reactions: emoji reactions
└── metadata: custom fields
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Blog Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    Post Manager                            │ │
│  │                                                            │ │
│  │  Create │ Update │ Publish │ Archive │ Delete             │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Engagement Manager                        │ │
│  │                                                            │ │
│  │  Comments (nested) │ Reactions (emoji) │ Views            │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Search Integration                       │ │
│  │                                                            │ │
│  │  Index posts ──▶ Full-text search ──▶ Faceted results     │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Post Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateBlogPost` | Create new post | `post` |
| `SaveBlogPost` | Update existing post | `post` |
| `DeleteBlogPost` | Remove post | `id` |
| `GetBlogPostsByAuthors` | Get posts by authors | `authorIds[]` |
| `SearchBlogPosts` | Search posts | `query`, `filters` |

### Comment Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `AddComment` | Add comment to post | `postId`, `comment` |
| `RemoveComment` | Delete comment | `postId`, `commentId` |

### Reaction Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `AddEmoji` | Add emoji reaction | `postId`, `emoji`, `userId` |
| `RemoveEmoji` | Remove reaction | `postId`, `emoji`, `userId` |

## Usage Examples

### Go Client

```go
import (
    blog "github.com/globulario/services/golang/blog/blog_client"
)

client, _ := blog.NewBlogService_Client("localhost:10112", "blog.BlogService")
defer client.Close()

// Create blog post
post := &blogpb.BlogPost{
    Title:    "Getting Started with Globular",
    Subtitle: "A comprehensive guide",
    Content:  "# Introduction\n\nGlobular is a microservices platform...",
    Author:   "author-123",
    Keywords: []string{"globular", "microservices", "tutorial"},
    Language: "en",
    Status:   blogpb.PostStatus_DRAFT,
}
err := client.CreateBlogPost(post)

// Publish post
post.Status = blogpb.PostStatus_PUBLISHED
post.PublishedAt = time.Now()
err = client.SaveBlogPost(post)

// Search posts
results, err := client.SearchBlogPosts("microservices", nil)
for _, post := range results {
    fmt.Printf("%s by %s\n", post.Title, post.Author)
}

// Add comment
comment := &blogpb.Comment{
    Id:       "comment-1",
    Author:   "user-456",
    Content:  "Great article! Very helpful.",
    PostedAt: time.Now(),
}
err = client.AddComment(post.Id, comment)

// Add reaction
err = client.AddEmoji(post.Id, "thumbsup", "user-789")

// Get posts by author
posts, err := client.GetBlogPostsByAuthors([]string{"author-123"})
```

### Comment Threading

```go
// Add reply to comment
reply := &blogpb.Comment{
    Id:       "comment-2",
    Author:   "author-123",
    Content:  "Thanks for reading!",
    ParentId: "comment-1",  // Reply to comment-1
    PostedAt: time.Now(),
}
err = client.AddComment(post.Id, reply)
```

## Configuration

### Configuration File

```json
{
  "port": 10112,
  "maxPostSize": "10MB",
  "allowedLanguages": ["en", "fr", "es", "de"],
  "searchEnabled": true,
  "commentsEnabled": true,
  "reactionsEnabled": true
}
```

## Dependencies

- [Persistence Service](../persistence/README.md) - Post storage
- [Search Service](../search/README.md) - Full-text search
- [Event Service](../event/README.md) - Notifications

---

[Back to Services Overview](../README.md)
