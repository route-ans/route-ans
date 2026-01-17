# Usage Examples

Common use cases and code examples for Route ANS Resolver.

## Basic Resolution

### Exact Version

```bash
curl "http://localhost:8080/v1/resolve?name=mcp://chatbot.conversation.PID-5678.v1.2.3.example.com"
```

### Latest Version

```bash
curl "http://localhost:8080/v1/resolve?name=mcp://chatbot.conversation.PID-5678.v1.0.0.example.com&version=*"
```

### Version Range

```bash
# Any 1.x version
curl "http://localhost:8080/v1/resolve?name=mcp://chatbot.conversation.PID-5678.v1.0.0.example.com&version=1.x"

# Compatible with 1.2.0
curl "http://localhost:8080/v1/resolve?name=mcp://chatbot.conversation.PID-5678.v1.2.0.example.com&version=^1.2.0"

# Patch updates only
curl "http://localhost:8080/v1/resolve?name=mcp://chatbot.conversation.PID-5678.v1.2.3.example.com&version=~1.2.3"
```

## Batch Resolution

```bash
curl -X POST http://localhost:8080/v1/resolve/batch \
  -H "Content-Type: application/json" \
  -d '{
    "names": [
      "mcp://agent1.capability.PID-123.v1.0.0.example.com",
      "a2a://agent2.capability.PID-456.v2.0.0.example.com"
    ]
  }'
```

## Agent Information

### Get Details

```bash
curl "http://localhost:8080/v1/agent/mcp://chatbot.conversation.PID-5678.v1.2.3.example.com"
```

### Verify Agent

```bash
curl "http://localhost:8080/v1/agent/mcp://chatbot.conversation.PID-5678.v1.2.3.example.com/verify"
```

## Next Steps

- **[Version Negotiation](reference/version-negotiation.md)** - Version range details
- **[Architecture](architecture/overview.md)** - System design
