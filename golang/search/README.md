# Search Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Search Service provides full-text search indexing and retrieval capabilities for Globular applications.

## Overview

This service enables fast, accurate full-text search across documents with support for multiple languages, snippet extraction, and per-field indexing.

## Features

- **Full-Text Indexing** - Index JSON documents for search
- **Multi-Language Support** - Language-aware tokenization
- **Snippet Extraction** - Highlight matching text
- **Streaming Results** - Efficient pagination
- **Per-Field Indexing** - Index specific document fields

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       Search Service                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                     Indexer                                │ │
│  │                                                            │ │
│  │   Document ──▶ Tokenizer ──▶ Analyzer ──▶ Index Store     │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                     Query Engine                           │ │
│  │                                                            │ │
│  │   Query ──▶ Parser ──▶ Searcher ──▶ Ranker ──▶ Results    │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                     Index Store                            │ │
│  │                                                            │ │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │ │
│  │   │  In-Memory  │  │  Persistent │  │  Bleve      │       │ │
│  │   │    Index    │  │    Index    │  │   Index     │       │ │
│  │   └─────────────┘  └─────────────┘  └─────────────┘       │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Methods

| Method | Description | Parameters |
|--------|-------------|------------|
| `IndexJsonObject` | Index a JSON document | `database`, `id`, `data`, `language`, `fields` |
| `SearchDocuments` | Search indexed documents | `database`, `query`, `offset`, `size` |
| `DeleteDocument` | Remove from index | `database`, `id` |
| `Count` | Count documents | `database` |
| `GetEngineVersion` | Get search engine version | - |

## Usage Examples

### Go Client

```go
import (
    search "github.com/globulario/services/golang/search/search_client"
)

client, _ := search.NewSearchService_Client("localhost:10110", "search.SearchService")
defer client.Close()

// Index a document
doc := map[string]interface{}{
    "title":   "Introduction to Go Programming",
    "content": "Go is a statically typed, compiled language...",
    "author":  "John Doe",
    "tags":    []string{"golang", "programming", "tutorial"},
}

err := client.IndexJsonObject(
    "articles",           // database
    "article-123",        // document ID
    doc,                  // document data
    "en",                 // language
    []string{"title", "content"}, // fields to index
)

// Search documents
results, err := client.SearchDocuments(
    "articles",           // database
    "Go programming",     // query
    0,                    // offset
    10,                   // page size
)

for result := range results {
    fmt.Printf("ID: %s, Score: %f\n", result.Id, result.Score)
    fmt.Printf("Snippet: %s\n", result.Snippet)
}

// Delete document
err = client.DeleteDocument("articles", "article-123")

// Get document count
count, err := client.Count("articles")
fmt.Printf("Total documents: %d\n", count)
```

### Search with Snippets

```go
// Search returns snippets with highlighted matches
results, _ := client.SearchDocuments("articles", "golang tutorial", 0, 10)

for result := range results {
    // Snippet contains surrounding text with match highlighted
    fmt.Println(result.Snippet)
    // Output: "...Introduction to <mark>Go</mark> Programming is a <mark>tutorial</mark>..."
}
```

## Configuration

### Configuration File

```json
{
  "port": 10110,
  "indexPath": "/var/lib/globular/search",
  "inMemory": false,
  "defaultLanguage": "en",
  "snippetSize": 200
}
```

## Supported Languages

| Language | Code |
|----------|------|
| English | `en` |
| French | `fr` |
| German | `de` |
| Spanish | `es` |
| Portuguese | `pt` |

## Integration

Used by:
- [Blog Service](../blog/README.md) - Post search
- [Title Service](../title/README.md) - Media search
- [File Service](../file/README.md) - Document search

---

[Back to Services Overview](../README.md)
