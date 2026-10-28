//go:build integration

package bootstrap_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/reedtang666/go-easy-starter/internal/bootstrap"
)

func TestExampleListFiltersAndOrdering(t *testing.T) {
	c := integrationConfig(t)
	c.WebSocket.Enabled = false
	app, err := bootstrap.New(context.Background(), c, "http")
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	api := testAPI{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", app.HTTP.GetListenedPort()), client: &http.Client{Timeout: 10 * time.Second}}
	name := fmt.Sprintf("list%d", time.Now().UnixNano())
	login := func(username string) string {
		api.request("POST", "/api/v1/auth/register", "", map[string]any{"username": username, "password": "listPassword-123"}, 200)
		return data(api.request("POST", "/api/v1/auth/login", "", map[string]any{"username": username, "password": "listPassword-123", "deviceId": "list-test"}, 200))["accessToken"].(string)
	}
	token := login(name)
	other := login(name + "b")
	prefix := fmt.Sprintf("q-%d-", time.Now().UnixNano())
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var ids []int64
	for i, title := range []string{prefix + "A%_!", prefix + "Alpha", prefix + "Alpha"} {
		item := data(api.request("POST", "/api/v1/example/items", token, map[string]any{"title": title}, 200))["item"].(map[string]any)
		id := int64(item["id"].(float64))
		ids = append(ids, id)
		if _, err := app.DB.Exec(context.Background(), "UPDATE example_items SET created_at=? WHERE id=?", start.Add(time.Duration(i)*time.Second), id); err != nil {
			t.Fatal(err)
		}
	}
	api.request("POST", "/api/v1/example/items", other, map[string]any{"title": prefix + "Alpha"}, 200)
	list := func(params url.Values, status int) map[string]any {
		result := api.request("GET", "/api/v1/example/items?"+params.Encode(), token, nil, status)
		if status == 200 {
			return data(result)
		}
		return result
	}
	assertOne := func(result map[string]any, id int64) {
		t.Helper()
		rows := result["list"].([]any)
		if result["total"].(float64) != 1 || len(rows) != 1 || int64(rows[0].(map[string]any)["id"].(float64)) != id {
			t.Fatalf("unexpected filtered list: %#v", result)
		}
	}
	assertOne(list(url.Values{"keyword": {"%_!"}}, 200), ids[0])
	assertOne(list(url.Values{"keyword": {prefix}, "createdFrom": {start.Format(time.RFC3339)}, "createdTo": {start.Format(time.RFC3339)}}, 200), ids[0])
	result := list(url.Values{"keyword": {prefix + "Alpha"}, "sortBy": {"title"}, "sortOrder": {"asc"}, "pageSize": {"1"}, "page": {"2"}}, 200)
	rows := result["list"].([]any)
	if result["total"].(float64) != 2 || len(rows) != 1 || int64(rows[0].(map[string]any)["id"].(float64)) != ids[2] {
		t.Fatalf("count, stable paging or owner filter failed: %#v", result)
	}
	result = list(url.Values{"keyword": {prefix}, "createdFrom": {start.Format(time.RFC3339)}, "createdTo": {start.Add(time.Second).Format(time.RFC3339)}, "sortBy": {"created_at"}, "sortOrder": {"desc"}}, 200)
	rows = result["list"].([]any)
	if result["total"].(float64) != 2 || len(rows) != 2 || int64(rows[0].(map[string]any)["id"].(float64)) != ids[1] {
		t.Fatalf("time range/order failed: %#v", result)
	}
	result = list(url.Values{"keyword": {"' OR 1=1 --"}}, 200)
	if result["total"].(float64) != 0 || len(result["list"].([]any)) != 0 {
		t.Fatal("query value was not treated literally")
	}
	list(url.Values{"sortBy": {"title desc; DROP TABLE users"}}, 400)
	list(url.Values{"createdFrom": {"invalid"}}, 400)
	list(url.Values{"createdFrom": {start.Add(time.Second).Format(time.RFC3339)}, "createdTo": {start.Format(time.RFC3339)}}, 400)
}
