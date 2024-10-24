# Pluto Restore Assets

## Overview

Pluto Restore Assets is a service designed to manage and monitor the restoration of assets from AWS S3 Glacier storage. It provides an HTTP server that listens for requests to initiate restore jobs and monitors their progress.

## Features

- **HTTP Server**: Listens on port 9000 for incoming requests to create restore jobs.
- **AWS S3 Integration**: Interacts with AWS S3 to manage asset restoration.
- **Logging**: Provides detailed logging of requests and operations.

## Environment Variables

The application relies on several environment variables for configuration:

- `KUBE_NAMESPACE`: Kubernetes namespace to use (default is "default").
- `ASSET_BUCKET_LIST`: Comma-separated list of asset buckets.
- `MANIFEST_BUCKET`: S3 bucket for storing manifests.
- `AWS_ROLE_ARN`: AWS role ARN for permissions.
- `AWS_ACCESS_KEY_ID`: AWS access key ID.
- `AWS_SECRET_ACCESS_KEY`: AWS secret access key.
- `AWS_DEFAULT_REGION`: AWS region.

## Endpoints

- **POST /createRestoreJob**: Initiates a restore job with the provided parameters.

## Code Structure

- **main.go**: Contains the main server logic and request handling.
  - [main.go](main.go)
  - [createRestoreJob function](main.go)
  - [LoggingMiddleware function](main.go)
  - [getAWSAssetPath function](main.go)

- **s3utils**: Contains utility functions for interacting with AWS S3.
  - [MonitorObjectRestoreStatus function](s3utils/monitor.go)
  - [removeDirectories function](s3utils/monitor.go)
  - [readManifestFile function](s3utils/monitor.go)
  - [checkRestoreStatus function](s3utils/monitor.go)

- **worker**: Contains the worker logic for handling restore operations.
  - [handleRestore function](worker/main.go)
  - [initiateRestore function](worker/main.go)
  - [getRestoreDetails function](worker/main.go)

## Testing

The project includes tests for various components, such as:

- **s3utils/monitor_test.go**: Tests for monitoring and manifest reading.
  - [TestCheckRestoreStatus function](s3utils/monitor_test.go)
  - [TestReadManifestFile function](s3utils/monitor_test.go)

- **s3utils/manifest_test.go**: Tests for manifest generation.
  - [TestGenerateCSVManifest function](s3utils/manifest_test.go)

## Building and Running locally

```make deploy-latest```



## Kubernetes Configuration

The project includes several Kubernetes configuration files to deploy and manage the application within a Kubernetes cluster.

### Deployment

The `deployment.yaml` file defines the deployment configuration for the `pluto-project-restore` service. It specifies the number of replicas, container image, environment variables, and ports.

### Service

The `service.yaml` file defines a Kubernetes Service to expose the `pluto-restore-assets` application on port 9000.

### Ingress

The `ingress.yaml` file configures an Ingress resource to route external HTTP requests to the appropriate services within the cluster. It includes paths for various services, including `pluto-restore-assets`.

### RBAC

The `job-creator-role.yaml` and `job-creator-rolebinding.yaml` files define the Role and RoleBinding for the `job-creator` service account, granting it permissions to manage Kubernetes jobs.

Summary of Costs for 1000 objects in Glacier totalling 1TB:

| Retrieval Option | Retrieval Time | Total Cost |
| ---------------- | -------------- | ---------- |
| Expedited        | 1–5 minutes    | $40.72     |
| Standard         | 3–5 hours      | $10.29     |
| Bulk             | 5–12 hours     | $2.59      |
