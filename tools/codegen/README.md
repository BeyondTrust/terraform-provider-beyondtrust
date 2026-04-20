# Terraform Provider Code Generator

This directory contains tooling for auto-generating Terraform provider code from the Workload Credentials OpenAPI specification.

## Overview

The code generator reads the OpenAPI spec and generates:
- Resource files (`secrets/resources/*.go`)
- Data source files (`secrets/datasources/*.go`)
- Documentation (`docs/**/*.md`)

## Usage

### Generate All Resources

```bash
# From the repository root
make generate
```

### Generate Specific Resource

```bash
go run tools/codegen/main.go generate \
  --spec <path-to-openapi-spec> \
  --resource folders \
  --output secrets/resources/folder_resource.go
```

## Features

The generator handles Workload Credentials-specific patterns:
- Path-based resources (name + folder query parameter)
- Merge patch semantics (RFC 7396)
- Soft deletes
- CSRF token requirements

## Testing Generated Code

```bash
make validate-generated
```
