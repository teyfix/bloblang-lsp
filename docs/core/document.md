# Module Specification: Document Actor & Store Management (`document.go`)

This document specifies the design, lifecycle, and orchestration standards of the **Document Manager & Document Actor** package (`internal/document/`).

---

## 1. Overview & Purpose

The `document` package controls the state, content history, and parsing lifecycles of active files. It utilizes `github.com/owenrumney/go-lsp/document`'s `Store` and implements a single-threaded **Document Actor** goroutine per file. 

The `DocumentActor` acts as the **central feature orchestrator**: it encapsulates the feature engines, maintains all feature-specific states (such as active diagnostics maps, inlay hints lists, and code lenses lists), coordinates concurrent feature queries over channels, aggregates and reduces responses, and sends the final reduced payloads back to the outer handler or client.

---

## 2. Component Architecture

```
                 +-----------------------+
                 |    document.Store     | (Raw string snapshot storage)
                 +-----------------------+
                             ^
                             |
                 +-----------------------+
                 |    DocumentManager    | (Spawns and maps active DocumentActors)
                 +-----------------------+
                             |
                      [DocumentActor] (One single-threaded goroutine per document)
                             |
         +-------------------+-------------------+
         | (Channels)        | (Channels)        | (Channels)
         v                   v                   v
+------------------+ +------------------+ +------------------+
| Feature: Diags   | | Feature: Hints   | | Feature: Lenses  |
+------------------+ +------------------+ +------------------+
```

---

## 3. The Document Actor Struct & Orchestration State

The `DocumentActor` is the exclusive owner of features and intermediate state buffers:

```go
type DocumentActor struct {
    uri        protocol.DocumentURI
    mailbox    chan ActorMessage
    logger     *slog.Logger
    astParser  *ast.Bloblang
    store      *document.Store
    client     *server.Client // Bidirectional RPC channel for pushing async notifications
    
    // Encapsulated Feature Instances
    sampleFeature      *sample.Feature      // Provides diagnostics, hints, lenses
    hoverFeature       *hover.Feature       // Provides documentation of hovered methods/functions
    completionFeature  *completion.Feature  // Provides completions based on tree nodes
    diagnosticFeature  *diagnostic.Feature  // Checks AST, parses whole document, publishes diagnostics

    // Active Feature States
    lastDiagnostics   []protocol.Diagnostic
    lastInlayHints    []protocol.InlayHint
    lastCodeLenses    []protocol.CodeLens
}
```

### mailbox Command Message Layout
Messages posted to the actor include synchronous response slots:
```go
type ActorMessage struct {
    Event      meta.HandlerEvent
    Params     any
    Nonce      string
    ReplyCh    chan<- ActorResponse // Used to return reduced responses to the handler
}

type ActorResponse struct {
    Nonce      string
    Result     any
    Error      error
}
```

---

## 4. Operational Lifecycle & Orchestration Flow

### 1. Actor Spawning (`DidOpen`)
* The outer handler registers the document in the store and signals `DocumentManager`.
* `DocumentManager` creates the `DocumentActor`, linking the `ast.Bloblang` parser and the target features.
* The actor starts its goroutine `go actor.loop()`.
* **State Hook**: The actor performs an initial syntax check and triggers the `diagnosticFeature` and `sampleFeature` asynchronously. They compile diagnostics and the actor publishes the baseline diagnostics back to the client immediately.

### 2. Document Mutation & State Invalidation (`DidChange`)
* When `DidChange` is received, the actor receives a mailbox command.
* The actor updates its local AST reference: `actor.astParser.Parse(...)`.
* It triggers interested features asynchronously using dedicated Go channels.
* For diagnostics, both `diagnosticFeature` and `sampleFeature` process the document, return their diagnostics over channels, and the actor merges (reduces) these into `lastDiagnostics`, publishing the merged slice to the client.

### 3. Query Execution & Response Reduction (`Hover`, `InlayHint`, `CodeLens`)
* When a query is called:
  1. The handler posts a message containing the parameters, a unique `Nonce`, and a `ReplyCh`.
  2. The actor reads the request sequentially from its mailbox.
  3. It identifies which feature computes the data (e.g. `sampleFeature` for inlay hints or lenses, `hoverFeature` for hover, `completionFeature` for completions).
  4. It dispatches a task to the feature, specifying a local response channel.
  5. The feature executes (under 5ms) and sends its results back.
  6. The actor takes the output, **reduces** it (merging it with current outstanding states or validating format constraints), and posts the finalized `ActorResponse` + `Nonce` back to the handler's `ReplyCh`.

### 4. Actor Reclamation (`DidClose`)
* Upon closure, a termination command is posted.
* The actor stops features, discards local diagnostic lists, closes its channels, and terminates its goroutine loop.

---

## 5. Key Constraints & Rules

### What is EXPECTED
* **Single Thread of State Control**: All states (diagnostics lists, hints, lenses) must be modified *only* inside the actor's goroutine loop, eliminating race conditions.
* **Reduction inside Actor**: The actor is the reducer. It gathers feature outputs and formats them into the standard LSP struct formats expected by the handler.

### What is FORBIDDEN
* **No Feature-Specific State in Handler**: The `Handler` must never keep lists of diagnostics or caches. Everything is isolated within the document's corresponding `DocumentActor`.
* **No Inter-Feature State Leaks**: Features must not directly query each other's structures. They only read parameters sent by the actor and return responses.
