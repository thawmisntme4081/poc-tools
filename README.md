# Project StockMind

StockMind is an AI-powered assistant designed to simplify access to financial information and insights about the Vietnamese stock market. By combining financial data retrieval with intelligent analysis, StockMind helps investors and analysts make informed decisions more efficiently.

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.

## sqlc golang

Compile SQL code

```bash
sqlc generate
```

## MakeFile

Run build make command with tests

```bash
make all
```

Build the application

```bash
make build
```

Run the application

```bash
make run
```

Create DB container

```bash
make docker-run
```

Shutdown DB Container

```bash
make docker-down
```

DB Integrations Test:

```bash
make itest
```

Live reload the application:

```bash
make watch
```

Run the test suite:

```bash
make test
```

Clean up binary from the last build:

```bash
make clean
```
