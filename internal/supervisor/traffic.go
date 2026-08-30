package supervisor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type connectionsFile struct {
	Connections []struct {
		ID       string   `json:"id"`
		Upload   int64    `json:"upload"`
		Download int64    `json:"download"`
		Chains   []string `json:"chains"`
		Metadata struct {
			User string `json:"user"`
			UID  string `json:"uid"`
			Type string `json:"type"`
		} `json:"metadata"`
	} `json:"connections"`
}

type TrafficDelta struct {
	User string
	ID   int64
	Up   int64
	Down int64
}

// Presence is a user with at least one live Clash connection this poll.
type Presence struct {
	User string
	ID   int64
}

type Counter struct {
	secret string
	mu     sync.Mutex
	seen   map[string][2]int64
	client *http.Client
	err    string
}

func NewCounter(secret string) *Counter {
	return &Counter{
		secret: secret,
		seen:   map[string][2]int64{},
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

func (c *Counter) LastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *Counter) Poll() ([]TrafficDelta, []Presence, error) {
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9090/connections", nil)
	if err != nil {
		return nil, nil, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.setErr(err.Error())
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("clash api %d: %s", resp.StatusCode, b)
		c.setErr(err.Error())
		return nil, nil, err
	}
	var file connectionsFile
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		c.setErr(err.Error())
		return nil, nil, err
	}
	c.setErr("")

	c.mu.Lock()
	defer c.mu.Unlock()
	liveConn := map[string]bool{}
	sum := map[string][3]int64{} // up, down, id
	names := map[string]string{}
	for _, conn := range file.Connections {
		user, uid := connUser(conn.Metadata.User, conn.Metadata.UID, conn.Chains)
		if user == "" && uid == 0 {
			continue
		}
		key := user
		if uid > 0 {
			key = "id:" + strconv.FormatInt(uid, 10)
		}
		if user != "" {
			names[key] = user
		}
		liveConn[conn.ID] = true
		prev := c.seen[conn.ID]
		dup := conn.Upload - prev[0]
		ddown := conn.Download - prev[1]
		if dup < 0 {
			dup = 0
		}
		if ddown < 0 {
			ddown = 0
		}
		c.seen[conn.ID] = [2]int64{conn.Upload, conn.Download}
		acc := sum[key]
		sum[key] = [3]int64{acc[0] + dup, acc[1] + ddown, uid}
	}
	for id := range c.seen {
		if !liveConn[id] {
			delete(c.seen, id)
		}
	}
	var out []TrafficDelta
	var live []Presence
	for key, v := range sum {
		p := Presence{User: names[key], ID: v[2]}
		if strings.HasPrefix(key, "id:") {
			p.ID, _ = strconv.ParseInt(strings.TrimPrefix(key, "id:"), 10, 64)
		} else if p.User == "" {
			p.User = key
		}
		live = append(live, p)
		if v[0] == 0 && v[1] == 0 {
			continue
		}
		out = append(out, TrafficDelta{User: p.User, ID: p.ID, Up: v[0], Down: v[1]})
	}
	return out, live, nil
}

func (c *Counter) setErr(s string) {
	c.mu.Lock()
	c.err = s
	c.mu.Unlock()
}

func connUser(metaUser, metaUID string, chains []string) (string, int64) {
	if metaUser != "" {
		return metaUser, 0
	}
	if metaUID != "" {
		return metaUID, 0
	}
	for _, ch := range chains {
		if strings.HasPrefix(ch, "user-") {
			rest := strings.TrimPrefix(ch, "user-")
			if id, err := strconv.ParseInt(rest, 10, 64); err == nil && id > 0 {
				return "", id
			}
			if rest != "" {
				return rest, 0
			}
		}
	}
	return "", 0
}
