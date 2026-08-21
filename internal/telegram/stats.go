package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type TrafficDelta struct {
	ID   int64
	Up   int64
	Down int64
}

type statsFile struct {
	Users map[string]struct {
		BytesIn  int64 `json:"bytes_in"`
		BytesOut int64 `json:"bytes_out"`
	} `json:"users"`
}

// StatsCounter reads mtg-multi GET /stats and returns per-user deltas
// to add to the same traffic_up/traffic_down pool as VPN usage.
type StatsCounter struct {
	URL    string
	mu     sync.Mutex
	seen   map[int64][2]int64
	client *http.Client
	err    string
}

func NewStatsCounter() *StatsCounter {
	return &StatsCounter{
		URL:    StatsURL,
		seen:   map[int64][2]int64{},
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

func (c *StatsCounter) LastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *StatsCounter) Poll() ([]TrafficDelta, error) {
	req, err := http.NewRequest(http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.setErr(err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("mtg stats %d: %s", resp.StatusCode, b)
		c.setErr(err.Error())
		return nil, err
	}
	var file statsFile
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		c.setErr(err.Error())
		return nil, err
	}
	c.setErr("")

	c.mu.Lock()
	defer c.mu.Unlock()
	live := map[int64]bool{}
	var out []TrafficDelta
	for name, st := range file.Users {
		id, ok := ParseSecretName(name)
		if !ok {
			continue
		}
		live[id] = true
		prev, had := c.seen[id]
		in, outb := st.BytesIn, st.BytesOut
		var dup, ddown int64
		if !had {
			c.seen[id] = [2]int64{in, outb}
			continue
		}
		if in < prev[0] || outb < prev[1] {
			dup, ddown = in, outb
		} else {
			dup, ddown = in-prev[0], outb-prev[1]
		}
		c.seen[id] = [2]int64{in, outb}
		if dup == 0 && ddown == 0 {
			continue
		}
		out = append(out, TrafficDelta{ID: id, Up: dup, Down: ddown})
	}
	for id := range c.seen {
		if !live[id] {
			delete(c.seen, id)
		}
	}
	return out, nil
}

func (c *StatsCounter) setErr(s string) {
	c.mu.Lock()
	c.err = s
	c.mu.Unlock()
}
