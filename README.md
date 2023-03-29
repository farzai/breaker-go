# Circuit Breaker in Golang

[![Go Reference](https://pkg.go.dev/badge/github.com/farzai/breaker-go.svg)](https://pkg.go.dev/github.com/farzai/breaker-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/farzai/breaker-go)](https://goreportcard.com/report/github.com/farzai/breaker-go)
![Github Actions](https://github.com/farzai/breaker-go/actions/workflows/ci.yaml/badge.svg?branch=main)
[![codecov](https://codecov.io/gh/farzai/breaker-go/branch/main/graph/badge.svg)](https://codecov.io/gh/farzai/breaker-go)

This repository contains a simple implementation of the Circuit Breaker design pattern in Golang. The implementation follows clean architecture principles and is designed to be easy to unit test.

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
import "breaker"

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

## Testing
Unit tests for the Circuit Breaker can be found in the breaker_test.go file. To run the tests, use the following command:

```bash
go test
```

## License
This project is licensed under the MIT License - see the [MIT license](LICENSE) for details.
