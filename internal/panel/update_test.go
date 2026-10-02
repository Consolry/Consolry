package panel

import "testing"

func TestNewerVersion(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.3.0", "0.2.0", true},
		{"v0.2.0", "0.2.0", false},
		{"v0.2.0", "0.10.0", false},
		{"v0.10.0", "0.9.9", true},
		{"v1.0", "0.9.9", true},
		{"v0.2.1", "0.2.0-preview", true},
		{"v0.3.0", "dev", false},
		{"", "0.2.0", false},
	}
	for _, c := range cases {
		if got := newerVersion(c.latest, c.current); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}
