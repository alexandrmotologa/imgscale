package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents complete gateway configuration.
type Config struct {
	Server   ServerConfig   `json:"server"`
	Storage  StorageConfig  `json:"storage"`
	Cache    CacheConfig    `json:"cache"`
	Security SecurityConfig `json:"security"`
	Limits   LimitsConfig   `json:"limits"`
}

type ServerConfig struct {
	Port         int           `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
}

type StorageConfig struct {
	Backend string             `json:"backend"` // "local", "s3", "http"
	Local   LocalStorageConfig `json:"local"`
	S3      S3StorageConfig    `json:"s3"`
	HTTP    HTTPStorageConfig  `json:"http"`
}

type LocalStorageConfig struct {
	RootDir string `json:"root_dir"`
}

type S3StorageConfig struct {
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	PathStyle       bool   `json:"path_style"`
}

type HTTPStorageConfig struct {
	BaseURL        string   `json:"base_url"`
	AllowedDomains []string `json:"allowed_domains"`
}

type CacheConfig struct {
	L1MaxBytes int64         `json:"l1_max_bytes"`
	L2Dir      string        `json:"l2_dir"`
	L2MaxBytes int64         `json:"l2_max_bytes"`
	L2MaxTTL   time.Duration `json:"l2_max_ttl"`
}

type SecurityConfig struct {
	EnforceSignature bool   `json:"enforce_signature"`
	Secret           string `json:"secret"`
}

type LimitsConfig struct {
	MaxWidth           int   `json:"max_width"`
	MaxHeight          int   `json:"max_height"`
	MaxSrcResolutionMP int   `json:"max_src_resolution_mp"`
	MaxPayloadBytes    int64 `json:"max_payload_bytes"`
}

// LoadConfig loads configuration from environment variables with sensible defaults.
func LoadConfig() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnvInt("PORT", 8080),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Storage: StorageConfig{
			Backend: getEnv("STORAGE_BACKEND", "local"),
			Local: LocalStorageConfig{
				RootDir: getEnv("STORAGE_LOCAL_ROOT", "./data/images"),
			},
			S3: S3StorageConfig{
				Endpoint:        getEnv("S3_ENDPOINT", "https://s3.amazonaws.com"),
				Bucket:          getEnv("S3_BUCKET", ""),
				Region:          getEnv("S3_REGION", "us-east-1"),
				AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", ""),
				SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", ""),
				PathStyle:       getEnvBool("S3_PATH_STYLE", false),
			},
			HTTP: HTTPStorageConfig{
				BaseURL:        getEnv("STORAGE_HTTP_BASE_URL", ""),
				AllowedDomains: getEnvSlice("STORAGE_HTTP_ALLOWED_DOMAINS", []string{}),
			},
		},
		Cache: CacheConfig{
			L1MaxBytes: int64(getEnvInt("CACHE_MEM_MAX_MB", 64)) * 1024 * 1024,
			L2Dir:      getEnv("CACHE_DISK_PATH", "./data/cache"),
			L2MaxBytes: int64(getEnvInt("CACHE_DISK_MAX_GB", 10)) * 1024 * 1024 * 1024,
			L2MaxTTL:   time.Duration(getEnvInt("CACHE_DISK_TTL_HOURS", 720)) * time.Hour,
		},
		Security: SecurityConfig{
			EnforceSignature: getEnvBool("SECURITY_ENFORCE_SIGNATURE", false),
			Secret:           getEnv("SECURITY_SECRET", ""),
		},
		Limits: LimitsConfig{
			MaxWidth:           getEnvInt("LIMITS_MAX_WIDTH", 8192),
			MaxHeight:          getEnvInt("LIMITS_MAX_HEIGHT", 8192),
			MaxSrcResolutionMP: getEnvInt("LIMITS_MAX_SRC_RESOLUTION_MP", 48),
			MaxPayloadBytes:    int64(getEnvInt("LIMITS_MAX_PAYLOAD_MB", 50)) * 1024 * 1024,
		},
	}
	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		v := strings.ToLower(val)
		return v == "1" || v == "true" || v == "yes"
	}
	return defaultVal
}

func getEnvSlice(key string, defaultVal []string) []string {
	if val := os.Getenv(key); val != "" {
		parts := strings.Split(val, ",")
		var out []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return defaultVal
}
