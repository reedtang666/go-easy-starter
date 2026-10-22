//go:build integration

package bootstrap_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/reedtang666/go-easy-starter/internal/bootstrap"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"github.com/reedtang666/go-easy-starter/internal/service"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type testAPI struct {
	t      *testing.T
	base   string
	client *http.Client
}

func (a testAPI) request(method, path, token string, body any, status int) map[string]any {
	a.t.Helper()
	var data io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			a.t.Fatal(e)
		}
		data = bytes.NewReader(b)
	}
	r, e := http.NewRequest(method, a.base+path, data)
	if e != nil {
		a.t.Fatal(e)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, e := a.client.Do(r)
	if e != nil {
		a.t.Fatal(e)
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(resp.Body)
	if e != nil {
		a.t.Fatal(e)
	}
	if resp.StatusCode != status {
		a.t.Fatalf("%s %s expected=%d actual=%d body=%s", method, path, status, resp.StatusCode, b)
	}
	var result map[string]any
	if e = json.Unmarshal(b, &result); e != nil {
		a.t.Fatalf("invalid JSON %s: %v", b, e)
	}
	if _, ok := result["traceId"]; !ok {
		a.t.Fatal("missing trace ID")
	}
	return result
}
func data(v map[string]any) map[string]any { return v["data"].(map[string]any) }
func (a testAPI) upload(path, method, token, name string, b []byte, status int) map[string]any {
	a.t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, e := w.CreateFormFile("file", name)
	if e != nil {
		a.t.Fatal(e)
	}
	if _, e = part.Write(b); e != nil {
		a.t.Fatal(e)
	}
	if e = w.Close(); e != nil {
		a.t.Fatal(e)
	}
	r, e := http.NewRequest(method, a.base+path, &body)
	if e != nil {
		a.t.Fatal(e)
	}
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+token)
	resp, e := a.client.Do(r)
	if e != nil {
		a.t.Fatal(e)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != status {
		a.t.Fatalf("upload %s expected %d got %d body=%s", path, status, resp.StatusCode, raw)
	}
	var out map[string]any
	if e = json.Unmarshal(raw, &out); e != nil {
		a.t.Fatal(e, string(raw))
	}
	return out
}
func integrationConfig(t *testing.T) config.Config {
	t.Helper()
	p := t.TempDir() + "/config.yaml"
	if e := config.Setup(p); e != nil {
		t.Fatal(e)
	}
	c, e := config.Load(p)
	if e != nil {
		t.Fatal(e)
	}
	c.Database.URL = os.Getenv("STARTER_TEST_DATABASE_URL")
	u, e := url.Parse(c.Database.URL)
	if e != nil || !strings.HasSuffix(u.Path, "_test") {
		t.Fatal("STARTER_TEST_DATABASE_URL must name an isolated *_test database")
	}
	address := os.Getenv("STARTER_TEST_REDIS_ADDRESS")
	if address == "" {
		t.Fatal("STARTER_TEST_REDIS_ADDRESS required")
	}
	c.Redis.Auth.Address = address
	c.Redis.Cache.Address = address
	c.Redis.Queue.Address = address
	c.Redis.Auth.DB = 11
	c.Redis.Cache.DB = 12
	c.Redis.Queue.DB = 13
	c.Redis.Auth.Prefix = fmt.Sprintf("it:%d:", time.Now().UnixNano())
	c.Server.Address = "127.0.0.1:0"
	c.Server.ShutdownSeconds = 2
	c.Queue.Concurrency = 2
	c.Queue.Name = fmt.Sprintf("it-%d", time.Now().UnixNano())
	local := c.Storage.Drivers["local"]
	local.Root = t.TempDir()
	c.Storage.Drivers["local"] = local
	c.WebSocket.Enabled = true
	c.WebSocket.HeartbeatSeconds = 1
	return c
}
func TestFullAPIAndWorker(t *testing.T) {
	c := integrationConfig(t)
	app, e := bootstrap.New(context.Background(), c, "all")
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	if e = app.Start(); e != nil {
		t.Fatal(e)
	}
	api := testAPI{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", app.HTTP.GetListenedPort()), client: &http.Client{Timeout: 10 * time.Second}}
	api.request("GET", "/health/live", "", nil, 200)
	api.request("GET", "/health/ready", "", nil, 200)
	api.request("GET", "/unknown", "", nil, 404)
	api.request("GET", "/api/v1/user/me", "", nil, 401)
	name := fmt.Sprintf("u%d", time.Now().UnixNano())
	password := "testPassword-123"
	register := map[string]any{"username": name, "password": password}
	api.request("POST", "/api/v1/auth/register", "", register, 200)
	api.request("POST", "/api/v1/auth/register", "", register, 409)
	api.request("POST", "/api/v1/auth/register", "", map[string]any{"username": "bad", "password": "short"}, 400)
	login := map[string]any{"username": name, "password": password, "deviceId": "integration"}
	pair := data(api.request("POST", "/api/v1/auth/login", "", login, 200))
	token := pair["accessToken"].(string)
	refresh := pair["refreshToken"].(string)
	me := api.request("GET", "/api/v1/user/me", token, nil, 200)
	raw, _ := json.Marshal(me)
	if bytes.Contains(raw, []byte("passwordHash")) || bytes.Contains(raw, []byte("credentialVersion")) {
		t.Fatal("private user fields exposed")
	}
	pair = data(api.request("POST", "/api/v1/auth/refresh", "", map[string]any{"refreshToken": refresh, "deviceId": "integration"}, 200))
	token = pair["accessToken"].(string)
	other := name + "b"
	api.request("POST", "/api/v1/auth/register", "", map[string]any{"username": other, "password": password}, 200)
	otherToken := data(api.request("POST", "/api/v1/auth/login", "", map[string]any{"username": other, "password": password, "deviceId": "b"}, 200))["accessToken"].(string)
	item := data(api.request("POST", "/api/v1/example/items", token, map[string]any{"title": "hello", "content": "world"}, 200))["item"].(map[string]any)
	itemID := int64(item["id"].(float64))
	itemPath := fmt.Sprintf("/api/v1/example/items/%d", itemID)
	api.request("GET", itemPath, otherToken, nil, 404)
	api.request("GET", "/api/v1/example/items?pageSize=101", token, nil, 400)
	api.request("POST", "/api/v1/example/tasks", token, map[string]any{"itemId": itemID, "key": "once"}, 200)
	api.request("POST", "/api/v1/example/tasks", token, map[string]any{"itemId": itemID, "key": "once"}, 200)
	deadline := time.Now().Add(15 * time.Second)
	for {
		v := data(api.request("GET", itemPath, token, nil, 200))["item"].(map[string]any)
		if v["taskRuns"].(float64) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not apply task")
		}
		time.Sleep(100 * time.Millisecond)
	}
	f := data(api.upload("/api/v1/files", "POST", token, "hello.txt", []byte("hello from integration"), 200))["file"].(map[string]any)
	fileID := int64(f["id"].(float64))
	filePath := fmt.Sprintf("/api/v1/files/%d", fileID)
	api.request("GET", filePath, otherToken, nil, 404)
	link, _ := url.Parse(f["url"].(string))
	resp, e := api.client.Get(api.base + link.RequestURI())
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "hello from integration" {
		t.Fatalf("download status=%d body=%s", resp.StatusCode, body)
	}
	api.request("GET", filePath+"/content", "", nil, 403)
	api.request("PATCH", "/api/v1/user/me", token, map[string]any{"avatarId": fileID}, 200)
	api.request("DELETE", filePath, token, nil, 409)
	api.request("PATCH", "/api/v1/user/me", token, map[string]any{"avatarId": 0}, 200)
	api.request("DELETE", filePath, token, nil, 200)
	content := bytes.Repeat([]byte("a"), 65537)
	sum := sha256.Sum256(content)
	manifest := data(api.request("POST", "/api/v1/files/multipart", token, map[string]any{"name": "chunks.txt", "size": len(content), "chunkSize": 65536, "digest": hex.EncodeToString(sum[:])}, 200))["upload"].(map[string]any)
	uploadID := manifest["uploadId"].(string)
	uploadPath := "/api/v1/files/multipart/" + uploadID
	api.upload(uploadPath+"/parts/1", "PUT", token, "part", content[65536:], 200)
	api.upload(uploadPath+"/parts/0", "PUT", token, "part", content[:65536], 200)
	complete := api.request("POST", uploadPath+"/complete", token, nil, 200)
	repeat := api.request("POST", uploadPath+"/complete", token, nil, 200)
	if data(complete)["file"].(map[string]any)["id"] != data(repeat)["file"].(map[string]any)["id"] {
		t.Fatal("completion not idempotent")
	}
	ticket := data(api.request("POST", "/api/v1/ws/ticket", token, nil, 200))["ticket"].(string)
	wsURL := "ws" + strings.TrimPrefix(api.base, "http") + "/ws?ticket=" + ticket
	conn, _, e := websocket.DefaultDialer.Dial(wsURL, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	if e = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"echo","data":"hello"}`)); e != nil {
		t.Fatal(e)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, e = conn.ReadMessage(); e != nil {
		t.Fatal(e)
	}
	if reused, resp, e := websocket.DefaultDialer.Dial(wsURL, nil); e == nil {
		reused.Close()
		t.Fatal("reused ticket accepted")
	} else if resp != nil {
		resp.Body.Close()
	}
	api.request("PUT", "/api/v1/user/password", token, map[string]any{"oldPassword": password, "newPassword": "changedPassword-456"}, 200)
	api.request("GET", "/api/v1/user/me", token, nil, 401)
	api.request("POST", "/api/v1/auth/refresh", "", map[string]any{"refreshToken": pair["refreshToken"], "deviceId": "integration"}, 401)
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, _, e = conn.ReadMessage(); e == nil {
		t.Fatal("revoked websocket remained active")
	}
	if e = service.Example().ApplyTask(context.Background(), int64(data(me)["user"].(map[string]any)["id"].(float64)), itemID, "manual-once"); e != nil {
		t.Fatal(e)
	}
}
func TestSchedulerRoleDoesNotNeedAuthOrDatabase(t *testing.T) {
	c := integrationConfig(t)
	c.Database.URL = ""
	c.Auth.AccessSecretKey = ""
	c.Auth.RefreshHashKey = ""
	c.Storage.Drivers = nil
	a, e := bootstrap.New(context.Background(), c, "scheduler")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if a.DB != nil || a.HTTP != nil || a.Worker != nil || a.Hub != nil {
		t.Fatal("scheduler initialized unrelated subsystem")
	}
	if e = a.Start(); e != nil {
		t.Fatal(e)
	}
}
