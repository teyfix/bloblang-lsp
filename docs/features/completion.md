# Feature Specification: Autocomplete Completion Feature (`completion.go`)

This document specifies the design, trigger character parsing, and context-aware filtering of the **Completion** feature (`internal/feature/completion/`).

---

## 1. Overview & Purpose

The `completion` feature provides context-aware autocomplete recommendations as the user types. By evaluating the AST node context surrounding the cursor position, it selectively presents standard functions, methods, or variable structures to avoid flooding the editor autocomplete dropdown with irrelevant options.

---

## 2. Technical Execution Flow

```mermaid
sequenceDiagram
    autonumber
    Actor->>Completion: Channel (CompletionParams, Nonce)
    Note over Completion: Step 1: Extract preceding line tokens
    
    alt Preceding token ends with "." (Method trigger)
        Note over Completion: Filter completion items to "Method" kinds
        Completion-->>Actor: CompletionList (Methods) + Nonce
    alt Preceding token ends with "@" (Metadata trigger)
        Note over Completion: Return metadata references / options
        Completion-->>Actor: CompletionList (Metadata Keys) + Nonce
    alt Preceding token ends with "$" (Variable trigger)
        Note over Completion: Return local scope variable completions
        Completion-->>Actor: CompletionList (Variables) + Nonce
    else Standard expression context
        Note over Completion: Return standard "Function" kinds
        Completion-->>Actor: CompletionList (Functions) + Nonce
    end
```

---

## 3. Implementation Specifications

### 1. Registration
* **Interested Events**: Pure synchronous channel listener.
* **Output Interface**: Returns `*protocol.CompletionList` directly over its reply channel.

### 2. Trigger Characters & Context Filtering
* **Announced trigger characters**: `.` (method trigger), `@` (metadata trigger), `$` (variable trigger).
* When autocomplete is invoked, inspect the cursor's character offsets on the current line:
  * **Method Autocomplete Context**: If the preceding characters indicate a dot-prefix (e.g. `this.`), filter the pre-built completion cache list, returning **only** items with `Kind = protocol.CompletionItemKindMethod`.
  * **Function Autocomplete Context**: If the cursor is positioned in an open statement area (no dot-prefix), filter the cache, returning **only** items with `Kind = protocol.CompletionItemKindFunction`.
  * **Variable/Metadata Context**: If typing `@` or `$`, return local reference snippets.

### 3. Pre-Compiled Autocomplete Cache
* To guarantee sub-millisecond response times, the completion feature must maintain a pre-constructed index of all Bloblang standard functions and methods.
* The index is generated once upon server handshake (`Initialize`) using the Benthos reflection environment APIs.

---

## 4. Key Constraints & Rules

### What is EXPECTED
* **Sub-millisecond responses**: Autocomplete queries must return in under 1ms. Caching must be completely offline in memory.
* **Snippets Integration**: Include standard snippet completions for common blocks (e.g., `map` definitions or `if` statements) when completing in open source scopes.

### What is FORBIDDEN
* **No AST Traversal on keystroke**: Do not perform deep recursive AST parsing during autocomplete calculations. Rely strictly on rapid, lightweight token scans of the current line text to identify the trigger character context.
* **No Mutex Locking**: The pre-built completion list is treated as an immutable shared array. No locks should be held during reads.
