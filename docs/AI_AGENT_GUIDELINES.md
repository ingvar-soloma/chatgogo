# AI Agent Development Guidelines

This document provides a set of guidelines and best practices for AI agents (like Jules) working on the ChatGoGo codebase. Adhering to these standards will ensure consistency, maintainability, and high quality of code.

## 1. Golden Rule: Always Verify

After every action that modifies the codebase (e.g., creating, editing, or deleting a file), you **must** use a read-only tool (like `read_file`, `ls`, or `grep`) to confirm that the action was executed successfully and had the intended effect. Do not mark a plan step as complete until you have verified the outcome.

## 2. Development Workflow

Follow this step-by-step process for all tasks (new features, bug fixes, refactoring).

1.  **Understand the Goal:** Read the task description carefully. If you have any doubts, ask clarifying questions before starting.
2.  **Explore the Code:** Use tools like `list_files` and `read_file` to understand the relevant parts of the codebase. Pay special attention to the `docs/` directory, especially `ARCHITECTURE.md` and `LLM_CONTEXT_INDEX.yaml`.
3.  **Formulate a Plan:** Create a detailed, step-by-step plan using `set_plan`. The plan must include a step for testing your changes.
4.  **Implement the Changes:** Write the code, following the standards outlined below.
5.  **Run Tests:** Execute the relevant tests to ensure your changes are correct and have not introduced any regressions.
    -   Run all tests: `go test ./... -v`
    -   Generate a coverage report to ensure new code is tested: `go test ./... -coverprofile=coverage.out`
6.  **Update Documentation:** If your changes affect the architecture, data models, or user-facing features, you **must** update the relevant documentation (`README.md`, `ARCHITECTURE.md`, etc.).
7.  **Submit for Review:** Once all steps are complete and verified, submit your changes with a clear and descriptive commit message.

## 3. Code and Documentation Standards

### Go Code Style

-   **Formatting:** All Go code must be formatted with `gofmt`. Run `go fmt ./...` before submitting.
-   **GoDoc:** All public functions, methods, types, and constants must have clear and descriptive GoDoc comments. Explain what the component does, its parameters, and what it returns.
-   **Error Handling:** Handle all errors. Do not use blank identifiers (`_`) to discard errors unless there is a very specific and justified reason. When returning errors, add context using `fmt.Errorf("...: %w", err)`.
-   **Structured Logging:** Use a structured logger (`slog`) for all logging. Avoid using the standard `log` package. Include relevant context in your log messages.

### Documentation

-   **`LLM_CONTEXT_INDEX.yaml`:** This file is a machine-readable index of the codebase. If you add a new service, package, or critical component, you **must** add a corresponding entry to this file.
-   **`README.md`:** If you introduce a change that affects the setup, configuration, or how to run the project, update the `README.md` accordingly.
-   **Clarity and Conciseness:** Write clear and easy-to-understand documentation.

## 4. Testing Requirements

-   **Test-Driven Development (TDD):** For new features, practice TDD when possible. Write a failing test first, then write the code to make it pass.
-   **Unit Tests:** All new business logic must be accompanied by unit tests. Use mocks for external dependencies (e.g., the `storage.Storage` interface).
-   **Integration Tests:** For changes that affect multiple components, consider adding an integration test to verify the end-to-end functionality.
-   **No Regressions:** Before submitting, ensure that **all** existing tests pass. A change that breaks an existing test will be rejected.

## 5. Agent's Checklist Before Submission

Before using the `submit` tool, confirm the following:

-   [ ] All my code changes have been implemented as per the plan.
-   [ ] I have verified every file modification.
-   [ ] The code is formatted correctly (`go fmt ./...`).
-   [ ] I have added GoDoc comments to all new public symbols.
-   [ ] I have written unit tests for my new code.
-   [ ] All tests pass (`go test ./...`).
-   [ ] I have updated all relevant documentation (`README.md`, `ARCHITECTURE.md`, `LLM_CONTEXT_INDEX.yaml`).
-   [ ] My commit message is clear and descriptive.
