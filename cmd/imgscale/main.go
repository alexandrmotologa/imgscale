package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/alexandrmotologa/imgscale/internal/cache"
	"github.com/alexandrmotologa/imgscale/internal/config"
	"github.com/alexandrmotologa/imgscale/internal/engine"
	"github.com/alexandrmotologa/imgscale/internal/server"
	"github.com/alexandrmotologa/imgscale/internal/storage"
	"github.com/alexandrmotologa/imgscale/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "serve":
		cmdServe(os.Args[2:])
	case "sign":
		cmdSign(os.Args[2:])
	case "bench":
		cmdBench(os.Args[2:])
	case "purge":
		cmdPurge(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`imgscale: Real-Time Dynamic Image Resizing, WebP/AVIF Transcoding & Smart Crop Gateway

Usage:
  imgscale <command> [options]

Commands:
  serve      Start the image processing gateway HTTP daemon and embedded visual studio
  sign       Generate an HMAC-SHA256 signature for a transformation URL
  bench      Run image processing and smart crop performance benchmarks
  purge      Flush and prune cached image blocks from disk
  help       Show this help message

Use "imgscale <command> --help" for more information about a command.`)
}

func cmdServe(args []string) {
	fsFlags := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fsFlags.Int("port", 0, "HTTP server port (default 8080)")
	storageBackend := fsFlags.String("storage", "", "Storage backend: local, s3, http")
	storageDir := fsFlags.String("storage-dir", "", "Local storage root directory for images")
	cacheDir := fsFlags.String("cache-dir", "", "L2 persistent disk cache directory")
	memMaxMB := fsFlags.Int("mem-mb", 0, "L1 memory cache maximum MB")
	diskMaxGB := fsFlags.Int("disk-gb", 0, "L2 disk cache maximum GB")
	secret := fsFlags.String("secret", "", "Shared HMAC secret for URL signing")
	enforceSig := fsFlags.Bool("enforce-sig", false, "Strictly require HMAC signatures on all requests")

	_ = fsFlags.Parse(args)

	cfg := config.LoadConfig()
	if *port > 0 {
		cfg.Server.Port = *port
	}
	if *storageDir != "" {
		cfg.Storage.Local.RootDir = *storageDir
	}
	if *cacheDir != "" {
		cfg.Cache.L2Dir = *cacheDir
	}
	if *memMaxMB > 0 {
		cfg.Cache.L1MaxBytes = int64(*memMaxMB) * 1024 * 1024
	}
	if *diskMaxGB > 0 {
		cfg.Cache.L2MaxBytes = int64(*diskMaxGB) * 1024 * 1024 * 1024
	}
	if *secret != "" {
		cfg.Security.Secret = *secret
	}
	if *enforceSig {
		cfg.Security.EnforceSignature = true
	}
	if *storageBackend != "" {
		cfg.Storage.Backend = *storageBackend
	}

	// Initialize storage source
	var store storage.StorageSource
	var err error

	switch cfg.Storage.Backend {
	case "local":
		store, err = storage.NewLocalStorage(cfg.Storage.Local.RootDir)
		if err != nil {
			log.Fatalf("Failed to initialize local storage: %v", err)
		}
	case "s3":
		store = storage.NewS3Storage(storage.S3Config{
			Endpoint:        cfg.Storage.S3.Endpoint,
			Bucket:          cfg.Storage.S3.Bucket,
			Region:          cfg.Storage.S3.Region,
			AccessKeyID:     cfg.Storage.S3.AccessKeyID,
			SecretAccessKey: cfg.Storage.S3.SecretAccessKey,
			PathStyle:       cfg.Storage.S3.PathStyle,
		}, 20*time.Second)
	case "http":
		store = storage.NewHTTPStorage(
			cfg.Storage.HTTP.BaseURL,
			storage.SSRFPolicy{AllowedDomains: cfg.Storage.HTTP.AllowedDomains},
			cfg.Limits.MaxPayloadBytes,
			15*time.Second,
		)
	default:
		log.Fatalf("Unknown storage backend: %s", cfg.Storage.Backend)
	}

	// Initialize Two-Tier Cache
	tc, err := cache.NewTwoTierCache(cache.Config{
		L1MaxBytes: cfg.Cache.L1MaxBytes,
		L2Dir:      cfg.Cache.L2Dir,
		L2MaxBytes: cfg.Cache.L2MaxBytes,
		L2MaxTTL:   cfg.Cache.L2MaxTTL,
	})
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}

	// Embedded UI
	var uiFS fs.FS
	if dist, err := ui.GetFS(); err == nil {
		uiFS = dist
	} else {
		log.Printf("Embedded UI not found (%v), serving API only", err)
	}

	srv := server.NewServer(cfg, store, tc, uiFS)
	if err := srv.Run(); err != nil {
		log.Fatalf("Gateway server terminated: %v", err)
	}
}

func cmdSign(args []string) {
	fsFlags := flag.NewFlagSet("sign", flag.ExitOnError)
	targetURL := fsFlags.String("url", "", "Transformation URL to sign (e.g. /v1/images/avatar.jpg?w=800&fit=cover)")
	secret := fsFlags.String("secret", "", "Shared HMAC secret key")
	_ = fsFlags.Parse(args)

	if *targetURL == "" || *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: both --url and --secret flags are required.")
		fsFlags.Usage()
		os.Exit(1)
	}

	signed, err := server.SignURL(*targetURL, *secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error signing URL: %v\n", err)
		os.Exit(1)
	}

	parsed, _ := url.Parse(signed)
	sig := parsed.Query().Get("s")

	fmt.Printf("Signature:  %s\n", sig)
	fmt.Printf("Signed URL: %s\n", signed)
}

func cmdBench(args []string) {
	fsFlags := flag.NewFlagSet("bench", flag.ExitOnError)
	imagePath := fsFlags.String("image", "./testdata/portrait.png", "Source image to benchmark")
	iterations := fsFlags.Int("n", 20, "Number of benchmark iterations")
	_ = fsFlags.Parse(args)

	file, err := os.Open(*imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening image: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading image: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Benchmarking image %s (%d bytes, %d iterations)...\n\n", *imagePath, len(data), *iterations)

	// Test 1: Lanczos3 Resize to 600x400
	var totalLanczos time.Duration
	for i := 0; i < *iterations; i++ {
		start := time.Now()
		_, err := engine.Transform(readerFrom(data), engine.TransformOptions{
			Width:   600,
			Height:  400,
			Fit:     engine.FitCover,
			Filter:  engine.FilterLanczos,
			Format:  engine.FormatJPEG,
			Quality: 80,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Transform failed: %v\n", err)
			os.Exit(1)
		}
		totalLanczos += time.Since(start)
	}
	avgLanczos := totalLanczos / time.Duration(*iterations)
	fmt.Printf("1. Lanczos3 SIMD Resize (600x400 JPEG):  %v avg (%.1f ops/sec)\n", avgLanczos, float64(time.Second)/float64(avgLanczos))

	// Test 2: Sobel Edge Entropy Smart Crop
	srcImg, _, err := engine.DecodeImage(readerFrom(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Decode failed: %v\n", err)
		os.Exit(1)
	}
	var totalEntropy time.Duration
	for i := 0; i < *iterations; i++ {
		start := time.Now()
		_ = engine.FindSmartCrop(srcImg, 400, 400)
		totalEntropy += time.Since(start)
	}
	avgEntropy := totalEntropy / time.Duration(*iterations)
	fmt.Printf("2. Sobel Visual Entropy Smart Crop:       %v avg (%.1f ops/sec)\n", avgEntropy, float64(time.Second)/float64(avgEntropy))

	// Test 3: WebP Transcoding
	var totalWebP time.Duration
	for i := 0; i < *iterations; i++ {
		start := time.Now()
		_, err := engine.Transform(readerFrom(data), engine.TransformOptions{
			Width:   400,
			Height:  400,
			Fit:     engine.FitCover,
			Gravity: engine.GravitySmart,
			Format:  engine.FormatWebP,
			Quality: 80,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "WebP transform failed: %v\n", err)
			os.Exit(1)
		}
		totalWebP += time.Since(start)
	}
	avgWebP := totalWebP / time.Duration(*iterations)
	fmt.Printf("3. WebP Transcode + Smart Crop:          %v avg (%.1f ops/sec)\n", avgWebP, float64(time.Second)/float64(avgWebP))
}

func cmdPurge(args []string) {
	fsFlags := flag.NewFlagSet("purge", flag.ExitOnError)
	cacheDir := fsFlags.String("cache-dir", "./data/cache", "Cache directory to purge")
	key := fsFlags.String("key", "", "Specific image path or prefix to purge (e.g. avatar.jpg)")
	_ = fsFlags.Parse(args)

	l2, err := cache.NewL2DiskCache(*cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error accessing cache directory: %v\n", err)
		os.Exit(1)
	}

	count, err := l2.PurgePrefix(*key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error purging cache: %v\n", err)
		os.Exit(1)
	}

	if *key != "" {
		fmt.Printf("Successfully purged %d cached items matching key %q from %s\n", count, *key, *cacheDir)
	} else {
		fmt.Printf("Successfully purged all %d cached items from %s\n", count, *cacheDir)
	}
}

type byteReader struct {
	data []byte
	pos  int
}

func readerFrom(data []byte) io.Reader {
	return &byteReader{data: data, pos: 0}
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
