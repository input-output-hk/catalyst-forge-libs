# git — High‑Level Go Wrapper for go‑git (Architecture)

**Status**: Draft v1 · **Owner**: Platform Eng / SRE · **Last updated**: 2025‑09‑16

## 0) Executive Summary
`git` is a small, idiomatic Go façade over `github.com/go-git/go-git/v5` that exposes a handful of **high‑level, task‑oriented** operations (clone/open, branch workflows, fetch/pull/merge, stage/commit/push, history, diffs, tags, ref resolution). It is **not** a reimplementation of Git semantics; it composes the `go-git` porcelain APIs in a clean, testable shape and **must** operate exclusively through the project’s native filesystem abstraction (`fs.Filesystem`) for both on‑disk and in‑memory repositories.

Design emphasizes:
- **Minimal surface area** (easy to learn, easy to extend)
- **Testability by construction** (in‑memory FS, side‑effect control)
- **Security & performance** (context timeouts, controlled transports, LRU object cache, shallow ops)
- **Go idioms** (accept interfaces, return concrete types)

---

## 1) Goals & Non‑Goals
### Goals
1. Offer a **simple, stable** API for common repository tasks.
2. Enforce the team’s **native filesystem abstraction** everywhere (no direct `os` use).
3. Be **easy to mock** and to run entirely **in‑memory**.
4. Keep **adding new operations trivial** (thin wrappers mapping 1:1 to `go-git`).
5. Provide **sane defaults** and **safe auth** integration (HTTPS/SSH).

### Non‑Goals
- Full Git CLI parity; advanced edge cases (e.g., interactive merges, rebase orchestration) are out of scope.
- Replacing `go-git`; we intentionally delegate behavior to it.
- Inventing a new FS abstraction; we adopt the team’s `fs.Filesystem` and its `billy` adapter.

---

## 2) Key Principles
- **Small façade, large leverage**: A single public type (`*Repo`) exposes high‑value operations; everything else is helper/adapter.
- **Accept interfaces, return types**: Consumers pass in `fs.Filesystem` and `AuthProvider`; we return `*Repo` (not interfaces). Consumers can define their own narrow interfaces if they wish to mock.
- **Pure‑FS design**: No hidden tempdirs. All state (objects, index, worktree) lives on the provided `fs.Filesystem`.
- **Context everywhere**: Each operation has a `Context` variant internally to enable timeouts/cancellation.

---

## 3) External Dependencies & Compatibility
- **go‑git**: `github.com/go-git/go-git/v5` (target v5.16.x)
  - We use porcelain APIs: `Repository.Clone/Open/Init`, `Worktree.Checkout/Add/Commit/Reset/Pull`, `Repository.Fetch/Push/Log/Merge/ResolveRevision/References/Branches/Tags/CreateTag/DeleteTag`, `plumbing/object` diffs (`Tree.Patch`, `Changes.Patch`).
- **go‑billy**: `github.com/go-git/go-billy/v5` (transitively via go‑git storage).
- **Native FS abstraction**:
  - Module: `github.com/input-output-hk/catalyst-forge/lib/tools/fs` (this project’s **canonical** FS interface)
  - Subpackage adapter: `github.com/input-output-hk/catalyst-forge/lib/tools/fs/billy`
    - `billy.NewOSFS(root) *billy.FS`
    - `billy.NewInMemoryFS() *billy.FS`
    - `billy.NewFS(b billy.Filesystem) *billy.FS` (wrap existing go‑billy FS)

> **Contract**: `git` always accepts `fs.Filesystem`. Internally we adapt to `billy.Filesystem` for go‑git’s storage & worktree.

---

## 4) Package Layout
```
.git/
  git.go                // public façade: Options, Repo, high-level ops
  errors.go              // sentinel error catalog + wrapping helpers
  internal/
    fsbridge/            // adapt fs.Filesystem <-> billy.Filesystem, open/clone/init
    auth/                // AuthProvider -> go-git transport.AuthMethod helpers
    refs/                // ref normalization & filters (heads/tags/remotes)
    diff/                // resolve two revs -> unified diff text/patch
```

- **Public surface** intentionally small; internal packages are tiny utility layers to keep `git.go` readable.

---

## 5) Public API (shape & semantics)

> **Note**: Signatures shown for shape; actual code lives in `git.go`. The façade methods perform argument validation, compute reasonable defaults, then delegate to go‑git.

### 5.1 Construction
```go
// Options configures repository discovery/creation and performance.
type Options struct {
    FS              fs.Filesystem  // REQUIRED – native FS root (OS or in‑memory)
    Workdir         string         // path within FS for the worktree root
    Bare            bool           // if true, create/use a bare repo (.git only)
    StorerCacheSize int            // LRU objects cache entries (default sensible)
    Auth            AuthProvider   // optional; resolves per‑URL AuthMethod
    HTTPClient      *http.Client   // optional; custom transport (timeouts, proxy)
    ShallowDepth    int            // if >0, perform shallow clone/fetch
}

// Open, Clone, or Init a repo rooted at Options.FS + Workdir.
func Open(ctx context.Context, opts Options) (*Repo, error)
func Clone(ctx context.Context, remoteURL string, opts Options) (*Repo, error)
func Init(ctx context.Context, opts Options) (*Repo, error)
```

**Behavior**
- Storage uses `filesystem.NewStorage(billyFS, cache.NewObjectLRU(N))`.
- Non‑bare repos use the passed FS for both `.git` and worktree (via `billy` adapter). Bare repos set `worktree=nil`.
- `ShallowDepth` is honored on clone/fetch/pull where supported.

### 5.2 Repo façade
```go
type Repo struct { /* unexported: holds go‑git Repository, Worktree, config */ }

// Branches
func (r *Repo) CurrentBranch(ctx context.Context) (string, error)
func (r *Repo) CreateBranch(ctx context.Context, name string, startRev string, trackRemote bool, force bool) error
func (r *Repo) CheckoutBranch(ctx context.Context, name string, createIfMissing bool, force bool) error
func (r *Repo) DeleteBranch(ctx context.Context, name string) error
func (r *Repo) CheckoutRemoteBranch(ctx context.Context, remote, remoteBranch, localName string, track bool) error

// Sync with upstream
func (r *Repo) Fetch(ctx context.Context, remote string, prune bool, depth int) error
func (r *Repo) PullFFOnly(ctx context.Context, remote string) error
func (r *Repo) FetchAndMerge(ctx context.Context, remote, fromRef string, strategy MergeStrategy) error

// Stage / Unstage / Commit / Push
func (r *Repo) Add(ctx context.Context, paths ...string) error
func (r *Repo) Remove(ctx context.Context, paths ...string) error
func (r *Repo) Unstage(ctx context.Context, paths ...string) error     // uses Reset (mixed) or ResetSparsely
func (r *Repo) Commit(ctx context.Context, msg string, who Signature, opts CommitOpts) (string, error)
func (r *Repo) Push(ctx context.Context, remote string, force bool) error

// History & diffs
func (r *Repo) Log(ctx context.Context, f LogFilter) (CommitIter, error)
func (r *Repo) Diff(ctx context.Context, a, b string, pathFilter func(string) bool) (PatchText, error)

// Tags
func (r *Repo) CreateTag(ctx context.Context, name, target, message string, annotated bool) error
func (r *Repo) DeleteTag(ctx context.Context, name string) error
func (r *Repo) Tags(ctx context.Context, pattern string) ([]string, error)

// Refs & Resolution
type RefKind int // Branch, RemoteBranch, Tag, Commit, Other
func (r *Repo) Refs(ctx context.Context, kind RefKind, pattern string) ([]string, error)
func (r *Repo) Resolve(ctx context.Context, rev string) (ResolvedRef, error)
```

**Semantics & mapping (selected):**
- **CreateBranch**: builds `config.Branch` (Remote/Merge) when `trackRemote=true`; `startRev` resolves via `ResolveRevision`.
- **CheckoutBranch**: uses `Worktree.Checkout` with `Create`/`Branch` fields as needed.
- **Unstage**: uses `Worktree.Reset` (mixed) or `ResetSparsely` for specific paths.
- **PullFFOnly**: uses `Worktree.Pull` (fast‑forward merges only).
- **FetchAndMerge**: `Repository.Fetch` then `Repository.Merge` with a `MergeStrategy` (e.g., FF only vs. normal merge) as supported by go‑git.
- **Diff**: resolve `a` and `b` → commits → trees → `Tree.Patch` → unified text (`Patch.String()`).
- **Resolve**: `ResolveRevision` + classification (`heads/tags/remotes` prefix) to produce a `ResolvedRef{Kind, Hash, CanonicalName}`.

---

## 6) Filesystem Integration (hard requirement)
`git` **only** accepts `fs.Filesystem`. Typical patterns:

- **OS‑backed repo**
  ```go
  root := "/projects/repo"
  myFS := billyfs.NewOSFS(root) // from fs/billy
  r, _ := git.Open(ctx, git.Options{FS: myFS, Workdir: "."})
  ```
- **In‑memory repo (tests / ephemeral)**
  ```go
  mem := billyfs.NewInMemoryFS()  // from fs/billy
  r, _ := git.Init(ctx, git.Options{FS: mem, Workdir: "/"})
  ```

**Why this matters**
- All IO is mediated by the team’s FS abstraction (observability, determinism, portability).
- Unit tests can run **entirely in memory** with `file://` remotes for push/pull/fetch integration tests.

**Storage**
- Objects storer: `filesystem.NewStorage(billyFS, cache.NewObjectLRU(size))`.
- Worktree: mounted using the same `billyFS` (non‑bare).

---

## 7) Authentication Model
```go
type AuthProvider interface {
    Method(ctx context.Context, remoteURL string) (transport.AuthMethod, error)
}
```
Built‑ins (composable):
- **HTTPS**: token/password via `http.BasicAuth` (most providers accept OAuth token as password or username).
- **SSH**: key files or SSH agent using `ssh.PublicKeys` / `ssh.NewPublicKeysFromFile`, configurable host‑key callback.

**Usage**: Construction passes `AuthProvider` into `Options`. Each networked operation (clone/fetch/pull/push) resolves per‑URL credentials just‑in‑time.

**Security defaults**
- No credential logging. Host key checking **on** by default. `InsecureSkipVerify` only behind explicit test flags.
- Optional `HTTPClient` to control timeouts, connection pools, and proxies; installed into go‑git’s HTTP transport where applicable.

---

## 8) Error Model
We expose a small set of **sentinel** errors for consumers to branch on with `errors.Is`, while preserving original `go-git` errors via `%w`:

- `ErrAlreadyUpToDate` (maps from go‑git’s no‑op fetch/pull/push cases)
- `ErrAuthRequired`, `ErrAuthFailed`
- `ErrBranchExists`, `ErrBranchMissing`, `ErrTagExists`, `ErrTagMissing`
- `ErrNotFastForward`, `ErrMergeConflict` (from `Repository.Merge` where applicable)
- `ErrInvalidRef`, `ErrResolveFailed`

This keeps calling code simple without parsing strings.

---

## 9) Concurrency & Lifecycle
- A `Repo` instance is **not** intended for concurrent **writes**. Reads (e.g., `Log`, `Diff`, `Refs`) are safe to run concurrently if they don’t mutate state.
- All operations are context‑aware; network transports are bound by the passed `Context`.
- `Repo` holds onto the underlying `go-git` repository and worktree; no background goroutines.

---

## 10) Performance Considerations
- **LRU object cache** on the storer (configurable `StorerCacheSize`).
- **Shallow** clone/fetch (`depth` fields) to reduce network/data volume where applicable.
- **HTTP client reuse** (single tuned `*http.Client` per `Repo` when provided).
- **Minimal diff**: allow optional `pathFilter` to limit `Tree.Patch` scope.

---

## 11) Security Considerations
- **Protocol allow‑list** (optional): reject non‑HTTPS/SSH URLs unless explicitly enabled.
- **Host key verification** for SSH; centralized callback configuration.
- **Redaction**: redact URLs/headers in logs.
- **No shelling out**; pure Go execution path.

---

## 12) Test Strategy
- **Unit tests**: construct repos on `billy.NewInMemoryFS()`; inject `file://` remotes for networked flows.
- **Golden tests**: diff rendering (`Patch.String()`), ref resolution, branch tracking config.
- **Fault injection**: fake `AuthProvider` to simulate auth failures; timeouts via short contexts.
- **No external network** in CI; fixtures served from local bare repos on the in‑memory FS.

---

## 13) Extensibility Pattern
Adding a new operation should be a ~20–40 LOC method that:
1. Validates args → resolves refs with `Resolve`.
2. Calls the corresponding `go-git` API (Context variant when available).
3. Maps known errors to `git` sentinels, wraps with context.
4. Returns **plain values** (strings, small structs) instead of leaking `go-git` internals.

---

## 14) Example Flows (abridged)
- **Clone & fast‑forward pull**
  1) `Clone(ctx, url, opts{FS, Workdir, ShallowDepth:1, Auth})`
  2) `CurrentBranch` → `main`
  3) `PullFFOnly(ctx, "origin")`

- **Create feature branch from upstream**
  1) `Fetch(ctx, "origin", prune=true, depth=0)`
  2) `CreateBranch(ctx, "feature/x", "origin/main", trackRemote=false, force=false)`
  3) `CheckoutBranch(ctx, "feature/x", createIfMissing=false, force=false)`

- **Commit & push**
  1) `Add(ctx, "cmd/foo/main.go")`
  2) `Commit(ctx, "feat: foo", who, opts)` → returns commit SHA
  3) `Push(ctx, "origin", force=false)`

- **Resolve & diff**
  1) `Resolve(ctx, "v1.2.3")` → `{Kind: Tag, Hash: …}`
  2) `Diff(ctx, "HEAD~1", "HEAD", filter)` → unified diff text

---

## 15) Open Questions / Risks
- **Merge coverage**: go‑git supports fast‑forward pulls and a `Repository.Merge` API for non‑FF cases; complex conflict resolution remains limited compared to the Git CLI. We’ll document limitations and surface clear errors.
- **Submodules**: not included in v1 façade; support later if needed (`Submodules()` APIs exist but add complexity).
- **Rename/Similarity** detection in diffs: go‑git differs from Git CLI heuristics; acceptable for our use‑cases but should be noted in docs.

---

## 16) Configuration & Defaults
- Default remote: `origin` (overridable per call).
- Default branch base: `HEAD` (or `origin/HEAD` if explicit remote flow).
- Tag creation: lightweight by default; annotated when `message != ""` and signer/tagger provided.
- Diff output: unified text via `Patch.String()`; optionally return structured hunks later.

---

## 17) Appendix A — go‑git API Mapping (non‑exhaustive)
- **Repository lifecycle**: `Repository.Clone/Open/Init` (+ Plain* helpers)
- **Worktree ops**: `Worktree.Checkout/Add/Remove/Commit/Reset/ResetSparsely/Pull`
- **Sync**: `Repository.Fetch/Push`, `Worktree.Pull`
- **Merge**: `Repository.Merge(ref, MergeOptions)`
- **Refs**: `Repository.References/Branches/Tags`, `Reference`, `Head`
- **Resolve**: `Repository.ResolveRevision(plumbing.Revision)`
- **History**: `Repository.Log(LogOptions)`
- **Diff**: `object.(*Tree).Patch(to)`, `object.(Changes).Patch()`

---

## 18) Appendix B — FS Abstraction Contract (summary)
Minimal `fs.Filesystem` requirements (per team’s package):
- File ops: `Open`, `OpenFile`, `Create`, `WriteFile`, `ReadFile`, `ReadDir`, `Remove`, `Stat`, `MkdirAll`, `TempDir`, `Walk`.
- File handle: `Read`, `ReadAt`, `Write`, `Seek`, `Stat`, `Close`, `Name`.
- Helpers: `GetAbs(path) (string, error)`, `Exists(path) (bool, error)`.

**Implementations available** (via `fs/billy` subpackage):
- `NewOSFS(root string) *billy.FS` – OS‑backed, rooted at `root`.
- `NewInMemoryFS() *billy.FS` – in‑memory.
- `NewFS(b billy.Filesystem) *billy.FS` – wraps an existing `go-billy` FS.

`git` works with **any** of these; tests exclusively use the in‑memory variant.

---

## 19) Milestones
1. **MVP**: façade + branch/sync/commit/diff/tags + in‑memory tests.
2. **DX polish**: better errors, docstrings, examples, path filters.
3. **Nice‑to‑haves**: structured diff API, submodule listing, progress callbacks.

---

## 20) Acceptance Criteria
- All public APIs operate solely via `fs.Filesystem`.
- Full in‑memory test suite (no OS tempdirs, no external network).
- Clear error mapping and documentation of merge limitations.
- Adding a new high‑level operation requires touching only the façade and (optionally) a tiny helper in `internal/`.

