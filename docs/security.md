# imgscale Security Guide

imgscale implements three layers of security to prevent resource exhaustion, remote server-side request forgery (SSRF), and cache flooding.

---

## 1. URL Signatures with HMAC-SHA256

Unsigned dynamic transformation endpoints allow malicious clients to generate millions of slightly different parameter variations (for example `w=100`, `w=101`, `w=102`), causing CPU exhaustion and filling cache storage.

When signature enforcement is enabled (`SECURITY_ENFORCE_SIGNATURE=true`), imgscale requires an `s` query parameter containing a valid HMAC-SHA256 hex digest.

### Signature Calculation Formula

1. Sort all query parameters alphabetically by key, excluding the `s` parameter itself.
2. Build the signing payload string:
   `{canonical_path}?{sorted_query_string}`
   Example:
   Path: `/v1/images/hero.jpg`
   Params: `fit=cover&format=webp&h=600&q=80&w=800`
   Payload: `/v1/images/hero.jpg?fit=cover&format=webp&h=600&q=80&w=800`
3. Compute the HMAC-SHA256 digest using your shared secret:
   $$\text{digest} = \text{Hex}(\text{HMAC-SHA256}(\text{secret}, \text{payload}))$$
4. Append `&s={digest}` to the final request URL.

### Command Line Signature Utility

Generate a signed URL using the CLI:
```bash
imgscale sign --secret "your-secret-key" --url "http://localhost:8080/v1/images/hero.jpg?w=800&h=600&fit=cover"
```

---

## 2. Server-Side Request Forgery (SSRF) Protection

When configured with an HTTP upstream proxy backend (`STORAGE_BACKEND=http`), imgscale fetches source images from external URLs. To prevent attackers from using the gateway to scan internal networks or query cloud metadata services, the HTTP client enforces network-level validation:

1. **Private Subnet Blocking**:
   - IPv4 loopback: `127.0.0.0/8`
   - IPv6 loopback: `::1`
   - RFC 1918 subnets: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`
   - Link-local and cloud metadata: `169.254.0.0/16` (including `169.254.169.254`)
   - IPv6 unique local: `fc00::/7`
   - Multicast: `224.0.0.0/4`, `ff00::/8`

2. **Domain Whitelisting**:
   Configure allowed host patterns in `config.yaml` or via `STORAGE_HTTP_ALLOWED_DOMAINS`:
   ```yaml
   storage:
     http:
       allowed_domains:
         - "cdn.example.com"
         - "*.storage.example.com"
   ```

3. **DNS Rebinding Prevention**:
   The custom dialer resolves the target hostname and checks all resolved IP addresses immediately prior to socket connection. If any IP belongs to a restricted subnet, the connection is refused.

---

## 3. Rate Limiting and Maximum Dimensions

To prevent Denial of Service via large image allocation:
- **Maximum Input Resolution**: Configurable via `LIMITS_MAX_SRC_RESOLUTION_MP` (default: 48 Megapixels). Images exceeding this limit are rejected before full decoding.
- **Maximum Output Dimensions**: Clamped to `LIMITS_MAX_WIDTH` and `LIMITS_MAX_HEIGHT` (default: 4096px).
- **Concurrency Singleflight**: Duplicate concurrent requests for identical uncached images share a single decoding and transformation pass.
