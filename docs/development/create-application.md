# Create an Application

## What Is a Globular Application

A Globular application is a web application that runs on the platform and uses Globular services (authentication, file storage, RBAC, etc.) via gRPC-Web through the Envoy gateway.

## Application Structure

Applications are served as static web assets through the gateway. They communicate with backend services via gRPC-Web (HTTP/2 over the Envoy proxy at port 443).

## Available Client APIs

Applications can use the TypeScript client libraries generated from proto definitions:

| Service | Client | Purpose |
|---------|--------|---------|
| Authentication | `authentication_grpc_web_pb` | Login, token management |
| RBAC | `rbac_grpc_web_pb` | Permission checks |
| File | `file_grpc_web_pb` | File upload/download |
| Resource | `resource_grpc_web_pb` | Account/group management |
| Event | `event_grpc_web_pb` | Event subscription |
| Search | `search_grpc_web_pb` | Full-text search |

## Deployment

Applications are packaged and deployed through the same package pipeline as services:

1. Build the web application (e.g., `npm run build`)
2. Package the static assets
3. Publish to the repository
4. Set desired state

## Application Management CLI

```bash
globular app install <name>
globular app uninstall <name>
globular app list
```
