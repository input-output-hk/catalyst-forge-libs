## OCI Caching Guide

This guide explains the caching architecture, configuration, troubleshooting, and performance tuning for the OCI Bundle Distribution Module.

### Overview

- Caching accelerates repeated pulls by storing previously fetched bundles locally.
- Integration is transparent via `Client.PullWithCache` and cache-related options.
- Policies control when caching is applied (enabled, pull-only, push-only, disabled).

### Architecture

Caching is designed as layered components under `oci/internal/cache/`:

- Manifest cache: Short‑TTL metadata for quick validation.
- Blob cache: Long‑TTL content store for artifact layers.
- Tag resolver and mapping: Efficient tag→digest validation and movement detection.
- Manager and eviction: Size tracking, TTL expiry, LRU eviction, and maintenance.
- Storage layer: Atomic writes, file locking, checksums, and crash‑safe cleanup.

These components are coordinated through an internal `cache.Cache` interface to keep the public API stable and testable.

### Public API Surface

- `Client.PullWithCache(ctx, reference, targetDir, opts...)` – Pull with caching.
- `WithCachePolicy(policy)` – Set when caching is applied.
- `WithPullCacheBypass(true)` / `WithCacheBypass(true)` – Per‑operation bypass.
- `WithCache(coordinator, cachePath, maxSizeBytes, defaultTTL)` – Configure a cache coordinator and settings.

Notes:
- The coordinator parameter is an implementation of the internal cache interface. This allows advanced users and tests to inject custom behavior. If you do not supply a coordinator, use policy controls and the default behavior until a built‑in coordinator is exposed.

### Configuration Examples

Enable cache policy for pulls and use the cache‑aware pull path:

```go
client, err := ocibundle.NewWithOptions(
    ocibundle.WithCachePolicy(ocibundle.CachePolicyPull),
)
if err != nil { /* handle */ }

if err := client.PullWithCache(ctx, "ghcr.io/org/repo:tag", "./out"); err != nil {
    // falls back to network pull if cache is bypassed/disabled
}
```

Bypass cache for a specific call:

```go
err := client.PullWithCache(ctx, ref, outDir,
    ocibundle.WithCacheBypass(true),
)
```

Custom policy selection:

```go
// Disable caching entirely
client, _ := ocibundle.NewWithOptions(
    ocibundle.WithCachePolicy(ocibundle.CachePolicyDisabled),
)

// Enable for all operations (future‑proofing)
client, _ = ocibundle.NewWithOptions(
    ocibundle.WithCachePolicy(ocibundle.CachePolicyEnabled),
)
```

Advanced (coordinator injection):

```go
// Provide a coordinator implementing the internal cache interface
var coordinator cache.Cache // from oci/internal/cache

client, err := ocibundle.NewWithOptions(
    ocibundle.WithCache(coordinator, "/var/tmp/oci-cache", 1<<30, 24*time.Hour),
    ocibundle.WithCachePolicy(ocibundle.CachePolicyPull),
)
```

### Troubleshooting

- Cache miss when expected:
  - Ensure `PullWithCache` is used (not `Pull`).
  - Verify `WithCachePolicy` is not set to `disabled` or `push`.
  - Check per‑call `WithCacheBypass(true)` is not set.

- Registry still used on repeated pulls:
  - TTL or validation may cause a revalidation step; ensure policy/TTL suits your needs.

- Permissions or extraction errors:
  - Validate `PullOptions` limits match your content (file count, sizes, permissions).

### Performance Tuning

- Policy: Use `pull` or `enabled` to maximize cache hits on reads.
- TTL: Increase `defaultTTL` for manifests that change infrequently; decrease for fast‑moving tags.
- Size limits: Set `maxSizeBytes` based on available disk; larger caches increase hit rate.
- Concurrency: The storage layer uses file locking; avoid unnecessary parallel pulls of the same reference.

### Benchmarking

Run benchmarks within the module:

```bash
go test -bench=. -benchmem ./oci/...
```

Interpretation tips:
- Compare first pull (network) vs subsequent pulls (cache) on identical references.
- Track allocations (`-benchmem`) and wall‑clock speedups.

### Notes

- Caching is feature‑flagged via policy to preserve backward compatibility.
- All cache operations aim to be safe under crashes via atomic writes and cleanup.
- See `oci/examples/` for runnable demonstrations of cache usage and management.


