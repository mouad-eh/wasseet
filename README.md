# Wasseet

**Wasseet (وَسيط)** is a lightweight, configurable HTTP reverse proxy written in Go. It leverages the standard `net/http` package and provides features like load balancing, request/response rewriting, health checks, and hot configuration reloading.

## Features

- **Backend Groups** - Organize your backend servers into logical groups
- **Load Balancing** - Distribute traffic across backends (round-robin, least connections)
- **Request/Response Rewriting** - Modify headers, paths, and query parameters on the fly
- **Health Checks** - Automatically detect and route around unhealthy backends
- **Hot Configuration Reloading** - Update configuration without restarting the proxy
- **Rule-based Routing** - Route requests based on host and path matching

## Roadmap

- [ ] Rate limiting
- [ ] Support for more request/response operations
- [ ] More load balancing algorithms
- [ ] Circuit breaker pattern
- [ ] Metrics and monitoring

## Config example

```yaml
port: 8080
backend_groups:
  - name: backend1
    load_balancing: round_robin
    servers:
      - http://server1.com
      - server2.com
    health_check:
      path: /health
      interval: 10s
      timeout: 5s
      retries: 3
rules:
  - path: /api
    backend_group: backend1
    request_operations:
      - type: add_header
        header: X-Forwarded-For
        value: 127.0.0.1
    response_operations:
      - type: add_header
        header: X-Response-From
        value: proxy
```

## Contributing

Contributions from the community are welcome! If you have an idea for a new feature or a bug to fix, please open an issue or submit a pull request.
