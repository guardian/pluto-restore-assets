# Pluto Restore Assets

## Overview

Pluto Restore Assets is a service designed to manage and monitor the restoration of assets from AWS S3 Glacier storage. It consists of two main components: an API server and a worker service, both running in Kubernetes.

## Features

- **API Server**: RESTful service listening on port 9000 for restore requests
- **Worker Service**: Handles the actual restoration process with AWS S3
- **Kubernetes Integration**: Runs as containerized services with proper RBAC
- **AWS S3 Integration**: Manages asset restoration from Glacier
- **Logging**: Comprehensive request and operation logging
- **Cost Estimation**: Provides cost estimates for Standard and Bulk retrievals

## Environment Variables

The application requires the following environment variables:

- `KUBE_NAMESPACE`: Kubernetes namespace (default: "default")
- `ASSET_BUCKET_LIST`: Comma-separated list of asset buckets
- `MANIFEST_BUCKET`: S3 bucket for storing manifests
- `AWS_ROLE_ARN`: AWS role ARN for permissions
- `AWS_ACCESS_KEY_ID`: AWS access key ID
- `AWS_SECRET_ACCESS_KEY`: AWS secret access key
- `AWS_DEFAULT_REGION`: AWS region
- `WORKER_IMAGE`: Docker image for the worker service
- `BASE_PATH`: Base path for local assets
- `SMTP_HOST`: SMTP server hostname
- `SMTP_PORT`: SMTP server port
- `SMTP_FROM`: Email sender address
- `NOTIFICATION_EMAIL`: Email recipient for notifications
- `PLUTO_PROJECT_URL`: Base URL for project references

## API Endpoints

- **POST /api/v1/restore**: Create a new restore job
  - Required fields: `id`, `user`, `path`, `retrievalType`
- **GET /api/v1/restore/{id}**: Get status of a restore job
- **GET /health**: Health check endpoint

## Code Structure

### API Server (`cmd/api/`)
- `main.go`: Server initialization and routing
- `handlers/`: Request handlers and interfaces
  - `restore.go`: Main restore endpoint logic
  - `interfaces.go`: Interface definitions
  - `restore_test.go`: Handler unit tests

### Worker Service (`cmd/worker/`)
- `main.go`: Worker process implementation
- Handles AWS S3 interactions and restore operations

### Internal Packages
- `internal/s3utils/`: AWS S3 utility functions
  - `manifest.go`: Manifest generation
  - `monitor.go`: Restore status monitoring
  - `upload.go`: S3 upload operations
- `internal/types/`: Shared type definitions
- `pkg/kubernetes/`: Kubernetes integration

## Testing

The project includes comprehensive tests:

- API Handler Tests: `cmd/api/handlers/restore_test.go`
- S3 Utility Tests: `internal/s3utils/monitor_test.go`
- Manifest Generation Tests: `internal/s3utils/manifest_test.go`

## Building and Running

```bash
make deploy-latest
```

## Kubernetes Configuration

The service uses several Kubernetes resources:

### Deployments
- API Server deployment
- Worker deployment for handling restore jobs

### Service
Exposes the API server on port 9000

### RBAC
- `job-creator-role.yaml`: Defines permissions
- `job-creator-rolebinding.yaml`: Binds role to service account

## AWS S3 Glacier Restore Costs

Summary for 1000 objects totaling 1TB:

| Retrieval Option | Retrieval Time | Total Cost |
| ---------------- | -------------- | ---------- |
| Expedited        | 1–5 minutes    | $40.72     |
| Standard         | 3–5 hours      | $10.29     |
| Bulk             | 5–12 hours     | $2.59      |
