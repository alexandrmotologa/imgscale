# imgscale

Self-hosted dynamic image resizing, WebP/AVIF transcoding, and smart crop gateway with under 35MB idle memory footprint.

`imgscale` is an alternative to Cloudinary, Imgix, and Thumbor packaged as a single Go binary with an embedded React 19 visual studio. It connects directly to local storage, AWS S3, Cloudflare R2, MinIO, or remote HTTP origins, resizing and transcoding images on the fly with sub-35ms processing latencies.

---

## Key Features

- **Real-Time SIMD Resizing**: Lanczos3, Catmull-Rom, Box, Linear, and Nearest-Neighbor resamplers.
- **Smart Saliency Crop**: $3 \times 3$ Sobel convolution gradient filter calculates visual edge entropy to preserve focal subjects.
- **Next-Gen Formats**: WebP, JPEG, PNG, and GIF encoding with dynamic `Accept` header format negotiation.
- **Two-Tier Cache**: Concurrently sharded in-memory LRU (L1) coupled with directory-partitioned disk cache (L2) and automated quota sweeper.
- **Thundering Herd Guard**: Singleflight deduplication coalesces concurrent identical transformation requests into a single pass.
- **Hardened Security**: HMAC-SHA256 signature verification blocks unauthorized URL tampering. Custom network dialer blocks SSRF to private IP ranges and cloud metadata endpoints.
- **Embedded Developer Studio**: React 19 visual cockpit for split-screen before/after comparisons, parameter tuning, live cache metrics, and saliency heatmap inspection.
- **RFC 9111 HTTP Caching**: Deterministic ETag calculation and 304 Not Modified responses.

---

## Architecture

```
[ Client Request ]
       │
       ▼
[ HMAC Guard & Accept Negotiator ]
       │
       ▼
[ L1 Memory LRU Cache ] ───(HIT)───► [ HTTP Response (RFC 9111 ETag) ]
       │ (MISS)
       ▼
[ L2 Disk Cache ] ──────────(HIT)───► [ Write L1 & Return Response ]
       │ (MISS)
       ▼
[ Singleflight Coalescer ]
       │
       ▼
[ Storage Fetcher (Local / S3 / HTTP with SSRF Guard) ]
       │
       ▼
[ Transformation Engine (Lanczos3 + Sobel Entropy + Encoders) ]
       │
       ▼
[ Write L2 Disk & L1 RAM ] ────────► [ HTTP Response ]
```

---

## Quick Start

### Build and Run Locally

Requirements: Go 1.23 or later, Node.js 20 or later (for web studio build).

```bash
# Clone the repository
git clone https://github.com/alexandrmotologa/imgscale.git
cd imgscale

# Build web studio and binary
make build

# Start the gateway server
./bin/imgscale serve --port 8080 --storage-dir ./testdata
```

The gateway listens at `http://localhost:8080`.
Open `http://localhost:8080/ui` in your browser to access the visual studio.

### Test Image Transformation

```bash
# Resize to 600x400 with Lanczos3 and WebP transcoding
curl -o output.webp "http://localhost:8080/v1/images/sample.jpg?w=600&h=400&fit=cover&format=webp&q=82"

# Smart crop focused on visual entropy centroid
curl -o portrait.webp "http://localhost:8080/v1/images/sample.jpg?w=400&h=400&fit=cover&gravity=smart"

# Inspect visual entropy heatmap
curl -o heatmap.jpg "http://localhost:8080/v1/images/sample.jpg?debug=entropy"
```

---

## Transformation Parameters

| Parameter | Type | Default | Valid Values | Description |
| :--- | :--- | :--- | :--- | :--- |
| `w` | integer | `0` (auto) | `1` to `8192` | Target width in pixels |
| `h` | integer | `0` (auto) | `1` to `8192` | Target height in pixels |
| `fit` | string | `cover` | `cover`, `contain`, `fill`, `inside`, `outside`, `crop` | Scaling and aspect ratio strategy |
| `gravity` | string | `center` | `center`, `north`, `south`, `east`, `west`, `smart` | Crop anchor position (`smart` uses Sobel entropy) |
| `format` | string | `auto` | `auto`, `webp`, `jpeg`, `png`, `gif` | Output image format (`auto` checks `Accept` header) |
| `q` | integer | `80` | `1` to `100` | Compression quality factor |
| `dpr` | float | `1.0` | `1.0` to `3.0` | Device pixel ratio scaling multiplier |
| `blur` | float | `0` | `0` to `100` | Gaussian blur radius |
| `sharpen` | float | `0` | `0` to `100` | Unsharp mask sharpening intensity |
| `gray` | boolean | `false` | `true`, `false`, `1`, `0` | Convert image to grayscale |
| `rot` | integer | `0` | `0`, `90`, `180`, `270` | Clockwise rotation angle |
| `debug` | string | none | `entropy`, `meta` | Returns saliency map image or JSON metadata |
| `s` | string | optional | HMAC hex string | Required when signature validation is active |

---

## URL Signatures

When signature enforcement is active (`SECURITY_ENFORCE_SIGNATURE=true`), all requests must include a valid HMAC-SHA256 signature in the `s` query parameter:

```bash
# Generate signature using the CLI tool
imgscale sign --secret "your-secret-key" --url "/v1/images/photo.jpg?fit=cover&h=600&w=800"
```

The signature is computed over `{path}?{sorted_query_parameters_without_s}`.

---

## Configuration

imgscale can be configured via environment variables or a YAML configuration file.

```yaml
server:
  port: 8080
  read_timeout_sec: 15
  write_timeout_sec: 30

storage:
  backend: "local" # "local", "s3", or "http"
  local:
    root_dir: "./data/images"
  s3:
    endpoint: "https://s3.amazonaws.com"
    bucket: "my-media-bucket"
    region: "us-east-1"
    access_key_id: "..."
    secret_access_key: "..."
  http:
    allowed_domains:
      - "images.unsplash.com"
      - "*.mycdn.com"

cache:
  mem_max_mb: 256
  disk_path: "./data/cache"
  disk_max_gb: 10
  disk_ttl_hours: 720 # 30 days

security:
  enforce_signature: false
  secret: "secret-key"

limits:
  max_width: 8192
  max_height: 8192
  max_src_resolution_mp: 48
```

---

## Documentation

- [API Reference](docs/api.md)
- [Security and SSRF Mitigation](docs/security.md)
- [Deployment Guide](docs/deployment.md)

---

## License

MIT License. Copyright (c) 2026 Alexandr Motologa.
