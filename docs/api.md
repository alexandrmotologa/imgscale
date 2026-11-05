# imgscale API Reference

imgscale processes images dynamically via HTTP GET requests. Transformations are controlled using query parameters.

---

## Endpoint: Image Transformation

`GET /v1/images/{path...}`

Transforms the origin image located at `{path...}` according to the supplied query parameters.

### Query Parameters

| Parameter | Type | Default | Description | Example |
| :--- | :--- | :--- | :--- | :--- |
| `w` | integer | `0` (auto) | Target width in pixels (1 to 8192). | `w=800` |
| `h` | integer | `0` (auto) | Target height in pixels (1 to 8192). | `h=600` |
| `fit` | string | `cover` | Resize fit strategy: `cover`, `contain`, `fill`, `inside`, `outside`, `crop`. | `fit=contain` |
| `gravity` | string | `center` | Crop anchor position: `center`, `north`, `south`, `east`, `west`, `northeast`, `northwest`, `southeast`, `southwest`, `smart`, `fp`. | `gravity=smart` |
| `fp_x` | float | `0.5` | Focal point horizontal coordinate from 0.0 (left) to 1.0 (right). Active when `gravity=fp`. | `fp_x=0.65` |
| `fp_y` | float | `0.5` | Focal point vertical coordinate from 0.0 (top) to 1.0 (bottom). Active when `gravity=fp`. | `fp_y=0.35` |
| `format` | string | `auto` | Output image format: `auto`, `webp`, `jpeg`, `png`, `gif`. When `auto`, the gateway inspects the client `Accept` header. | `format=webp` |
| `q` | integer | `80` | Compression quality factor from 1 to 100 (lossy formats). | `q=85` |
| `dpr` | float | `1.0` | Device pixel ratio multiplier from 1.0 to 3.0. Multiplies width and height before scaling. | `dpr=2.0` |
| `blur` | float | `0` | Gaussian blur sigma radius (0 to 100). | `blur=5` |
| `sharpen` | float | `0` | Unsharp mask sharpening intensity (0 to 100). | `sharpen=15` |
| `gray` | boolean | `false` | Converts the output image to grayscale (`true` or `1`). | `gray=true` |
| `rot` | integer | `0` | Clockwise rotation angle: `0`, `90`, `180`, `270`. | `rot=90` |
| `auto_orient` | boolean | `true` | Automatically corrects orientation based on EXIF tag metadata (tags 1 to 8). | `auto_orient=true` |
| `lqip` | string | none | Generates low-quality placeholders: `datauri` (Base64 string), `json` (metadata + dominant color), `blur` (16px micro-image). | `lqip=datauri` |
| `wm_image` | string | none | Relative path or filename of overlay image (e.g., logo or badge). | `wm_image=badge.png` |
| `wm_opacity` | float | `0.5` | Watermark opacity multiplier between 0.0 and 1.0. | `wm_opacity=0.7` |
| `wm_scale` | integer | `20` | Watermark width as a percentage of background width (5% to 50%). | `wm_scale=25` |
| `wm_gravity` | string | `southeast` | Watermark anchor position: `southeast`, `southwest`, `center`, `northeast`, `northwest`. | `wm_gravity=southeast` |
| `wm_pad` | integer | `10` | Watermark margin in pixels from the canvas edge. | `wm_pad=16` |
| `debug` | string | none | Set to `entropy` to output a visual saliency heatmap, or `meta` to output JSON image metadata. | `debug=entropy` |
| `s` | string | optional | HMAC-SHA256 signature hex digest. Required when signature enforcement is active. | `s=a1b2c3d4...` |

---

## Fit Modes

- **cover**: Scales the image preserving aspect ratio to fill the entire target rectangle, cropping overflow pixels.
- **contain**: Scales the image preserving aspect ratio so the entire image fits within the target rectangle without cropping.
- **fill**: Stretches the image directly to match target width and height, ignoring aspect ratio.
- **inside**: Scales the image down so its dimensions do not exceed the target rectangle. Never enlarges.
- **outside**: Scales the image so its smaller dimension matches the target, preserving aspect ratio.
- **crop**: Crops directly to the requested dimensions using the chosen gravity anchor without pre-scaling.

---

## Smart Crop with Sobel Edge Entropy

When `gravity=smart` is requested with `fit=cover` or `fit=crop`:
1. The engine computes horizontal and vertical image gradients using 3x3 Sobel convolution kernels:
   $$\begin{aligned} G_x = \begin{bmatrix} -1 & 0 & 1 \\ -2 & 0 & 2 \\ -1 & 0 & 1 \end{bmatrix}, \quad G_y = \begin{bmatrix} -1 & -2 & -1 \\ 0 & 0 & 0 \\ 1 & 2 & 1 \end{bmatrix} \end{aligned}$$
2. Gradient magnitude $M(x, y) = \sqrt{G_x^2 + G_y^2}$ produces an edge density map.
3. A Summed Area Table (SAT) calculates rectangle entropy sums in $O(1)$ time across candidate windows.
4. The crop window containing highest visual entropy is selected.

---

## Low-Quality Image Placeholders (LQIP)

Use `lqip` to load instant placeholders for web applications:

- `lqip=datauri`: Returns a raw Base64 Data-URI (`data:image/webp;base64,...`) ready to inject into `src` attributes.
- `lqip=json`: Returns JSON metadata containing dimensions, byte size, dominant color in `#RRGGBB` format, and the Data-URI:
  ```json
  {
    "data_uri": "data:image/webp;base64,UklGR...",
    "dominant_color": "#606673",
    "width": 800,
    "height": 1200,
    "byte_size": 90
  }
  ```
- `lqip=blur`: Returns a miniature 16-pixel WebP binary image with Gaussian blur for CSS blur-up effects.

---

## HTTP Caching and Conditional Requests

imgscale implements RFC 9111 HTTP caching:
- **Cache-Control**: `public, max-age=31536000, immutable` for signed or static asset routes.
- **ETag**: Deterministic hash incorporating origin modification timestamp and transformation parameters.
- **304 Not Modified**: Returned when client sends matching `If-None-Match`.

---

## Operational and Management Endpoints

### Health Check
`GET /healthz`
Returns gateway status and uptime in seconds.

### Telemetry Stats
`GET /api/stats`
Returns JSON stats covering L1 RAM cache, L2 disk cache, and Go runtime allocations.

### Prometheus Metrics
`GET /metrics`
Returns raw Prometheus text metrics:
- `imgscale_requests_total`: Total HTTP requests counter.
- `imgscale_cache_hits_total{tier="l1"}`: L1 memory hits.
- `imgscale_cache_hits_total{tier="l2"}`: L2 disk hits.
- `imgscale_cache_misses_total`: Cache misses.
- `imgscale_transform_duration_seconds`: Histogram of transform processing latency.
- `imgscale_memory_bytes`: In-use heap memory.

### Cache Purge
`DELETE /api/cache/purge`
Purges cached variants. Pass an optional query parameter `path=image.png` to purge variants of a specific image, or omit the parameter to clear both L1 memory and L2 disk caches entirely.

### Custom Image Upload
`POST /api/upload`
Uploads a local image (multipart form with field `image`) into the storage directory for instant testing.

### URL Signing
`POST /api/sign`
Accepts `{ "url": "...", "secret": "..." }` and returns `{ "signature": "hex", "signed_url": "..." }`.

### Presets
`GET /api/presets`
Returns metadata list of sample images stored in the storage directory.
