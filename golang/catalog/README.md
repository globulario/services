# Catalog Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Catalog Service provides inventory and product catalog management for e-commerce and supply chain applications.

## Overview

This service manages product definitions, categories, suppliers, manufacturers, inventory levels, and localization - everything needed for comprehensive product catalog management.

## Features

- **Product Management** - Items with properties and variants
- **Category Hierarchy** - Nested product categories
- **Supplier Tracking** - Vendor management
- **Inventory Control** - Stock levels and reorder points
- **Localization** - Multi-language product info
- **Package Definitions** - Shipping and packaging

## Core Entities

| Entity | Description |
|--------|-------------|
| **Item** | Product with properties, pricing, and variants |
| **Category** | Hierarchical product classification |
| **PropertyDefinition** | Custom product attributes |
| **Supplier** | Vendor information |
| **Manufacturer** | Producer information |
| **Inventory** | Stock levels and safety quantities |
| **Package** | Packaging specifications |
| **Localization** | Translated content |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       Catalog Service                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Product Manager                          │ │
│  │                                                            │ │
│  │  Items ◄──► Properties ◄──► Categories                    │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Supply Chain Manager                      │ │
│  │                                                            │ │
│  │  Suppliers ◄──► Manufacturers ◄──► Inventory              │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                 Localization Manager                       │ │
│  │                                                            │ │
│  │  Entity + Language ──▶ Translated Content                 │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Item Operations

| Method | Description |
|--------|-------------|
| `CreateItem` | Create product |
| `GetItem` | Get product by ID |
| `UpdateItem` | Update product |
| `DeleteItem` | Remove product |
| `ListItems` | List all products |

### Category Operations

| Method | Description |
|--------|-------------|
| `CreateCategory` | Create category |
| `GetCategory` | Get category |
| `UpdateCategory` | Update category |
| `DeleteCategory` | Remove category |

### Inventory Operations

| Method | Description |
|--------|-------------|
| `GetInventory` | Get stock level |
| `UpdateInventory` | Update stock |
| `SetReorderPoint` | Set safety stock |

## Usage Examples

### Go Client

```go
import (
    catalog "github.com/globulario/services/golang/catalog/catalog_client"
)

client, _ := catalog.NewCatalogService_Client("localhost:10115", "catalog.CatalogService")
defer client.Close()

// Create category
category := &catalogpb.Category{
    Id:          "electronics",
    Name:        "Electronics",
    Description: "Electronic devices and accessories",
}
err := client.CreateCategory(category)

// Create item
item := &catalogpb.Item{
    Id:          "laptop-001",
    Name:        "Pro Laptop",
    Description: "High-performance laptop",
    CategoryId:  "electronics",
    Price:       1299.99,
    Currency:    "USD",
    Properties: map[string]string{
        "brand":  "TechCorp",
        "memory": "16GB",
        "storage": "512GB SSD",
    },
}
err = client.CreateItem(item)

// Update inventory
err = client.UpdateInventory("laptop-001", 50)
err = client.SetReorderPoint("laptop-001", 10)

// Add localization
localization := &catalogpb.Localization{
    EntityId: "laptop-001",
    Language: "fr",
    Name:     "Laptop Pro",
    Description: "Ordinateur portable haute performance",
}
err = client.CreateLocalization(localization)
```

## Configuration

```json
{
  "port": 10115,
  "database": "catalog",
  "defaultCurrency": "USD",
  "supportedLanguages": ["en", "fr", "es", "de"]
}
```

## Dependencies

- [Persistence Service](../persistence/README.md) - Data storage

---

[Back to Services Overview](../README.md)
