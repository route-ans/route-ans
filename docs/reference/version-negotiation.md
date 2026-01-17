# Version Negotiation

Semantic version negotiation for ANS agent resolution.

## Overview

Version negotiation allows clients to request compatible agent versions using semantic version ranges instead of exact versions. The resolver automatically selects the highest matching version from available agents.

## Version Range Formats

| Format | Example | Meaning |
|--------|---------|---------|
| **Exact** | `1.2.3` | Exact version 1.2.3 |
| **Caret** | `^1.2.3` | Compatible versions (≥1.2.3, <2.0.0) |
| **Tilde** | `~1.2.3` | Patch updates only (≥1.2.3, <1.3.0) |
| **Wildcard** | `1.x` | Any version in major release |
| **Latest** | `*` | Latest available version |
| **Comparisons** | `>=1.0.0`, `<2.0.0` | Version constraints |

### Caret Ranges (Recommended)

- `^1.2.3` → Compatible with 1.2.3 (allows 1.x.x, but not 2.0.0)
- `^0.2.3` → Compatible with 0.2.3 (allows 0.2.x, but not 0.3.0)
- `^0.0.3` → Exact 0.0.3 (early development, no compatibility assumed)

### Tilde Ranges

- `~1.2.3` → Patch-level updates (1.2.3, 1.2.4, 1.2.5, but not 1.3.0)

## Usage

Add `version` parameter to resolution requests:

```
GET /v1/resolve?name=mcp://agent.example.com&version=^1.0.0
```

**Selection Strategy**: When multiple versions match, the **highest** version is selected.

## Examples

**Request latest compatible version:**
```
GET /v1/resolve?name=mcp://agent.example.com&version=^1.0.0
```
Returns: `1.5.2` (if available and compatible)

**Request patch updates only:**
```
GET /v1/resolve?name=mcp://agent.example.com&version=~1.2.0
```
Returns: `1.2.5` (highest patch in 1.2.x)

**Request any version in major release:**
```
GET /v1/resolve?name=mcp://agent.example.com&version=1.x
```
Returns: `1.9.3` (highest 1.x version)

**Request latest version:**
```
GET /v1/resolve?name=mcp://agent.example.com&version=*
```
Returns: Latest registered version

## See Also

- [ANS Specification](../architecture/ans-spec.md) - ANS name format and versioning
- [Semantic Versioning 2.0.0](https://semver.org/) - Version semantics
- [npm semver](https://docs.npmjs.com/cli/v6/using-npm/semver) - Range syntax reference
