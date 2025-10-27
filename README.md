# Circuit Breaker in Golang

[![Go Reference](https://pkg.go.dev/badge/github.com/farzai/breaker-go.svg)](https://pkg.go.dev/github.com/farzai/breaker-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/farzai/breaker-go)](https://goreportcard.com/report/github.com/farzai/breaker-go)
![Github Actions](https://github.com/farzai/breaker-go/actions/workflows/ci.yaml/badge.svg?branch=main)
[![codecov](https://codecov.io/gh/farzai/breaker-go/branch/main/graph/badge.svg)](https://codecov.io/gh/farzai/breaker-go)

This repository contains a simple implementation of the Circuit Breaker in Golang.

## Overview

The Circuit Breaker pattern is used to protect a system from failures in its external dependencies. It helps to improve the system's resiliency by preventing cascading failures and allowing the system to recover gracefully when a dependency fails.

The Circuit Breaker implementation in this repository has three main states:

- Closed: The external dependency is considered healthy, and all requests are executed as expected.
- Open: The external dependency is considered unhealthy, and no requests are executed to prevent further failures.
- Half-Open: The external dependency is being tested for healthiness, and a limited number of requests are executed to determine if the dependency has recovered.


## Installation

```bash
go get -u github.com/farzai/breaker-go
```

## Usage

To use the Circuit Breaker, first import the `breaker` package:

```go
import "github.com/farzai/breaker-go"

func main() {
    // Create a new Circuit Breaker instance with the desired failure threshold and reset timeout:
    cb := breaker.NewCircuitBreaker(3, 5*time.Second)

    // Wrap any function that communicates with an external dependency using the Execute method of the Circuit Breaker:
    result, err := cb.Execute(func() (interface{}, error) {
        // Code to interact with the external dependency
    })

    // Handle the result of the function call
}
```

### State Persistence

The Circuit Breaker supports persisting its state to survive application restarts. You can choose between in-memory storage (default) or file-based storage.

#### In-Memory Storage (Default)

By default, the circuit breaker uses in-memory storage, which is fast but doesn't persist across restarts:

```go
cb := breaker.NewCircuitBreaker(3, 5*time.Second)
```

#### File-Based Storage

For persistent state storage, use the file-based repository:

```go
// Option 1: Use a temporary file (recommended for most cases)
repo, err := breaker.NewTempFileSnapshotRepository()
if err != nil {
    log.Fatal(err)
}

cb := breaker.NewCircuitBreaker(
    3,
    5*time.Second,
    breaker.WithSnapshotRepository(repo),
)

// Option 2: Specify a custom file path
repo, err := breaker.NewFileSnapshotRepository("/var/app/circuit_breaker.json")
if err != nil {
    log.Fatal(err)
}

cb := breaker.NewCircuitBreaker(
    3,
    5*time.Second,
    breaker.WithSnapshotRepository(repo),
)

// Option 3: Use advanced options
repo, err := breaker.NewFileSnapshotRepositoryWithOptions(breaker.FileSnapshotRepositoryOptions{
    DirPath:  "/var/app/state",
    FileName: "circuit_breaker_snapshot.json",
})
if err != nil {
    log.Fatal(err)
}

cb := breaker.NewCircuitBreaker(
    3,
    5*time.Second,
    breaker.WithSnapshotRepository(repo),
)
```

The file-based repository provides:
- **Atomic writes**: State changes are written atomically to prevent corruption
- **Thread-safe operations**: Safe for concurrent use
- **Automatic directory creation**: Parent directories are created automatically
- **JSON serialization**: Human-readable snapshot format

## Logging & Observability

The circuit breaker includes comprehensive structured logging support for production observability.

### Basic Logging

```go
import "github.com/farzai/breaker-go/logging"

// Create a logger with INFO level
logger := logging.NewDefaultLogger(logging.LevelInfo)

cb, err := breaker.New(
    breaker.WithFailureThreshold(3),
    breaker.WithResetTimeout(5*time.Second),
    breaker.WithLogger(logger),
)
```

### Log Levels

- **Debug**: Detailed execution traces, persistence operations
- **Info**: State transitions, important events
- **Warn**: Circuit breaker blocking requests
- **Error**: Operation failures, persistence errors

### Quick Setup

```go
// Convenience option for quick setup
cb, err := breaker.New(
    breaker.WithLogLevel(logging.LevelInfo),
)
```

### Integration with Popular Loggers

**Go's standard slog (Go 1.21+):**

```go
import "log/slog"

handler := slog.NewJSONHandler(os.Stdout, nil)
slogger := slog.New(handler)
logger := logging.NewSlogAdapter(slogger)

cb, err := breaker.New(
    breaker.WithLogger(logger),
)
```

### Advanced Configuration

**Builder pattern:**

```go
logger := logging.NewBuilder().
    WithLevel(logging.LevelInfo).
    WithFields(
        logging.String("service", "api"),
        logging.String("version", "1.0.0"),
    ).
    Build()

cb, err := breaker.New(
    breaker.WithLogger(logger),
)
```

**Separate persistence logger:**

```go
mainLogger := logging.NewDefaultLogger(logging.LevelInfo)
persistLogger := logging.NewDefaultLogger(logging.LevelDebug)

cb, err := breaker.New(
    breaker.WithLogger(mainLogger),
    breaker.WithPersistenceLogger(persistLogger),
)
```

### What Gets Logged

- ✅ State transitions (Closed → Open → HalfOpen)
- ✅ Execution attempts and results
- ✅ Persistence operations (save/load with timing)
- ✅ Manual reset operations
- ✅ Configuration validation errors
- ✅ Retry attempts with delays

### Examples

See the `examples/` directory for complete logging examples:
- `logging_basic.go` - Basic logging setup
- `logging_advanced.go` - Custom adapters and patterns

## Testing
Unit tests for the Circuit Breaker can be found in the breaker_test.go file. To run the tests, use the following command:

```bash
go test
```

## License
This project is licensed under the MIT License - see the [MIT license](LICENSE) for details.
