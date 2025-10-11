# go-project

A small Go HTTP application scaffold.

## Overview

- Module: `github.com/tsatsarig/go-project`
- Go version: 1.24.4 (as defined in `go.mod`)

## Prerequisites

- Go (recommended version 1.24.x)

Verify your Go version:

```bash
go version
```

## Build

Build a binary in the project root:

```bash
go build -o bin/server .
```

Run the binary:

```bash
./bin/server -port=8080
```

You can also run directly with `go run` while developing:

```bash
go run main.go -port=8080
```

## Usage

The server listens on the port provided by the `-port` flag (default: `8080`).

Health endpoint:

```bash
curl -i http://localhost:8080/health
```

Response:

```
HTTP/1.1 200 OK

OK
```

Logs are written to stdout by the internal logger found in `internal/app`.

## Project layout

```
.
├── go.mod
├── main.go            # application entrypoint
└── internal
    ├── app
    │   └── app.go     # application initializer (logger, config)
    └── routes
        └── routes.go  # route definitions (currently empty)
```

## Extending the project

- Add route handlers inside `internal/routes` and wire them up in `main.go` or an HTTP router (e.g., `net/http`, `gorilla/mux`, `chi`).
- Add graceful shutdown handling: trap signals and call `server.Shutdown(ctx)` to allow in-flight requests to finish.
- Add configuration (environment variables or config file) and dependency injection for easier testing.

## Tests

There are no tests in the repository yet. Consider adding unit tests for handlers and integration tests for the HTTP endpoints.

## Contributing

1. Fork the repository.
2. Create a feature branch: `git checkout -b feat/your-feature`.
3. Make changes and add tests.
4. Open a pull request describing your changes.

## License

This repository is licensed under a simple "Viewing Only License". See the `LICENSE` file for the full text.

In short:

- You may view and read the source for personal or educational purposes.
- You may not copy, distribute, publish, modify, or use the code in production or commercial settings without prior written permission from the repository owner.
