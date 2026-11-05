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
| `gravity` | string | `center` | Crop anchor position: `center`, `north`, `south`, `east`, `west`, `smart`. `smart` uses Sobel edge entropy to preserve focal subjects. | `gravity=smart` |
| `format` | string | `auto` | Output image format: `auto`, `webp`, `jpeg`, `png`, `gif`. When `auto`, the gateway inspects the client `Accept` header. | `format=webp` |
| `q` | integer | `80` | Compression quality factor from 1 to 100 (lossy formats). | `q=85` |
| `dpr` | float | `1.0` | Device pixel ratio multiplier from 1.0 to 3.0. Multiplies width and height before scaling. | `dpr=2.0` |
| `blur` | float | `0` | Gaussian blur sigma radius (0 to 100). | `blur=5` |
| `sharpen` | float | `0` | Unsharp mask sharpening intensity (0 to 100). | `sharpen=15` |
| `gray` | boolean | `false` | Converts the output image to grayscale (`true` or `1`). | `gray=true` |
| `rot` | integer | `0` | Clockwise rotation angle: `0`, `90`, `180`, `270`. | `rot=90` |
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
1. The engine computes horizontal and vertical image gradients using $3 \times 3$ Sobel convolution kernels:
   $$G_x = \begin{bmatrix} -1 & 0 & 1 \\ -2 & 0 & 2 \\ -1 & 0 & 1 \end{bmatrix}, \quad G_y = \begin{bmatrix} -1 & -2 & -1 \\ 0 & 0 & 0 \\ 1 & 2 & 1 \end{bmatrix}$$
2. Gradient magnitude $M(x, y) = \sqrt{G_x^2 + G_y^2}$ produces an edge density map.
3. A sliding window evaluates candidate crop frames to locate the sub-region containing the highest total visual entropy.
4. The crop coordinates center on this focal centroid.

---

## HTTP Caching and Conditional Requests

imgscale implements RFC 9111 HTTP caching:
- **Cache-Control**: `public, max-age=31536000, immutable` for signed or static asset routes.
- **ETag**: Deterministic hash incorporating origin modification timestamp and transformation parameters.
- **304 Not Modified**: Returned when client sends matching `If-None-Match`.

---

## System Endpoints

- `GET /healthz`: Health status indicator returning `{ "status": "ok", "uptime_sec": 120 }`.
- `GET /api/stats`: Real-time cache metrics including L1 memory usage, L2 disk usage, hit ratios, and processed counts.
- `GET /ui`: Web cockpit and visual before/after comparison studio.
