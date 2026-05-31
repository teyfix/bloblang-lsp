# Module Specification: Document Actor & Store Management (`document.go`)

This document specifies the design, lifecycle, and orchestration standards of the **Document Manager & Document Actor** package (`internal/document/`).

---

## 1. Overview & Purpose

The `document` package controls the state, content history, and parsing lifecycles of active files. It uses the LSP `document.Store` for raw text snapshots and implements a single-threaded **Document Actor** goroutine per file.

The `DocumentActor` acts as the **central feature orchestrator**: it owns a fixed set of feature goroutine handles (one instance per feature per document), maintains intermediate state buffers (`lastDiagnostics`, `lastInlayHints`, `lastCodeLenses`), dispatches jobs into feature mailboxes, aggregates and reduces responses, and sends the final payloads back to the handler or publishes them to the client.

**Key ownership rule**: Each `DocumentActor` instance — and the feature instances it constructs — belong exclusively to one document URI. There is no shared state between actors of different documents. This eliminates the entire class of inter-document race conditions.

---

## 2. Component Architecture

```
+-----------------------+
|    document.Store     |  (Raw string snapshot storage, thread-safe)
+-----------------------+
            ^
            |
+--------------------------+
|    DocumentManager       |  (Spawns and maps active DocumentActors)
|    Single goroutine loop |  (Owns uri → *DocumentActor map; lock-free)
+--------------------------+
            |
     [DocumentActor goroutine]  (One per open document URI)
            |
     Dispatches FeatureJob / QueryJob into feature mailboxes
            |
    +-------+-------+--------+-------+
    |       |       |        |       |
[diag  ] [sample] [hover ] [compl ]  ← each is its own goroutine
[loop()]  [loop()]  [loop()]  [loop()]
```

---

## 3. DocumentManager (Lock-Free)

`DocumentManager` runs as a **single goroutine** — the exclusive owner of the `uri → *DocumentActor` map. All external interactions (spawn, lookup, terminate) are expressed as channel commands, not method calls that acquire locks.

```go
type ManagerCommand struct {
    Kind    ManagerCommandKind  // Spawn | Lookup | Terminate
    URI     protocol.DocumentURI
    ReplyCh chan<- *DocumentActor
}

type DocumentManager struct {
    commands chan ManagerCommand
}

func (m *DocumentManager) run() {
    actors := map[protocol.DocumentURI]*DocumentActor{}
    for cmd := range m.commands {
        switch cmd.Kind {
        case Spawn:
            actor := newDocumentActor(cmd.URI, ...)
            actors[cmd.URI] = actor
            if cmd.ReplyCh != nil { cmd.ReplyCh <- actor }
        case Lookup:
            cmd.ReplyCh <- actors[cmd.URI] // nil if not found
        case Terminate:
            if a, ok := actors[cmd.URI]; ok {
                a.stop()
                delete(actors, cmd.URI)
            }
        }
    }
}
```

---

## 4. The DocumentActor Struct & Orchestration State

The `DocumentActor` is the exclusive owner of all feature handles and intermediate state buffers. It **never** holds feature-internal types — it holds only the interface handles (`AttributeProducer`, `QueryFeature`).

```go
type DocumentActor struct {
    uri     protocol.DocumentURI
    mailbox chan ActorMessage
    stop    chan struct{}
    logger  *slog.Logger
    store   *document.Store
    client  *server.Client  // for pushing async notifications

    // Async attribute-producing features (registered as AttributeProducer).
    producers []feature.AttributeProducer  // e.g. diagnostic, sample

    // Persistent per-feature reply channels — created once at construction,
    // passed to each feature. The actor select-loops over these permanently.
    // Key: feature.Name()
    producerReplyChs map[string]<-chan feature.AttributeResponse

    // Sync query features (registered as QueryFeature).
    queries map[meta.HandlerEvent]feature.QueryFeature  // e.g. hover, completion

    // Per-feature snapshot of the last valid reply, keyed by feature.Name().
    // Snapshots are never cleared on nonce advance — old data remains until
    // overwritten by a fresh valid reply, preventing blank intermediate states.
    snapshots map[string]*FeatureSnapshot

    // Per-feature nonce: tracks what nonce was last dispatched to each producer.
    // An incoming AttributeResponse is valid only if its Nonce matches this value.
    nonces map[string]uint64

    // Current dispatch nonce, incremented on each DidOpen/DidChange.
    currentNonce uint64
}
```

### Mailbox Message Layout

```go
// ActorMessage is sent by the Handler into the actor's mailbox.
type ActorMessage struct {
    Event   meta.HandlerEvent
    Params  any
    Nonce   uint64         // handler-level nonce for sync query staleness
    ReplyCh chan<- ActorResponse // nil for fire-and-forget events (DidOpen, DidChange)
}

type ActorResponse struct {
    Nonce  uint64
    Result any
    Err    error
}

// FeatureSnapshot holds the most recent valid output from one feature.
// It is updated in-place on each valid reply; never reset between dispatch rounds.
type FeatureSnapshot struct {
    Diagnostics []protocol.Diagnostic
    InlayHints  []protocol.InlayHint
    CodeLenses  []protocol.CodeLens
}
```

---

## 5. Feature Construction at Spawn Time

When `DocumentManager` spawns a new `DocumentActor`, the actor's constructor immediately builds all feature goroutines. Each feature receives only the dependencies it needs — no shared global state, no cross-document references.

```go
func newDocumentActor(uri protocol.DocumentURI, cfg *config.Config, store *document.Store,
    client *server.Client, astParser *ast.Bloblang, logger *slog.Logger) *DocumentActor {

    // Each New*Feature() starts its own loop() goroutine internally.
    diagFeature    := diagnostic.New(astParser, logger)
    sampleFeature  := sample.New(cfg, astParser, logger)
    hoverFeature   := hover.New(astParser, logger)
    completionFeature := completion.New(logger)

    return &DocumentActor{
        uri:       uri,
        mailbox:   make(chan ActorMessage),
        stop:      make(chan struct{}),
        store:     store,
        client:    client,
        producers: []feature.AttributeProducer{diagFeature, sampleFeature},
        queries: map[meta.HandlerEvent]feature.QueryFeature{
            meta.EventHover:      hoverFeature,
            meta.EventCompletion: completionFeature,
        },
        logger: logger,
    }
}
```

---

## 6. Operational Lifecycle & Orchestration Flow

### 1. Actor Spawning (`DidOpen`)
* Handler sends a `Spawn` command to `DocumentManager`.
* `DocumentManager` creates the `DocumentActor` — which internally constructs and starts all feature goroutines.
* The actor starts `go actor.loop()`.
* The actor immediately dispatches `DidOpen` jobs to all interested `AttributeProducer`s, collects responses, and publishes baseline diagnostics to the client.

### 2. Document Mutation & Streaming Reduction (`DidChange`)
* When `DidChange` arrives in the mailbox:
  1. Actor increments `currentNonce`.
  2. Actor updates the document text snapshot in `store`.
  3. Actor calls `Dispatch(FeatureJob{Nonce: currentNonce, ...})` on each interested producer.
     * `Dispatch` uses the **drain-and-replace** pattern on the capacity-1 mailbox — always non-blocking.
     * `a.nonces[feature.Name()] = currentNonce` is recorded for future reply validation.
  4. Actor immediately returns to its `select` loop — it does **not** block waiting for replies.
* Feature goroutines process their jobs and write `AttributeResponse`s to their persistent reply channels.
* The actor's `select` loop picks up each reply as it arrives:
  * **Nonce check**: if `resp.Nonce != a.nonces[resp.Feature]` → stale → drop silently.
  * **Snapshot update**: update `a.snapshots[resp.Feature]` with the new value.
  * **Merge & publish**: immediately merge all current snapshots and call `client.PublishDiagnostics` / update hint/lens buffers.
* The client receives incremental updates — one per feature as each finishes — with no blank intermediate state, because snapshots from other features remain until replaced.

### 3. Query Execution (`Hover`, `Completion`)
* Handler generates a nonce, wraps the request in an `ActorMessage` with a `ReplyCh`, and sends it to the actor's mailbox.
* The actor reads the message sequentially from its mailbox.
* It creates a fresh single-use `chan QueryResponse`, builds a `QueryJob{Nonce: msg.Nonce}`, and calls `Query(job)` on the matching `QueryFeature` (drain-and-replace into the feature's mailbox).
* The actor then **blocks only on this one reply channel** until the feature responds.
* Nonce is validated on the response. If stale, nil is returned to the handler.
* The result is forwarded to the handler's `ReplyCh`.

### 4. InlayHint & CodeLens Queries
* These are served directly from the actor's pre-computed merged snapshot buffers — no feature dispatch on query.
* The actor returns its current merged `lastInlayHints` / `lastCodeLenses` immediately.

### 5. Actor Reclamation (`DidClose`)
* Handler sends a `Terminate` command to `DocumentManager`.
* The manager calls `actor.stop()` — which closes the `stop` channel, causing `loop()` to exit.
* The actor calls `Close()` on every `AttributeProducer` and `QueryFeature`, causing their goroutines to drain and exit cleanly.

---

## 7. Key Constraints & Rules

### What is EXPECTED
* **Single Thread of State Control**: `lastDiagnostics`, `lastInlayHints`, and `lastCodeLenses` are only modified inside `actor.loop()`. No external code reads them directly.
* **Features via Interface Only**: The actor stores features as `AttributeProducer` / `QueryFeature` interfaces. It never imports or references a feature's concrete package types.
* **Reduction Inside Actor**: The actor is the reducer. It aggregates feature outputs by `AttributeType` and formats unified LSP structs for the handler.

### What is FORBIDDEN
* **No Feature-Specific State in Handler**: The `Handler` must never hold diagnostic lists, completion caches, or sample states. Everything is isolated inside each document's `DocumentActor`.
* **No Inter-Feature Communication**: Feature goroutines must not communicate with each other. They receive jobs from the actor and reply to the actor. Period.
* **No Locks Inside Features**: Because each feature goroutine is the sole owner of its internal state (scoped to one document), `sync.Mutex` and `sync.RWMutex` are never needed inside a feature package.
* **No Feature Logic in Actor**: The actor dispatches, collects, and reduces. It does not parse, evaluate, or inspect document content itself.
