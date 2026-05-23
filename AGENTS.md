# Go AI Agent Guidelines

You are an AI coding assistant operating in a modern Go (1.26+) environment. Your objective is to write clean, maintainable, and idiomatic Go code tailored for high-performance backend systems and control plane services. 

## Language Features & 1.26 Idioms
- **Pointer Instantiation:** Use the 1.26 `new(expr)` built-in to instantiate pointers to literals directly, eliminating the need for custom pointer helper functions.
  - *Good:* `timeout := new(int(30))`
  - *Bad:* `t := 30; timeout := &t`
- **Error Unwrapping:** Use the type-safe `errors.AsType` introduced in Go 1.26 instead of the legacy `errors.As`.
  - *Good:* `if connErr, ok := errors.AsType[*net.OpError](err); ok { ... }`
- **Generics:** Use self-referential type constraints when building complex recursive data structures.
- **Routing & Loops:** Rely on standard `net/http` pattern matching (1.22+) and modern loop variable semantics.
- **Logging:** Default to `log/slog` for structured, leveled logging.

## Architecture & Patterns
- **Simplicity First:** Avoid over-engineering. Assume the developer is proficient in Go and values clear, readable logic over clever abstractions.
- **Context:** Always pass `context.Context` as the first parameter for functions that perform I/O or network calls. Ensure context propagation is flawless across API boundaries.
- **Cloud-Native Resilience:** Design services with graceful shutdowns, explicit timeouts, and clear readiness/liveness probes suitable for Kubernetes deployment.

## Concurrency & Memory
- Manage goroutines responsibly. Ensure every goroutine has a clear exit condition to avoid leaks (preventing flags from the 1.26 `goroutineleak` profile).
- Keep variable scopes tight so the compiler can confidently allocate slice backing stores on the stack, taking full advantage of 1.26 compiler optimizations.

## Testing
- Rely strictly on the standard `testing` package.
- Default to table-driven tests for broad coverage of edge cases.
