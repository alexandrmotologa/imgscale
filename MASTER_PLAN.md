# imgscale: Architecture and Engineering Master Plan

> Self-Hosted Dynamic Image Resizing, WebP/AVIF Transcoding and Smart Crop Gateway (< 35MB RAM)
> Cloudinary, Imgix, and Thumbor alternative packaged as a single Go binary with an embedded React 19 visual studio.

---

## 1. Executive Summary and Problem Space

### The Problem
Modern web and mobile applications require responsive, format-optimized images to meet Google Core Web Vitals criteria, particularly Largest Contentful Paint (LCP). Engineering teams face two major tradeoffs:
1. SaaS image providers (Cloudinary, Imgix, Cloudflare Images) impose steep monthly fees tied to transformation counts and bandwidth, while locking workflows into vendor-specific URL parameters.
2. Self-hosted alternatives present operational bottlenecks:
   - Thumbor relies on Python and heavy C libraries, regularly idling above 250MB RAM and complicating container builds.
   - Next.js dynamic image optimization consumes Node.js memory limits and frequently triggers out-of-memory errors during traffic spikes.

### The Solution: imgscale
imgscale is a standalone media processing gateway written in Go with an embedded React 19 visual studio. It connects client applications to upstream image storage (local filesystem, AWS S3, Cloudflare R2, MinIO, or remote HTTP origins), transforming and caching images with sub-35ms processing latency and less than 35MB idle RAM.

---

## 2. System Architecture and Lifecycle

```
[ Client Browser / Mobile App ]
          │  GET /v1/images/photo.jpg?w=800&h=600&fit=cover&gravity=smart&format=auto&q=82&s=e9b2...
          ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        imgscale Gateway Daemon                         │
│                                                                        │
│  ┌───────────────────────┐         ┌────────────────────────────────┐  │
│  │  HMAC Signature Guard │ ──────► │ HTTP Accept Format Negotiator  │  │
│  └───────────────────────┘         └────────────────────────────────┘  │
│                                                     │                  │
│  ┌──────────────────────────────────────────────────▼───────────────┐  │
│  │ L1 In-Memory Fast LRU Cache (Lock-free Sharded Key-Value)        │  │
│  └────────────────────────┬─────────────────────────────────────────┘  │
│               Cache HIT   │                     │ Cache MISS           │
│       ┌───────────────────┘                     ▼                      │
│       │                    ┌────────────────────────────────────────┐  │
│       │                    │ L2 Persistent Disk Cache (Content Addr)│  │
│       │                    └────────────────────┬───────────────────┘  │
│       │                             Cache HIT   │   │ Cache MISS       │
│       │                         ┌───────────────┘   ▼                  │
│       │                         │    ┌──────────────────────────────┐  │
│       │                         │    │ Singleflight Request Merger  │  │
│       │                         │    └──────────────┬───────────────┘  │
│       │                         │                   ▼                  │
│       │                         │    ┌──────────────────────────────┐  │
│       │                         │    │ Upstream Fetcher (FS/S3/HTTP)│  │
│       │                         │    │ with SSRF Protection         │  │
│       │                         │    └──────────────┬───────────────┘  │
│       │                         │                   ▼                  │
│       │                         │    ┌──────────────────────────────┐  │
│       │                         │    │ Dynamic Transform Pipeline   │  │
│       │                         │    │ - Lanczos3 SIMD Resampler    │  │
│       │                         │    │ - Sobel Saliency Smart Crop  │  │
│       │                         │    │ - Filters: Blur/Sharpen/Rot  │  │
│       │                         │    │ - WebP / JPEG / PNG Encoders │  │
│       │                         │    └──────────────┬───────────────┘  │
│       │                         │                   ▼                  │
│       │                         │    ┌──────────────────────────────┐  │
│       │                         │    │ Write to L2 Disk & L1 RAM    │  │
│       │                         │    └──────────────┬───────────────┘  │
│       │                         ▼                   │                  │
│       └────────────────► [ HTTP Response Formatter ]◄┘                  │
│                          - RFC 9111 ETag / 304 Not Modified            │
│                          - Cache-Control: public, max-age=31536000     │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Technology Stack

| Component | Choice | Rationale |
| :--- | :--- | :--- |
| **Language and Runtime** | Go 1.23+ | Fast compilation, concurrent goroutines, low memory overhead (< 35MB idle). |
| **HTTP Routing** | `go-chi/chi/v5` | Low-allocation HTTP router conforming to standard `net/http` interfaces. |
| **Image Processing Engine** | Pure Go + SIMD Acceleration | `disintegration/imaging` filter algorithms with Lanczos3, Catmull-Rom, Box, and Nearest-Neighbor resamplers. Zero mandatory CGo dependency for clean cross-compilation. |
| **Format Encoders** | WebP, JPEG, PNG, GIF | Native Go encoders with configurable quality factors and chroma subsampling. |
| **Smart Crop Algorithm** | Sobel Edge Entropy | Convolution gradient analysis identifying regions of highest visual entropy to position the focal window. |
| **Concurrency Safeguard** | `golang.org/x/sync/singleflight` | Merges duplicate concurrent transformation requests, preventing cache stampedes. |
| **Cache Architecture** | Two-Tier (L1 RAM + L2 Disk) | Sharded memory LRU for hot assets plus directory-partitioned disk storage with background quota cleanup. |
| **Web Studio** | React 19 + TypeScript + Vite | Interactive visual cockpit for side-by-side comparison, parameter tuning, and cache inspection. |
| **Styling** | Tailwind CSS + Lucide Icons | Responsive dark theme with clear data density. |

---

## 4. Repository Structure

```
B:\workgit\imgscale\
├── cmd\
│   └── imgscale\
│       └── main.go                 # CLI commands (serve, sign, bench, purge)
├── internal\
│   ├── config\
│   │   └── config.go               # Environment and YAML configuration
│   ├── server\
│   │   ├── server.go               # HTTP server lifecycle and shutdown handling
│   │   ├── router.go               # Route registration and middleware mounting
│   │   ├── handlers.go             # Image delivery, entropy debug, health, and stats endpoints
│   │   └── middleware.go           # HMAC validation, CORS, rate limiting, request tracing
│   ├── engine\
│   │   ├── transformer.go          # Pipeline orchestrator
│   │   ├── resizer.go              # Lanczos3, Catmull-Rom, Box, Linear, and Nearest-Neighbor filters
│   │   ├── crop.go                 # Aspect ratio cropping and focal coordinate alignment
│   │   ├── entropy.go              # Sobel kernel edge detection and focal point detector
│   │   ├── filters.go              # Blur, sharpen, grayscale, brightness, contrast, rotation
│   │   └── format.go               # Format transcoding, MIME negotiation, quality encoding
│   ├── storage\
│   │   ├── source.go               # Storage backend interface
│   │   ├── local.go                # Local filesystem storage provider
│   │   ├── s3.go                   # S3-compatible backend (AWS, Cloudflare R2, MinIO)
│   │   ├── http_upstream.go        # Remote HTTP fetcher with domain whitelist
│   │   └── ssrf.go                 # Network dialer blocking private IPs and loopbacks
│   ├── cache\
│   │   ├── l1_memory.go            # Sharded concurrent memory LRU
│   │   ├── l2_disk.go              # Hashed block storage with TTL and size quotas
│   │   ├── sweeper.go              # Background disk cleaner
│   │   └── etag.go                 # Deterministic ETag generation (FNV-1a / SHA-256)
│   └── telemetry\
│       └── metrics.go              # Cache hit counters, latency tracking, system memory stats
├── web\
│   ├── src\
│   │   ├── components\
│   │   │   ├── BeforeAfterDiff.tsx # Interactive split slider with zoom/pan
│   │   │   ├── UrlBuilder.tsx      # Real-time transformation configurator with code snippets
│   │   │   ├── CacheMetrics.tsx    # Live hit ratios, memory graphs, bandwidth savings
│   │   │   ├── SaliencyOverlay.tsx # Visual heatmap display for smart crop validation
│   │   │   └── SourceExplorer.tsx  # Preset origin gallery and custom upload playground
│   │   ├── App.tsx                 # Developer cockpit layout
│   │   ├── main.tsx                # React 19 mount
│   │   └── index.css               # Tailwind CSS rules
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── testdata\                       # Sample test images (portrait, landscape, diagram, pattern)
├── docs\                           # Documentation guides
│   ├── api.md                      # Query parameter reference and transformation examples
│   ├── deployment.md               # Docker, Kubernetes, systemd, and reverse-proxy guides
│   └── security.md                 # HMAC signing, SSRF mitigation, and rate limiting
├── Makefile                        # Build, test, and release targets
├── Dockerfile                      # Scratch-based multi-stage container
├── LICENSE                         # MIT License
├── README.md                       # Factual project documentation
└── MASTER_PLAN.md                  # This architecture specification
```

---

## 5. Query Parameters and Security Specification

### Transformation Query Parameters

| Parameter | Type | Default | Valid Values | Description |
| :--- | :--- | :--- | :--- | :--- |
| `w` | integer | `0` (auto) | `1` to `8192` | Target width in pixels |
| `h` | integer | `0` (auto) | `1` to `8192` | Target height in pixels |
| `fit` | string | `cover` | `cover`, `contain`, `fill`, `inside`, `outside`, `crop` | Aspect ratio constraint behavior |
| `gravity` | string | `center` | `center`, `north`, `south`, `east`, `west`, `smart` | Crop anchor position. `smart` runs Sobel entropy |
| `format` | string | `auto` | `auto`, `webp`, `jpeg`, `png`, `gif` | Output format. `auto` inspects the HTTP `Accept` header |
| `q` | integer | `80` | `1` to `100` | Output compression quality |
| `dpr` | float | `1.0` | `1.0` to `3.0` | Device pixel ratio scaling multiplier |
| `blur` | float | `0` | `0` to `100` | Gaussian blur radius |
| `sharpen` | float | `0` | `0` to `100` | Unsharp mask intensity |
| `gray` | boolean | `false` | `true`, `false`, `1`, `0` | Convert image to grayscale |
| `rot` | integer | `0` | `0`, `90`, `180`, `270` | Clockwise rotation degrees |
| `debug` | string | none | `entropy`, `meta` | Returns saliency map or JSON transformation metadata |
| `s` | string | optional | HMAC hex string | Required when signature validation is enabled |

### URL Signature Formula
To block URL tampering and unauthorized cache consumption:
$$\text{Signature} = \text{Hex}(\text{HMAC-SHA256}(\text{SecretKey}, \text{Path} + \text{"?"} + \text{SortedQueryStringWithoutS}))$$

---

## 6. Implementation Tasks and Work Breakdown

### Task 1: Environment, Repository Setup and Scaffolding
- [x] Create public GitHub repository `alexandrmotologa/imgscale`.
- [x] Configure repository description and 14 topics covering all four required dimensions.
- [x] Synchronize portfolio registries (`mtlg.site` and `mtlglabs.space`) in active development.
- [x] Initialize Go module (`github.com/alexandrmotologa/imgscale`).
- [x] Add MIT License, standard `.gitignore`, and initial project documentation.

### Task 2: Core Image Processing Engine
- [x] Implement `internal/engine/resizer.go` with Lanczos3, Catmull-Rom, Linear, Box, and Nearest-Neighbor filters.
- [x] Implement `internal/engine/crop.go` for cover, contain, fill, inside, and outside bounding math.
- [x] Implement `internal/engine/entropy.go` using Sobel kernels for visual saliency and focal window detection.
- [x] Implement `internal/engine/filters.go` for blur, sharpen, grayscale, and rotation.
- [x] Implement `internal/engine/format.go` for WebP, JPEG, PNG, and GIF encoding/decoding.
- [x] Implement `internal/engine/transformer.go` pipeline orchestrator.
- [x] Create synthetic test images and unit tests validating image dimensions, aspect ratios, and format outputs.

### Task 3: Storage Layer and SSRF Protection
- [x] Define `internal/storage/source.go` generic image storage interface.
- [x] Implement `internal/storage/local.go` for directory based image retrieval.
- [x] Implement `internal/storage/ssrf.go` with strict network dialer blocking private IPs and cloud metadata addresses.
- [x] Implement `internal/storage/http_upstream.go` for remote image fetching with domain whitelists.
- [x] Implement `internal/storage/s3.go` for AWS S3, Cloudflare R2, and MinIO compatibilities.
- [x] Add unit tests for storage readers and SSRF network rejection rules.

### Task 4: Two-Tier Caching and Singleflight Deduplication
- [x] Implement `internal/cache/l1_memory.go` (sharded concurrent LRU cache with byte limits).
- [x] Implement `internal/cache/l2_disk.go` with atomic temporary writes and two-level directory sharding (`aa/bb/aabb...`).
- [x] Implement `internal/cache/sweeper.go` background goroutine for disk size quota enforcement and TTL purging.
- [x] Implement `internal/cache/etag.go` for RFC 9111 conditional request validation (`ETag` and `If-None-Match`).
- [x] Add singleflight request deduplication to prevent thundering herd during cache misses.
- [x] Unit test L1/L2 cache operations, evictions, and concurrency.

### Task 5: HTTP Gateway, Routing, and Security
- [x] Implement `internal/config/config.go` with environment variable and CLI flag support.
- [x] Implement `internal/server/middleware.go` with HMAC-SHA256 signature verification, CORS, and request metrics.
- [x] Implement `internal/server/router.go` mounting `/v1/images/*`, `/v1/entropy/*`, `/healthz`, and `/api/stats`.
- [x] Implement `internal/server/handlers.go` with `Accept` header format negotiation and response streaming.
- [x] Implement CLI commands in `cmd/imgscale/main.go` (`serve`, `sign`, `bench`).

### Task 6: Embedded React 19 Visual Studio
- [x] Scaffold Vite + React 19 project in `web/` with TypeScript and Tailwind CSS.
- [x] Build `BeforeAfterDiff.tsx` interactive split-slider component.
- [x] Build `UrlBuilder.tsx` query parameter tuner with real-time preview and exportable code snippets.
- [x] Build `SaliencyOverlay.tsx` rendering Sobel edge detection heatmaps.
- [x] Build `CacheMetrics.tsx` displaying live hit ratios, memory usage, and transform timings.
- [x] Build `SourceExplorer.tsx` with curated test images and custom URL input.
- [x] Connect React 19 static build to Go via `//go:embed` mounted at `/ui` and `/`.

### Task 7: Verification, Documentation, and Release
- [x] Run end-to-end tests across all transformation combinations and cache paths.
- [x] Benchmark RPS and latency on standard image datasets.
- [x] Write documentation in `docs/api.md`, `docs/deployment.md`, and `docs/security.md`.
- [x] Write humanized `README.md` without superlatives or ungrounded claims.
- [x] Build multi-stage `Dockerfile` and `Makefile`.
- [x] Verify test suite passes with 100% success rate.
