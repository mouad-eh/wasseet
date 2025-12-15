# Wasseet

**Wasseet (وَسيط)** is an HTTP reverse proxy built in Go. It uses the already existing HTTP server implementation from the `net/http` package and does not implement an HTTP server from scratch tuned for proxying.

**Implemented features:** Backend groups, Load balancing, Request and response rewriting, Hot configuration reloading and Backend health checks.

**Upcoming features:** Rate limiting, more load balancing algorithms, more request and response operations ...etc.

**Config example:**

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