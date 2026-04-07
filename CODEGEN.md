# Code Generation Guide

This document explains how to use the code generation tools for the BeyondTrust Terraform Provider.

## Overview

The provider includes code generation tools that automatically create Terraform resource implementations from the SMOP OpenAPI specification.

## Quick Start

```bash
# Generate all resources and data sources
make generate

# Validate generated code
make validate-generated
```

## What Gets Generated?

1. **Resource Files** (`secrets/resources/*.go`)
   - Resource struct definition
   - Schema definition with all attributes
   - CRUD operations
   - Import logic

2. **Data Source Files** (`secrets/datasources/*.go`)
   - Data source struct definition
   - Schema definition
   - Read operation

3. **Documentation** (`docs/**/*.md`)
   - Resource/data source documentation
   - Examples
   - Import instructions

## Generator Features

### Automatic Field Mapping

| OpenAPI Type | Terraform Type  |
|--------------|-----------------|
| string       | StringAttribute |
| integer      | Int64Attribute  |
| boolean      | BoolAttribute   |
| array        | ListAttribute   |
| object       | MapAttribute    |

### SMOP Pattern Recognition

The generator recognizes SMOP-specific patterns:
- Path-based resources (name + folder query parameter)
- Merge patch semantics (RFC 7396)
- Soft deletes (permanent query parameter)
- CSRF token requirements

## Workflow

### Adding a New Resource

1. Ensure OpenAPI spec is up-to-date
2. Generate the resource: `make generate-folder`
3. Review generated code
4. Add custom logic if needed
5. Test: `make test-acc`

### When OpenAPI Changes

1. Pull latest OpenAPI spec
2. Regenerate code: `make generate`
3. Review diffs
4. Update tests

## Best Practices

1. Don't edit generated files directly
2. Use version control to track changes
3. Mark custom sections clearly
4. Validate after generation

## Troubleshooting

### Generator Fails to Parse OpenAPI

- Validate OpenAPI spec
- Check for syntax errors
- Ensure all $ref references are valid

### Generated Code Doesn't Compile

- Run `make validate-generated`
- Check for missing imports
- Verify type mappings
