package selfcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"task005-notify/internal/httpapi"
	"task005-notify/internal/notify"
)

type clock struct {
	mu sync.RWMutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.RLock(); defer c.mu.RUnlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); defer c.mu.Unlock(); c.t = c.t.Add(d) }

func Run() int {
	passed, failed := 0, 0
	check := func(name string, fn func() error) {
		if err := fn(); err != nil {
			failed++
			fmt.Printf("FAIL %-32s %v\n", name, err)
		} else {
			passed++
			fmt.Printf("PASS %s\n", name)
		}
	}

	clk := &clock{t: time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)}
	api := httpapi.NewWithClock(clk.now)
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	do := func(method, path, body string) (*http.Response, []byte, error) {
		var r io.Reader
		if body != "" {
			r = bytes.NewReader([]byte(body))
		}
		req, err := http.NewRequest(method, srv.URL+path, r)
		if err != nil {
			return nil, nil, err
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, nil, err
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, data, readErr
	}

	createBody := func(id, recipient, content, priority string, scheduleAt *time.Time) string {
		type req struct {
			ID         string     `json:"id"`
			Recipient  string     `json:"recipient"`
			Content    string     `json:"content"`
			Priority   string     `json:"priority,omitempty"`
			ScheduleAt *time.Time `json:"schedule_at,omitempty"`
		}
		b, _ := json.Marshal(req{ID: id, Recipient: recipient, Content: content, Priority: priority, ScheduleAt: scheduleAt})
		return string(b)
	}

	parseOne := func(data []byte) (notify.Notification, error) {
		var out struct {
			Notification notify.Notification `json:"notification"`
		}
		err := json.Unmarshal(data, &out)
		return out.Notification, err
	}

	parseList := func(data []byte) ([]notify.Notification, int, error) {
		var out struct {
			Notifications []notify.Notification `json:"notifications"`
			Total         int                   `json:"total"`
		}
		err := json.Unmarshal(data, &out)
		return out.Notifications, out.Total, err
	}

	check("健康检查", func() error {
		resp, _, err := do(http.MethodGet, "/healthz", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("创建通知返回待发送状态", func() error {
		resp, body, err := do(http.MethodPost, "/api/notifications", createBody("N1", "user-a", "你好", "", nil))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("status=%d body=%s", resp.StatusCode, body)
		}
		n, err := parseOne(body)
		if err != nil {
			return err
		}
		if n.ID != "N1" || n.Priority != notify.PriorityNormal || n.Status != notify.StatusPending {
			return fmt.Errorf("unexpected: %+v", n)
		}
		return nil
	})

	check("重复通知编号被拒绝", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications", createBody("N1", "user-a", "dup", "", nil))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("空接收人被拒绝", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications", createBody("NX1", "   ", "内容", "", nil))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusBadRequest {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("空内容被拒绝", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications", createBody("NX2", "user-a", "   ", "", nil))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusBadRequest {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("非法优先级被拒绝", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications", createBody("NX3", "user-a", "内容", "urgent", nil))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusBadRequest {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("非法计划发送时间被拒绝", func() error {
		past := clk.now().Add(-time.Hour)
		resp, _, err := do(http.MethodPost, "/api/notifications", createBody("NX4", "user-a", "内容", "", &past))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusBadRequest {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("多段 JSON 被拒绝", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications", createBody("NX5", "user-a", "内容", "", nil)+" {}")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusBadRequest {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	clk.add(time.Hour)

	check("标记已发送并记录时间", func() error {
		resp, body, err := do(http.MethodPost, "/api/notifications/N1/send", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status=%d body=%s", resp.StatusCode, body)
		}
		n, err := parseOne(body)
		if err != nil {
			return err
		}
		if n.Status != notify.StatusSent || n.SentAt == nil {
			return fmt.Errorf("unexpected: %+v", n)
		}
		return nil
	})

	check("已发送不能重复标记", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications/N1/send", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("标记已读并记录时间", func() error {
		resp, body, err := do(http.MethodPost, "/api/notifications/N1/read", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status=%d body=%s", resp.StatusCode, body)
		}
		n, err := parseOne(body)
		if err != nil {
			return err
		}
		if n.Status != notify.StatusRead || n.ReadAt == nil {
			return fmt.Errorf("unexpected: %+v", n)
		}
		return nil
	})

	check("已读不能重复标记", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications/N1/read", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("已读不能回退标记已发送", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications/N1/send", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	clk.add(time.Hour)

	check("创建第二条通知", func() error {
		resp, body, err := do(http.MethodPost, "/api/notifications", createBody("N2", "user-b", "提醒", "", nil))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("status=%d body=%s", resp.StatusCode, body)
		}
		n, err := parseOne(body)
		if err != nil {
			return err
		}
		if n.Status != notify.StatusPending {
			return fmt.Errorf("unexpected: %+v", n)
		}
		return nil
	})

	check("待发送不能直接标记已读", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications/N2/read", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("标记 N2 已发送", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications/N2/send", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	clk.add(time.Hour)

	check("创建高优先级通知", func() error {
		resp, body, err := do(http.MethodPost, "/api/notifications", createBody("N3", "user-c", "紧急", notify.PriorityHigh, nil))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("status=%d body=%s", resp.StatusCode, body)
		}
		n, err := parseOne(body)
		if err != nil {
			return err
		}
		if n.Priority != notify.PriorityHigh {
			return fmt.Errorf("unexpected: %+v", n)
		}
		return nil
	})

	check("列表优先高优先级并按创建时间倒序", func() error {
		resp, body, err := do(http.MethodGet, "/api/notifications", "")
		if err != nil {
			return err
		}
		items, total, err := parseList(body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK || total != 3 || len(items) != 3 {
			return fmt.Errorf("status=%d total=%d len=%d", resp.StatusCode, total, len(items))
		}
		// 期望顺序：N3(高优先级,T+3h)、N2(normal,T+2h)、N1(normal,T0)
		want := []string{"N3", "N2", "N1"}
		for i, w := range want {
			if items[i].ID != w {
				return fmt.Errorf("order=%+v", items)
			}
		}
		return nil
	})

	check("查询单个通知", func() error {
		resp, body, err := do(http.MethodGet, "/api/notifications/N1", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status=%d body=%s", resp.StatusCode, body)
		}
		n, err := parseOne(body)
		if err != nil {
			return err
		}
		if n.Status != notify.StatusRead {
			return fmt.Errorf("unexpected: %+v", n)
		}
		return nil
	})

	check("查询不存在通知返回 404", func() error {
		resp, _, err := do(http.MethodGet, "/api/notifications/NOPE", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("标记已发送不存在返回 404", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications/NOPE/send", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("标记已读不存在返回 404", func() error {
		resp, _, err := do(http.MethodPost, "/api/notifications/NOPE/read", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("删除通知返回 204", func() error {
		resp, _, err := do(http.MethodDelete, "/api/notifications/N3", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("删除已删除通知返回 404", func() error {
		resp, _, err := do(http.MethodDelete, "/api/notifications/N3", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("删除后列表减少", func() error {
		resp, body, err := do(http.MethodGet, "/api/notifications", "")
		if err != nil {
			return err
		}
		_, total, err := parseList(body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK || total != 2 {
			return fmt.Errorf("status=%d total=%d", resp.StatusCode, total)
		}
		return nil
	})

	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return 1
	}
	return 0
}
