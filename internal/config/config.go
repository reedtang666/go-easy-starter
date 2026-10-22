package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/encoding/gyaml"
)

type Redis struct {
	Address  string `json:"address"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	Prefix   string `json:"prefix"`
}
type Auth struct {
	Issuer                 string `json:"issuer"`
	Audience               string `json:"audience"`
	AccessSecretKey        string `json:"accessSecretKey"`
	RefreshHashKey         string `json:"refreshHashKey"`
	AccessExpires          int64  `json:"accessExpires"`
	RefreshIdleExpires     int64  `json:"refreshIdleExpires"`
	RefreshAbsoluteExpires int64  `json:"refreshAbsoluteExpires"`
	RefreshReuseGrace      int64  `json:"refreshReuseGrace"`
	Register               bool   `json:"register"`
}
type Storage struct {
	Enabled    bool   `json:"enabled"`
	Driver     string `json:"driver"`
	Root       string `json:"root"`
	PublicURL  string `json:"publicUrl"`
	SigningKey string `json:"signingKey"`
	Endpoint   string `json:"endpoint"`
	Region     string `json:"region"`
	Bucket     string `json:"bucket"`
	AccessKey  string `json:"accessKey"`
	SecretKey  string `json:"secretKey"`
	Secure     bool   `json:"secure"`
	BucketHost string `json:"bucketHost"`
	FileHost   string `json:"fileHost"`
	Prefix     string `json:"prefix"`
}
type Config struct {
	Limits struct {
		AuthPerMinute   int64 `json:"authPerMinute"`
		UploadPerMinute int64 `json:"uploadPerMinute"`
	} `json:"limits"`
	Server struct {
		Address         string   `json:"address"`
		BaseURL         string   `json:"baseUrl"`
		Development     bool     `json:"development"`
		Origins         []string `json:"origins"`
		MaxBodyBytes    int64    `json:"maxBodyBytes"`
		ShutdownSeconds int      `json:"shutdownSeconds"`
	} `json:"server"`
	Database struct {
		URL string `json:"url"`
	} `json:"database"`
	Redis struct {
		Auth  Redis `json:"auth"`
		Cache Redis `json:"cache"`
		Queue Redis `json:"queue"`
	} `json:"redis"`
	Auth    Auth `json:"auth"`
	Storage struct {
		Default          string             `json:"default"`
		MaxBytes         int64              `json:"maxBytes"`
		LinkSeconds      int64              `json:"linkSeconds"`
		MultipartSeconds int64              `json:"multipartSeconds"`
		AllowedTypes     []string           `json:"allowedTypes"`
		Drivers          map[string]Storage `json:"drivers"`
	} `json:"storage"`
	Queue struct {
		Concurrency    int    `json:"concurrency"`
		MaxRetry       int    `json:"maxRetry"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
		Name           string `json:"name"`
		CleanupCron    string `json:"cleanupCron"`
		Timezone       string `json:"timezone"`
	} `json:"queue"`
	WebSocket struct {
		Enabled          bool  `json:"enabled"`
		TicketSeconds    int   `json:"ticketSeconds"`
		MaxConnections   int   `json:"maxConnections"`
		MaxMessageBytes  int64 `json:"maxMessageBytes"`
		HeartbeatSeconds int   `json:"heartbeatSeconds"`
	} `json:"websocket"`
}

func Defaults() Config {
	var c Config
	c.Limits.AuthPerMinute = 30
	c.Limits.UploadPerMinute = 60
	c.Server.Address = ":8000"
	c.Server.BaseURL = "http://localhost:8000"
	c.Server.Development = true
	c.Server.Origins = []string{"http://localhost:5173"}
	c.Server.MaxBodyBytes = 21 << 20
	c.Server.ShutdownSeconds = 30
	c.Database.URL = "postgres://starter:starter@127.0.0.1:5432/starter?sslmode=disable"
	c.Redis.Auth = Redis{Address: "127.0.0.1:6379", DB: 1, Prefix: "starter:auth:"}
	c.Redis.Cache = Redis{Address: "127.0.0.1:6379", DB: 2, Prefix: "starter:cache:"}
	c.Redis.Queue = Redis{Address: "127.0.0.1:6379", DB: 3, Prefix: "asynq:"}
	c.Auth = Auth{Issuer: "go-easy-starter", Audience: "starter-api", AccessExpires: 900, RefreshIdleExpires: 604800, RefreshAbsoluteExpires: 2592000, RefreshReuseGrace: 5, Register: true}
	c.Storage.Default = "local"
	c.Storage.MaxBytes = 20 << 20
	c.Storage.LinkSeconds = 900
	c.Storage.MultipartSeconds = 86400
	c.Storage.AllowedTypes = []string{"image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf", "text/plain", "application/zip"}
	c.Storage.Drivers = map[string]Storage{"local": {Enabled: true, Driver: "local", Root: "storage/uploads"}}
	c.Queue.Concurrency = 10
	c.Queue.MaxRetry = 5
	c.Queue.TimeoutSeconds = 60
	c.Queue.Name = "default"
	c.Queue.CleanupCron = "@every 1h"
	c.Queue.Timezone = "Asia/Shanghai"
	c.WebSocket.TicketSeconds = 60
	c.WebSocket.MaxConnections = 1000
	c.WebSocket.MaxMessageBytes = 65536
	c.WebSocket.HeartbeatSeconds = 30
	return c
}
func Load(path string) (Config, error) {
	c := Defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("read configuration: %w", err)
	}
	j, err := gyaml.ToJson(data)
	if err != nil {
		return c, errors.New("invalid YAML configuration")
	}
	if err = json.Unmarshal(j, &c); err != nil {
		return c, fmt.Errorf("decode configuration: %w", err)
	}
	stringsEnv := map[string]*string{"STARTER_SERVER_ADDRESS": &c.Server.Address, "STARTER_SERVER_BASE_URL": &c.Server.BaseURL, "STARTER_DATABASE_URL": &c.Database.URL, "STARTER_AUTH_ACCESS_KEY": &c.Auth.AccessSecretKey, "STARTER_AUTH_REFRESH_KEY": &c.Auth.RefreshHashKey}
	for key, dst := range stringsEnv {
		if v, ok := os.LookupEnv(key); ok {
			*dst = v
		}
	}
	for _, entry := range []struct {
		name string
		v    *Redis
	}{{"AUTH", &c.Redis.Auth}, {"CACHE", &c.Redis.Cache}, {"QUEUE", &c.Redis.Queue}} {
		for key, dst := range map[string]*string{"ADDRESS": &entry.v.Address, "PASSWORD": &entry.v.Password, "PREFIX": &entry.v.Prefix} {
			if v, ok := os.LookupEnv("STARTER_REDIS_" + entry.name + "_" + key); ok {
				*dst = v
			}
		}
		if v, ok := os.LookupEnv("STARTER_REDIS_" + entry.name + "_DB"); ok {
			n, e := strconv.Atoi(v)
			if e != nil {
				return c, fmt.Errorf("invalid STARTER_REDIS_%s_DB", entry.name)
			}
			entry.v.DB = n
		}
	}
	if v, ok := os.LookupEnv("STARTER_AUTH_ACCESS_SECONDS"); ok {
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil {
			return c, errors.New("invalid STARTER_AUTH_ACCESS_SECONDS")
		}
		c.Auth.AccessExpires = n
	}
	if v, ok := os.LookupEnv("STARTER_DEVELOPMENT"); ok {
		b, e := strconv.ParseBool(v)
		if e != nil {
			return c, errors.New("invalid STARTER_DEVELOPMENT")
		}
		c.Server.Development = b
	}
	for name, driver := range c.Storage.Drivers {
		prefix := "STARTER_STORAGE_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_"
		for key, dst := range map[string]*string{"ACCESS_KEY": &driver.AccessKey, "SECRET_KEY": &driver.SecretKey, "SIGNING_KEY": &driver.SigningKey, "ENDPOINT": &driver.Endpoint, "BUCKET": &driver.Bucket, "PUBLIC_URL": &driver.PublicURL, "ROOT": &driver.Root} {
			if v, ok := os.LookupEnv(prefix + key); ok {
				*dst = v
			}
		}
		c.Storage.Drivers[name] = driver
	}
	return c, nil
}
func (c Config) Validate(role string) error {
	if role != "http" && role != "queue" && role != "scheduler" && role != "all" {
		return errors.New("role must be http, queue, scheduler or all")
	}
	if role == "http" || role == "all" {
		if len(c.Auth.AccessSecretKey) < 32 || len(c.Auth.RefreshHashKey) < 32 || c.Auth.AccessSecretKey == c.Auth.RefreshHashKey {
			return errors.New("auth requires separate accessSecretKey and refreshHashKey of at least 32 bytes; run setup")
		}
		if c.Auth.Issuer == "" || c.Auth.Audience == "" || c.Auth.AccessExpires <= 0 || c.Auth.RefreshIdleExpires <= 0 || c.Auth.RefreshIdleExpires > c.Auth.RefreshAbsoluteExpires || c.Auth.RefreshReuseGrace < 0 {
			return errors.New("invalid auth lifetimes or identity")
		}
		u, err := url.Parse(c.Database.URL)
		if err != nil || u.Host == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
			return errors.New("database.url must be a PostgreSQL URL")
		}
		if c.Storage.MaxBytes <= 0 || c.Storage.LinkSeconds <= 0 || c.Storage.MultipartSeconds <= 0 || len(c.Storage.AllowedTypes) == 0 || c.Server.MaxBodyBytes < c.Storage.MaxBytes {
			return errors.New("invalid upload limits")
		}
		if d, ok := c.Storage.Drivers[c.Storage.Default]; !ok || !d.Enabled {
			return errors.New("storage.default references missing driver")
		}
		if c.Limits.AuthPerMinute <= 0 || c.Limits.UploadPerMinute <= 0 {
			return errors.New("request rate limits must be positive")
		}
		if c.Server.ShutdownSeconds <= 0 {
			return errors.New("server.shutdownSeconds must be positive")
		}
		if c.WebSocket.Enabled && (c.WebSocket.TicketSeconds <= 0 || c.WebSocket.MaxConnections <= 0 || c.WebSocket.HeartbeatSeconds <= 0 || c.WebSocket.MaxMessageBytes <= 0) {
			return errors.New("invalid websocket limits")
		}
	}
	if role != "scheduler" && c.Database.URL == "" {
		return errors.New("database.url required for built-in database tasks")
	}
	if c.Queue.Concurrency <= 0 || c.Queue.MaxRetry < 0 || c.Queue.TimeoutSeconds <= 0 || c.Queue.Name == "" {
		return errors.New("invalid queue settings")
	}
	return nil
}
func Setup(path string) error {
	c := Defaults()
	secret := func() (string, error) {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
	var err error
	if c.Auth.AccessSecretKey, err = secret(); err != nil {
		return err
	}
	if c.Auth.RefreshHashKey, err = secret(); err != nil {
		return err
	}
	d := c.Storage.Drivers["local"]
	if d.SigningKey, err = secret(); err != nil {
		return err
	}
	c.Storage.Drivers["local"] = d
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create configuration (never overwrite): %w", err)
	}
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
