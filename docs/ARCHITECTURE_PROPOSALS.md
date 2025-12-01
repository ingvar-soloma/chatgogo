# Architecture & Refactoring Proposals

This document provides recommendations for architectural improvements, refactoring, and enhancing code quality for the ChatGoGo backend. The goal is to build a more scalable, maintainable, and robust application.

## 1. Configuration Management

**Problem:** Configuration values (e.g., database credentials, Redis address) are currently read directly from environment variables in multiple places. This can lead to inconsistencies and makes configuration validation difficult.

**Proposal:** Centralize application configuration into a single struct.

-   **Step 1:** Create a `config` package (`internal/config`).
-   **Step 2:** Define a `Config` struct that holds all configuration variables.
-   **Step 3:** Use a library like `envconfig` or `viper` to load environment variables into this struct at startup.
-   **Step 4:** Add validation to the `Config` struct (e.g., ensuring required variables are not empty).
-   **Step 5:** Pass the `Config` struct to the dependencies that need it, instead of having them access `os.Getenv` directly.

**Benefits:**
-   Single source of truth for all configuration.
-   Improved validation and error handling at startup.
-   Easier to manage and test.

## 2. Structured Logging

**Problem:** The current logging uses the standard `log` package, which produces unstructured, plain-text logs. This makes logs difficult to parse, filter, and analyze, especially in a production environment.

**Proposal:** Adopt a structured logging library like `slog` (available in Go 1.21+), `logrus`, or `zap`.

-   **Step 1:** Choose a logging library (`slog` is recommended for modern Go projects).
-   **Step 2:** Replace all `log.Printf`, `log.Println`, and `log.Fatalf` calls with the structured logger.
-   **Step 3:** Add context to log messages (e.g., `userID`, `roomID`, `handlerName`).

**Example (using `slog`):**
```go
// Before
log.Printf("Client registered: %s", client.GetUserID())

// After
slog.Info("client registered", "userID", client.GetUserID())
```

**Benefits:**
-   Logs are machine-readable (e.g., JSON format).
-   Enables powerful querying and filtering in log management systems (e.g., Grafana Loki, ELK Stack).
-   Improves observability and makes debugging significantly easier.

## 3. Decouple Matchmaking Logic

**Problem:** The `MatcherService` currently contains a hardcoded, random matchmaking algorithm. This makes it difficult to implement new matching strategies (e.g., profile-based matching) without modifying the core service.

**Proposal:** Use the Strategy design pattern to decouple the matching algorithm from the `MatcherService`.

-   **Step 1:** Define a `MatchingStrategy` interface with a `FindMatch` method.
    ```go
    type MatchingStrategy interface {
        FindMatch(currentUser models.User, candidates []models.User) (*models.User, error)
    }
    ```
-   **Step 2:** Create concrete implementations of this interface:
    -   `RandomMatchingStrategy`: The current default behavior.
    -   `ProfileBasedStrategy`: A new strategy that considers interests, age, etc.
-   **Step 3:** Modify `MatcherService` to hold a `MatchingStrategy` instance.
-   **Step 4:** The `MatcherService`'s `FindMatch` method will delegate the core logic to the current strategy.

**Benefits:**
-   **Flexibility:** Easily switch between matching algorithms, even at runtime.
-   **Extensibility:** Add new matching strategies without changing existing code.
-   **Testability:** Test matching algorithms in isolation.

## 4. Enhanced Testing Strategy

**Problem:** While tests exist, coverage can be improved, and there's a lack of integration tests that verify the interaction between different components (e.g., `Hub` -> `Matcher` -> `Storage`).

**Proposal:** Expand and formalize the testing strategy.

-   **Unit Tests:**
    -   Continue adding unit tests for individual functions and methods.
    -   Aim for higher test coverage, especially for critical business logic in `chathub` and `storage`.
    -   Use mocks for external dependencies (database, Redis) consistently. The existing `storage.Storage` interface is a great foundation for this.

-   **Integration Tests:**
    -   Create a new test suite for integration tests (e.g., using Go's build tags to separate them: `//go:build integration`).
    -   These tests should spin up real dependencies (e.g., a test database and Redis in Docker containers) using a library like `testcontainers-go`.
    -   Write tests that simulate a full user journey:
        1.  User A sends `/start`.
        2.  User B sends `/start`.
        3.  Verify that a room is created in the database.
        4.  User A sends a message.
        5.  Verify that User B receives the message.
        6.  User B sends `/stop`.
        7.  Verify the room is closed.

**Benefits:**
-   Increased confidence in code changes.
-   Early detection of regressions and bugs.
-   Ensures that different parts of the system work together as expected.

## 5. Graceful Shutdown

**Problem:** The application currently terminates abruptly (`log.Fatal`). In a production environment, this can lead to data loss or dropped connections.

**Proposal:** Implement a graceful shutdown mechanism.

-   **Step 1:** Use an `os.Signal` channel to listen for interrupt signals (`SIGINT`, `SIGTERM`).
-   **Step 2:** When a signal is received, trigger a shutdown sequence:
    -   Stop accepting new connections.
    -   Notify active clients of the impending shutdown.
    -   Wait for active processes (like saving messages) to complete.
    -   Close database and Redis connections cleanly.
-   **Step 3:** Use a `context` with a timeout to ensure the shutdown process doesn't hang indefinitely.

**Benefits:**
-   Prevents data corruption.
-   Improves the user experience by avoiding abrupt disconnections.
-   Essential for zero-downtime deployments.
