# imgscale Deployment Guide

imgscale compiles to a single binary with zero external runtime dependencies. This document details production deployment patterns using Docker, systemd, and Kubernetes.

---

## 1. Running with Docker

### Quick Start with Local Storage

```bash
docker run -d \
  --name imgscale \
  -p 8080:8080 \
  -v /var/data/images:/data/images:ro \
  -v /var/cache/imgscale:/data/cache \
  -e STORAGE_LOCAL_ROOT=/data/images \
  -e CACHE_DISK_PATH=/data/cache \
  -e CACHE_MEM_MAX_MB=256 \
  -e CACHE_DISK_MAX_GB=10 \
  ghcr.io/alexandrmotologa/imgscale:latest
```

### Docker Compose with S3 / Cloudflare R2

```yaml
version: '3.8'

services:
  imgscale:
    image: ghcr.io/alexandrmotologa/imgscale:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - STORAGE_BACKEND=s3
      - S3_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com
      - S3_BUCKET=production-media
      - S3_REGION=auto
      - S3_ACCESS_KEY_ID=${R2_ACCESS_KEY}
      - S3_SECRET_ACCESS_KEY=${R2_SECRET_KEY}
      - CACHE_DISK_PATH=/cache
      - CACHE_DISK_MAX_GB=20
      - SECURITY_ENFORCE_SIGNATURE=true
      - SECURITY_SECRET=${IMGSCALE_SECRET}
    volumes:
      - imgscale_cache:/cache

volumes:
  imgscale_cache:
```

---

## 2. Running as a systemd Service

1. Place the compiled `imgscale` binary in `/usr/local/bin/imgscale`.
2. Create service user and cache directories:
   ```bash
   sudo useradd -r -s /bin/false imgscale
   sudo mkdir -p /var/cache/imgscale /var/images
   sudo chown -R imgscale:imgscale /var/cache/imgscale
   ```
3. Create `/etc/systemd/system/imgscale.service`:
   ```ini
   [Unit]
   Description=imgscale Media Processing Gateway
   After=network.target

   [Service]
   Type=simple
   User=imgscale
   Group=imgscale
   ExecStart=/usr/local/bin/imgscale serve --config /etc/imgscale/config.yaml
   Restart=always
   RestartSec=5s
   LimitNOFILE=65536
   MemoryHigh=512M
   MemoryMax=1G

   [Install]
   WantedBy=multi-user.target
   ```
4. Enable and start:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now imgscale
   ```

---

## 3. Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: imgscale
  labels:
    app: imgscale
spec:
  replicas: 3
  selector:
    matchLabels:
      app: imgscale
  template:
    metadata:
      labels:
        app: imgscale
    spec:
      containers:
      - name: gateway
        image: ghcr.io/alexandrmotologa/imgscale:v1.0.0
        ports:
        - containerPort: 8080
        resources:
          requests:
            cpu: 100m
            memory: 64Mi
          limits:
            cpu: 1000m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 3
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 2
          periodSeconds: 5
        env:
        - name: CACHE_MEM_MAX_MB
          value: "128"
        - name: STORAGE_BACKEND
          value: "s3"
        - name: S3_BUCKET
          value: "media-bucket"
```

---

## 4. Reverse Proxy Setup (Nginx)

```nginx
upstream imgscale_backend {
    server 127.0.0.1:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name images.example.com;

    location / {
        proxy_pass http://imgscale_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Allow client browser caching
        expires 365d;
        add_header Cache-Control "public, no-transform";
    }
}
```
