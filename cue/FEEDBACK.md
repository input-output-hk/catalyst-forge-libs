Here’s what the **current CUE Go API** recommends for validation and errors—followed by targeted feedback on your helper.

# What “good” validation looks like (from current docs)

* **Build values in one `cuecontext.Context`** and then either:

  * `schema.Unify(data).Validate(...)` (classic pattern), or
  * validate a single value directly: `val.Validate(cue.Concrete(true))` (useful when `val` already includes `#Schema & data`). ([CUE][1])
* **Pass options to `Validate`** when you need stricter behavior:

  * `cue.Concrete(true)` → fail if anything’s incomplete
  * `cue.Final()` → resolve defaults before checking
  * `cue.All()` → report all errors, not just first (pairs well with `errors.Errors`)
    These options are defined on `cue.Option` alongside `Validate`. ([Go Packages][2])
* **Use `cue/errors` to inspect errors**, not string parsing:

  * `errors.Errors(err)` → slice of structured errors (each with its own message)
  * `errors.Path(err)` → logical field path
  * `errors.Positions(err)`/`errors.Details(err, nil)` → file/pos & human text, respectively. ([Go Packages][3])
* **Definitions are closed** by default (e.g., `#Schema`), which is important if you intend to reject unknown fields. If you want that behavior, define your schema as a `#Definition` and validate against it. ([CUE][4])
* **Common pitfalls** called out in the docs & guides:

  * Mixing `cue.Value`s from **different `cuecontext.Context`s** (operations must use the same context). ([Go Packages][2])
  * Treating `err.Error()` as canonical—**CUE error strings are aggregate summaries**; always unpack them to get *all* failures and structured paths/positions. ([Go Packages][3])
  * Forgetting to require concreteness: `Validate()` without options may allow incomplete values to pass when you expected a hard failure. ([CUE][5])
  * Loading/compiling schema/data incorrectly—prefer `cue/load` for modules and the `encoding/json|yaml` helpers for external data. ([CUE][1])

# Review of your helper (what you did well + concrete fixes)

Source reviewed:

**What’s good**

* ✅ Sensible flow: check `ctx.Err()`, `schema.Err()`, `data.Err()`, then `Unify` and `Validate`. That mirrors the official examples. ([CUE][1])
* ✅ You capture **“schema invalid”, “data invalid”** vs **“unify/validate failed”**—useful distinctions for callers.
* ✅ A separate `ValidateConstraint` is a nice ergonomic touch for one-off checks.

**Issues & changes to make**

1. **Don’t parse error strings to get “field paths”.**
   `extractFieldPaths` splits `err.Error()`—that’s brittle and loses structure. Use `cue/errors`:

```go
errs := errors.Errors(err)
for _, e := range errs {
    paths := e.Path()              // []string
    poss  := e.InputPositions()    // []token.Pos (or use errors.Positions(err))
    msgFmt, msgArgs := e.Msg()     // structured message
}
```

If you just need a single human string, prefer `errors.Details(err, nil)` over building your own. ([Go Packages][3])

2. **Report *all* failures, not just the first.**
   Your flow calls `unified.Err()` and `unified.Validate()` once, then returns. Wrap the outgoing error with **all** underlying errors to aid UX:

```go
if err := unified.Validate(cue.Concrete(true)); err != nil {
    details := errors.Details(err, nil)         // full multi-error text
    paths   := [][]string{}
    for _, e := range errors.Errors(err) {
        paths = append(paths, e.Path())
    }
    // include details + paths in your PlatformError context
}
```

(Optionally add `cue.All()` if you want broader coverage.) ([CUE][5])

3. **Require concreteness when appropriate.**
   If the call site expects fully-materialized data, use `Validate(cue.Concrete(true), cue.Final())`. Without this, “incomplete” values can slip through and surprise callers later. Provide a knob to toggle this strictness. ([CUE][5])

4. **Guarantee same `cuecontext.Context`.**
   Operations like `schema.Unify(data)` **must** use values from the *same* context. Add a guard:

```go
if schema.Context() != data.Context() {
    return wrap(..., "schema and data use different CUE contexts", nil)
}
```

Or re-build one into the other’s context upstream. ([Go Packages][2])

5. **Prefer structured paths over colon-splitting.**
   Your current path extraction will misbehave with messages containing colons or multi-line diagnostics. Switch to `errors.Path(err)` / `Error.Path()` (slice of labels) and serialize with `cue.MakePath`/`Path.String()` when you need a string. ([Go Packages][3])

6. **Surface positions when available.**
   Developers love “file:line:col”. Include `errors.Positions(err)` (or per-error `InputPositions`) in your enriched context so editors/CI can hyperlink failures. ([Go Packages][3])

7. **Constraint helper: enforce options & detail.**
   `ValidateConstraint` currently returns after `Unify`. Add a `Validate(cue.Concrete(true))` step and the same `errors.Errors/Details` handling as above so callers get consistent, multi-error feedback. ([CUE][5])

8. **(If you need strict “no extra fields”) make sure the schema is a definition.**
   If you want to reject unknown fields, define your schema as `#Schema` (definitions are *closed*) and unify `#Schema & data`. Otherwise structs are open and extra fields may be allowed. ([CUE][4])

# A minimal, more robust pattern (drop-in idea)

```go
// after building schema/data in the SAME context…
u := schema.Unify(data)
if err := u.Validate(cue.Concrete(true)); err != nil {
    // 1) full human-readable message
    human := errors.Details(err, nil)

    // 2) machine-usable items
    var issues []struct {
        Path []string
        Msg  string
    }
    for _, e := range errors.Errors(err) {
        fmtStr, args := e.Msg()
        issues = append(issues, struct {
            Path []string
            Msg  string
        }{Path: e.Path(), Msg: fmt.Sprintf(fmtStr, args...)})
    }

    // wrap in your PlatformError with {human, issues, positions}
    return wrapValidationErrorWithContext(err, "validation failed", makeContext(
        "details", human,
        "issues", issues,
        "positions", errors.Positions(err),
    ))
}
```

This uses only stable, public APIs; no string slicing. ([Go Packages][3])

[1]: https://cuelang.org/docs/howto/validate-json-using-go-api/ "Validating JSON using the Go API | CUE"
[2]: https://pkg.go.dev/cuelang.org/go/cue "cue package - cuelang.org/go/cue - Go Packages"
[3]: https://pkg.go.dev/cuelang.org/go/cue/errors "errors package - cuelang.org/go/cue/errors - Go Packages"
[4]: https://cuelang.org/docs/concept/schema-definition-use-case/?utm_source=chatgpt.com "Schema Definition use case - CUE"
[5]: https://cuelang.org/docs/howto/handle-errors-go-api/ "Handling errors in the Go API | CUE"
