# Terraform Provider Code Generator

This directory contains tooling for auto-generating Terraform provider code from the SMOP OpenAPI specification.

## Overview

The code generator reads the OpenAPI spec and generates:
- Resource files (`secrets/resources/*.go`)
- Data source files (`secrets/datasources/*.go`)
- Documentation (`docs/**/*.md`)

## Usage

### Generate All Resources

```bash
cd /Users/macole/workspace/terraform-provider-beyondtrust
make generate
```

### Generate Specific Resource

```bash
go run tools/codegen/main.go generate \
  --spec /Users/macole/workspace/platform-secrets-manager/schemas/openapi/openapi.yaml \
  --resource folders \
  --output secrets/resources/folder_resource.go
```

## Features

The generator handles SMOP-specific patterns:
- Path-based resources (name + folder query parameter)
- Merge patch semantics (RFC 7396)
- Soft deletes
- CSRF token requirements

## Testing Generated Code

```bash
make validate-generated
```
