package panel

import (
	"reflect"
	"testing"
	"time"
)

func TestScheduleNext(t *testing.T) {
	zone := time.FixedZone("test", 2*3600)
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, zone) }
	// 1 October 2026 is a Thursday.

	cases := []struct {
		name     string
		schedule Schedule
		now      time.Time
		want     time.Time
	}{
		{"interval counts from creation", Schedule{Mode: "interval", Minutes: 90, CreatedAt: at(1, 10, 0).Unix()}, at(1, 10, 5), at(1, 11, 30)},
		{"interval counts from the last run", Schedule{Mode: "interval", Minutes: 60, CreatedAt: at(1, 10, 0).Unix(), LastRun: at(1, 12, 0).Unix()}, at(1, 12, 1), at(1, 13, 0)},
		{"daily later today", Schedule{Mode: "daily", At: "16:00", CreatedAt: at(1, 10, 0).Unix()}, at(1, 10, 5), at(1, 16, 0)},
		{"daily already passed today", Schedule{Mode: "daily", At: "04:00", CreatedAt: at(1, 10, 0).Unix()}, at(1, 10, 5), at(2, 4, 0)},
		{"daily is due at its minute", Schedule{Mode: "daily", At: "04:00", CreatedAt: at(1, 10, 0).Unix()}, at(2, 4, 0), at(2, 4, 0)},
		{"daily after it ran moves to tomorrow", Schedule{Mode: "daily", At: "04:00", CreatedAt: at(1, 10, 0).Unix(), LastRun: at(2, 4, 0).Unix()}, at(2, 4, 1), at(3, 4, 0)},
		{"a run missed while the panel was off is skipped", Schedule{Mode: "daily", At: "04:00", CreatedAt: at(1, 10, 0).Unix()}, at(2, 15, 0), at(3, 4, 0)},
		{"weekly picks the right day", Schedule{Mode: "weekly", At: "09:30", Weekday: int(time.Monday), CreatedAt: at(1, 10, 0).Unix()}, at(1, 10, 5), at(5, 9, 30)},
		{"weekly on today's weekday, later", Schedule{Mode: "weekly", At: "18:00", Weekday: int(time.Thursday), CreatedAt: at(1, 10, 0).Unix()}, at(1, 10, 5), at(1, 18, 0)},
	}
	for _, c := range cases {
		if got := c.schedule.next(c.now); !got.Equal(c.want) {
			t.Errorf("%s: next = %s, want %s", c.name, got.Format("Mon 2 15:04"), c.want.Format("Mon 2 15:04"))
		}
	}
}

func TestScheduleValidate(t *testing.T) {
	good := []Schedule{
		{Action: "restart", Mode: "daily", At: "04:00"},
		{Action: "backup", Mode: "interval", Minutes: 360},
		{Action: "command", Command: "say Restarting soon", Mode: "weekly", At: "23:59", Weekday: 6},
	}
	for _, s := range good {
		if err := s.validate(); err != nil {
			t.Errorf("%+v should be valid: %v", s, err)
		}
	}
	bad := []Schedule{
		{Action: "explode", Mode: "daily", At: "04:00"},
		{Action: "restart", Mode: "daily", At: "25:00"},
		{Action: "restart", Mode: "interval", Minutes: 1},
		{Action: "command", Command: "say hi\nop intruder", Mode: "daily", At: "04:00"},
		{Action: "command", Command: "  ", Mode: "daily", At: "04:00"},
		{Action: "restart", Mode: "weekly", At: "04:00", Weekday: 9},
	}
	for _, s := range bad {
		if err := s.validate(); err == nil {
			t.Errorf("%+v should be rejected", s)
		}
	}
}

func TestMemoryArgs(t *testing.T) {
	got := memoryArgs([]string{"-Xms1024M", "-Xmx1024M", "-XX:+UseG1GC", "-jar", "server.jar", "nogui"}, 4096)
	want := []string{"-Xms4096M", "-Xmx4096M", "-XX:+UseG1GC", "-jar", "server.jar", "nogui"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPlayerNames(t *testing.T) {
	for _, name := range []string{"Notch", "brick_wren", ".BedrockUser", "a"} {
		if !playerName.MatchString(name) {
			t.Errorf("%q should be accepted", name)
		}
	}
	for _, name := range []string{"", "two words", "evil\nop me", "semi;colon", "x/y"} {
		if playerName.MatchString(name) {
			t.Errorf("%q should be rejected", name)
		}
	}
	for _, line := range []string{
		"[21:20:04 INFO]: There are 2 of a max of 20 players online: Mossy_Kat, brickwren",
		"[04:40:16 INFO]: There are 2 out of maximum 20 players online.",
	} {
		if match := listReply.FindStringSubmatch(line); match == nil || match[1] != "2" || match[2] != "20" {
			t.Errorf("the list reply %q was not understood: %v", line, match)
		}
	}

	loading := []string{"[consolry] Server started", "[04:39:35 INFO]: Preparing level \"world\""}
	if ready(loading) {
		t.Error("a server still loading its world is not ready for commands")
	}
	if !ready(append(loading, "[04:40:03 INFO]: Done (66.030s)! For help, type \"help\"")) {
		t.Error("a server that printed Done is ready")
	}
	if ready([]string{"[04:40:03 INFO]: Done (66.030s)! For help", "[consolry] Server stopped (exit code 0)", "[consolry] Server started"}) {
		t.Error("Done from an earlier run must not count for the current one")
	}
}
