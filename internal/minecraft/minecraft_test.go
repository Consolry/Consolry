package minecraft

import (
	"reflect"
	"strings"
	"testing"
)

func TestReleasesNewestFirst(t *testing.T) {
	got := releasesNewestFirst([]string{"1.21.4", "26.3-rc-3", "1.9", "26.3", "1.21.11", "1.21.11-pre5", "1.21"})
	want := []string{"26.3", "1.21.11", "1.21.4", "1.21", "1.9"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExplain(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		title string
		in    string // text the detail or fix must contain
	}{
		{"eula", []string{"[12:00:00 INFO]: You need to agree to the EULA in order to run the server. Go to eula.txt for more info."}, "The Minecraft EULA hasn't been accepted", "eula=true"},
		{"port", []string{"[12:00:00 WARN]: **** FAILED TO BIND TO PORT!"}, "The port is already in use", "server-port"},
		{"memory", []string{"java.lang.OutOfMemoryError: Java heap space"}, "The server ran out of memory", "-Xmx"},
		{"java", []string{"java.lang.UnsupportedClassVersionError: org/bukkit/Main has been compiled by a more recent version of the Java Runtime (class file version 65.0), this version only recognizes up to 61.0"}, "Java is too old for this server", "Java 21"},
		{
			"dependency",
			[]string{
				"[14:02:11 ERROR]: Could not load 'plugins/EssentialsXChat-2.21.0.jar' in folder 'plugins'",
				"org.bukkit.plugin.UnknownDependencyException: Unknown/missing dependency plugins: [Essentials]. Please download and install these plugins to run 'EssentialsChat'.",
			},
			"EssentialsXChat-2.21.0.jar is missing a plugin it depends on", "Essentials",
		},
		{"enable", []string{"[12:00:00 ERROR]: Error occurred while enabling WorldGuard v7.0.9 (Is it up to date?)"}, "WorldGuard failed while starting", "7.0.9"},
		{"jar", []string{"Error: Unable to access jarfile server.jar"}, "The server file is missing", "server.jar"},
		{"no java", []string{`[consolry] Could not start: exec: "java": executable file not found in %PATH%`}, "The program in the start command isn't installed", "Java"},
	}
	for _, c := range cases {
		findings := Explain(c.lines)
		if len(findings) != 1 {
			t.Errorf("%s: got %d findings, want 1", c.name, len(findings))
			continue
		}
		f := findings[0]
		if f.Title != c.title {
			t.Errorf("%s: title %q, want %q", c.name, f.Title, c.title)
		}
		if !strings.Contains(f.Detail+" "+f.Fix, c.in) {
			t.Errorf("%s: explanation should mention %q, got %q / %q", c.name, c.in, f.Detail, f.Fix)
		}
		if f.Line == "" {
			t.Errorf("%s: the matching log line should be kept", c.name)
		}
	}
}

func TestExplainOnlyReadsTheLatestRun(t *testing.T) {
	lines := []string{
		"[consolry] Server started",
		"java.lang.OutOfMemoryError: Java heap space",
		"[consolry] Server stopped (exit code 1)",
		"[consolry] Server started",
		"[12:00:00 INFO]: Done (4.2s)! For help, type \"help\"",
	}
	if findings := Explain(lines); len(findings) != 0 {
		t.Errorf("an old crash should not be reported for a healthy run, got %+v", findings)
	}
	if findings := Explain([]string{"[12:00:00 INFO]: Done"}); findings == nil || len(findings) != 0 {
		t.Errorf("a clean log should give an empty list, got %#v", findings)
	}
}
