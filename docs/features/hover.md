# Feature Specification: Hover Feature (`hover.go`)

This document specifies the design, AST node identification, and live execution evaluation of the **Hover** feature (`internal/feature/hover/`).

---

## 1. Overview & Purpose

The `hover` feature provides interactive contextual information as the user hovers over code elements. It handles:
1. **Standard Library Documentation**: Displays clean markdown summaries, signatures, and example uses of standard Bloblang functions and methods.
2. **Live State Inspection**: Displays a pretty-printed JSON representation of the cumulative object structure when hovering over the `root` keyword in assignments.

---

## 2. Technical Execution Flow

```mermaid
sequenceDiagram
    autonumber
    Actor->>Hover: Channel (HoverParams, Nonce)
    Note over Hover: Step 1: Identify AST node under cursor
    
    alt Cursor is on standard function or method
        Note over Hover: Match name against standard library docs
        Hover-->>Actor: MarkupContent (Markdown Doc) + Nonce
    else Cursor is on "root" keyword in root_assignment
        Hover->>SampleCache: Fetch Cumulative Execution State (line - 1)
        Hover-->>Actor: MarkupContent (Pretty JSON) + Nonce
    else Cursor is on empty space / non-interactive node
        Hover-->>Actor: Nil Response + Nonce
    end
```

---

## 3. Implementation Specifications

### 1. Registration
* **Interested Events**: Pure synchronous channel listener.
* **Output Interface**: Returns `*protocol.Hover` directly over its reply channel.

### 2. Node Under Cursor Identification
* Translate the hover position (line/char offsets) to a Tree-sitter `Point`.
* Resolve the exact target node: `node := root.DescendantForPointRange(point, point)`.

### 3. Step 1: Standard Library Documentation Lookup
* Check if the resolved node represents an `identifier`:
  * If `node.Parent().Kind() == "call_expr"` $\rightarrow$ Treat as a standard function. Match token text against the `functionDocs` map loaded at server boot.
  * If `node.Parent().Kind() == "method_call"` $\rightarrow$ Treat as a standard method. Match token text against the `methodDocs` map.
* Return a formatted `protocol.Hover` containing the markdown documentation (`MarkupKind = protocol.Markdown`) and the exact span coordinates of the token (for target highlight).

### 4. Step 2: Live State Hover (`root` keyword)
* If the hovered token matches the keyword `"root"` and its parent node is a `"root_assignment"`:
  * Access the sample execution cash for the active document.
  * Retrieve the cumulative output state calculated immediately prior to the current statement line (`lineIndex - 1`).
  * If valid, return a hover card containing the full pretty-printed JSON payload formatted as a code block:
    ```
    ```json
    {
      "foo": "bar",
      "values": [1, 2, 3]
    }
    ```
    ```
  * If no sample is loaded, return a hover card prompting the user: `"Provide a sample with #!sample {\"key\": \"value\"}"`.

---

## 4. Key Constraints & Rules

### What is EXPECTED
* **Pre-compiled Documentation Caching**: Function and method markdown indices must be built/loaded in the background after boot and cached as immutable static maps, ensuring standard documentation lookups complete in nanoseconds.
* **Accurate Coordinate Highlighting**: Always return the exact bounding span `Range` of the hovered token so the editor can visually isolate the target during hover.

### What is FORBIDDEN
* **No AST Reparsing**: Never invoke `astParser.Parse(...)` inside the hover handler. Always read the shared pre-parsed `AST` from the `EventPayload` sent by the `DocumentActor`.
* **No Blocking Caching**: Hover requests must never trigger live filesystem reads or heavy remote network requests. All data must reside locally in memory.
