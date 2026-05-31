# Feature Specification: Diagnostic Feature (`diagnostic.go`)

This document specifies the design, AST evaluation steps, and Benthos compilation rules of the **Diagnostic** feature (`internal/feature/diagnostic/`).

---

## 1. Overview & Purpose

The `diagnostic` feature is responsible for compile-time validation. It runs concurrently inside the `DocumentActor` event pipeline, checking the Tree-sitter Abstract Syntax Tree (AST) for syntax errors and executing the Benthos Bloblang compiler engine to detect semantic errors, unresolved maps, or type mismatches.

---

## 2. Technical Execution Pipeline

```mermaid
sequenceDiagram
    autonumber
    Actor->>Diagnostic: ProduceAttribute(Event, Payload)
    
    Note over Diagnostic: Step 1: AST Syntax Traversal
    opt AST has syntax errors
        Note over Diagnostic: Gather ERROR/MISSING nodes
    end
    
    Note over Diagnostic: Step 2: Benthos Environment Parse
    opt Syntax is clean
        Diagnostic->>Benthos Env: env.Parse(Text)
        Benthos Env-->>Diagnostic: error
        Note over Diagnostic: Match & extract multiline error line/char
    end
    
    Note over Diagnostic: Step 3: Parse Import Nodes
    Note over Diagnostic: Verify absolute path on filesystem
    
    Diagnostic-->>Actor: AttributeResponse (TypeDiagnostics, []Diagnostic)
```

---

## 3. Implementation Specifications

### 1. Registration
* **Interested Events**: `DidOpen`, `DidChange`.
* **Produced Attributes**: `AttributeDiagnostics`.

### 2. Step 1: Tree-sitter AST Syntax Validation
* Inspect the root node of the parsed Tree-sitter tree (`payload.AST.RootNode()`).
* If `root.HasError()` is true, recursively traverse children:
  * For any node where `IsError()` is true $\rightarrow$ Generate a `protocol.Diagnostic` with `SeverityError`, `Source = "bloblang (syntax)"`, and message `"Syntax error"`.
  * For any node where `IsMissing()` is true $\rightarrow$ Generate a `protocol.Diagnostic` with `SeverityError`, `Source = "bloblang (syntax)"`, and message `"Missing <node_kind>"`.
* **Important**: If syntax errors are found, skip Benthos semantic compilation to prevent flooded compiler errors.

### 3. Step 2: Benthos Environment Semantic Parse
* If syntax is clean, fetch or instantiate a Benthos Bloblang `Environment` loaded with custom filesystem importers matching the document's base directory.
* Call `env.Parse(payload.Text)`.
* If a parsing error occurs, parse the error message structure:
  * Detect `"line <X> char <Y>"` using string scans or regex.
  * Extract the clean compiler message (excluding location metadata).
  * Translate 1-indexed compiler offsets to 0-indexed LSP coordinates.
  * Generate a `protocol.Diagnostic` with `SeverityError`, `Source = "bloblang"`.

### 4. Step 3: Import Resolution Verification
* Locate all `import_statement` nodes in the tree-sitter AST.
* Resolve the target import path (e.g. `import "./methods.blobl"`) against the document's local base directory.
* Generate an information-level diagnostic (`SeverityInformation`, `Source = "bloblang"`) stating: `"Importing from <absolute_path>"`. This provides users with instant visual confirmation of resolved path targets.

---

## 4. Key Constraints & Rules

### What is EXPECTED
* **Pure Compile-time Validation**: Focus strictly on compilation semantics and syntax correctness. Do not execute runtime evaluations or load samples in this feature.
* **Instant processing**: Execution must finish within 1-2ms on typical documents, avoiding goroutine thread starvation.

### What is FORBIDDEN
* **No Client Calls**: The feature must never directly invoke `client.PublishDiagnostics(...)`. It merely returns its diagnostic array over its response channel to the `DocumentActor`, which handles reduction and publication.
