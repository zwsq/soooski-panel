package api

import "testing"

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{110, "110 B"},
		{1024, "1 KB"},
		{1536, "1.5 KB"},
		{10737418240, "10 GB"},
	}
	for _, c := range cases {
		if got := formatBytes(c.n); got != c.want {
			t.Errorf("formatBytes(%d)=%q want %q", c.n, got, c.want)
		}
	}
}

func TestTrafficPct(t *testing.T) {
	cases := []struct {
		used, limit int64
		want        int
	}{
		{0, 0, 0},
		{50, 0, 0},
		{0, 100, 0},
		{50, 100, 50},
		{200, 100, 100},
		{110, 10737418240, 1},
	}
	for _, c := range cases {
		if got := trafficPct(c.used, c.limit); got != c.want {
			t.Errorf("trafficPct(%d,%d)=%d want %d", c.used, c.limit, got, c.want)
		}
	}
}
