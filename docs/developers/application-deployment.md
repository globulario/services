# Application Deployment Model

This page covers how web applications are packaged and deployed on Globular. Unlike services (which are gRPC servers with their own ports), applications are web frontends served through the Envoy gateway via gRPC-Web protocol translation.

## What is an Application

In Globular, an **application** is a web frontend (HTML, JavaScript, CSS) that communicates with backend services via gRPC-Web. Applications:

- Do not have their own network port
- Are served through the Envoy gateway (ports 443/8443)
- Use gRPC-Web clients (generated from proto files) to call backend services
- Have their own RBAC roles and group associations
- Are packaged as APPLICATION kind packages

### Applications vs Services

| Aspect | Service | Application |
|--------|---------|-------------|
| **Kind** | SERVICE | APPLICATION |
| **Runtime** | Native binary, systemd unit | Static files served by gateway |
| **Port** | Own gRPC port | None (uses gateway) |
| **Protocol** | Native gRPC | gRPC-Web (via gateway) |
| **Client** | Go/gRPC client | TypeScript/gRPC-Web client |
| **Health check** | gRPC health endpoint | HTTP health check (if applicable) |
| **Process** | Long-running server process | No process (static files) |

## Application Package Structure

```
globular-myapp-1.0.0-linux_amd64-1.tgz
├── webapp/
│   ├── index.html              # Entry point
│   ├── app.js                  # Compiled JavaScript
│   ├── app.css                 # Styles
│   └── assets/                 # Images, fonts, etc.
├── specs/
│   └── myapp_application.yaml  # Application metadata
└── lib/                        # Optional
    └── config.json             # Default configuration
```

### Application Spec File

```yaml
# specs/myapp_application.yaml
name: myapp
version: 1.0.0
publisher: myteam@example.com
platform: linux_amd64
kind: APPLICATION
profiles:
  - gateway
priority: 80
```

## Using gRPC-Web Clients

### TypeScript Client Library

Globular generates TypeScript gRPC-Web clients for all services. These are in the `typescript/` directory:

```
typescript/
├── authentication/
│   ├── authentication_pb.js          # Message classes
│   └── authentication_grpc_web_pb.js # gRPC-Web client
├── inventory/
│   ├── inventory_pb.js
│   └── inventory_grpc_web_pb.js
└── ... (all services)
```

### Client Usage

```typescript
import { InventoryServiceClient } from './inventory/inventory_grpc_web_pb';
import { GetAssetRequest, CreateAssetRequest } from './inventory/inventory_pb';

// Connect to the gateway endpoint
const client = new InventoryServiceClient('https://mycluster.local:8443');

// Set authentication token
const metadata = { 'token': 'eyJhbGci...' };

// Call an RPC
const request = new GetAssetRequest();
request.setAssetId('asset-001');

client.getAsset(request, metadata, (err, response) => {
    if (err) {
        console.error('RPC failed:', err.message);
        return;
    }
    console.log('Asset:', response.getName(), response.getLocation());
});

// Create an asset
const createReq = new CreateAssetRequest();
createReq.setName('New Laptop');
createReq.setCategory('hardware');
createReq.setLocation('office-1');

client.createAsset(createReq, metadata, (err, response) => {
    if (err) {
        console.error('Create failed:', err.message);
        return;
    }
    console.log('Created asset:', response.getAsset().getId());
});
```

### Content Types

gRPC-Web supports two content types:
- `application/grpc-web+proto` — Binary protobuf encoding (default, most efficient)
- `application/grpc-web+json` — JSON encoding (useful for debugging)

### Authentication in the Browser

Browser applications authenticate by:
1. Calling the Authentication service's `Authenticate` RPC with username/password
2. Receiving a JWT token in the response
3. Including the token in the `token` metadata header for all subsequent RPCs

```typescript
import { AuthenticationServiceClient } from './authentication/authentication_grpc_web_pb';
import { AuthenticateRqst } from './authentication/authentication_pb';

const authClient = new AuthenticationServiceClient('https://mycluster.local:8443');

const authReq = new AuthenticateRqst();
authReq.setName('alice');
authReq.setPassword('...');

authClient.authenticate(authReq, {}, (err, response) => {
    if (err) {
        console.error('Auth failed:', err.message);
        return;
    }
    // Store token for subsequent requests
    const token = response.getToken();
    sessionStorage.setItem('globular_token', token);
});
```

## Building an Application

### Step 1: Create Your Web Frontend

Use any web framework (React, Vue, Angular, vanilla JS). The only requirement is that it uses the generated gRPC-Web clients to communicate with backend services.

### Step 2: Compile and Bundle

```bash
# Example with a bundler (webpack, vite, etc.)
npm run build
# Output in dist/ directory
```

### Step 3: Create the Package

```bash
# Prepare payload
mkdir -p packages/payload/myapp/webapp
mkdir -p packages/payload/myapp/specs
cp -r dist/* packages/payload/myapp/webapp/
cp specs/myapp_application.yaml packages/payload/myapp/specs/

# Build package
globular pkg build \
  --spec specs/myapp_application.yaml \
  --root packages/payload/myapp/ \
  --version 1.0.0
```

### Step 4: Publish and Deploy

```bash
# Publish
globular pkg publish globular-myapp-1.0.0-linux_amd64-1.tgz

# Deploy
globular services desired set myapp 1.0.0

# The gateway serves the application at its configured path
```

## Application Discovery

When an application is published, it's registered in the Discovery service with metadata:
- Application name and version
- Associated roles and groups (for access control)
- Publisher identity

The Discovery service's `PublishApplication` RPC records this metadata, making the application discoverable by the gateway and admin interfaces.

## RBAC for Applications

Applications have their own RBAC integration:
- **Roles**: Define who can access the application
- **Groups**: Associate the application with user groups
- **Resource paths**: Application-level resources follow the same `/app/{app_name}/...` hierarchy

```bash
# Grant a group access to the application
globular rbac set-permission \
  --subject group:engineering \
  --resource "/applications/myapp" \
  --permission read
```

## Practical Scenarios

### Scenario 1: Deploying an Admin Dashboard

```bash
# Build the dashboard
cd admin-dashboard
npm install && npm run build

# Package
mkdir -p ../packages/payload/admin_dashboard/{webapp,specs}
cp -r dist/* ../packages/payload/admin_dashboard/webapp/
cat > ../packages/payload/admin_dashboard/specs/admin_dashboard_application.yaml << 'EOF'
name: admin_dashboard
version: 1.0.0
publisher: core@globular.io
platform: linux_amd64
kind: APPLICATION
profiles:
  - gateway
priority: 90
EOF

cd ..
globular pkg build --spec specs/admin_dashboard_application.yaml --root packages/payload/admin_dashboard/ --version 1.0.0
globular pkg publish globular-admin_dashboard-1.0.0-linux_amd64-1.tgz
globular services desired set admin_dashboard 1.0.0
```

### Scenario 2: Updating an Application

```bash
# Build new version
cd admin-dashboard
npm run build

# Package and publish
globular pkg build --spec specs/admin_dashboard_application.yaml --root packages/payload/admin_dashboard/ --version 1.1.0
globular pkg publish globular-admin_dashboard-1.1.0-linux_amd64-1.tgz

# Deploy — the gateway serves the new version after convergence
globular services desired set admin_dashboard 1.1.0
```

## What's Next

- [Workflow Integration](developers/workflow-integration.md): Custom workflow steps and hooks
- [Writing a Microservice](developers/writing-a-microservice.md): Backend service development
