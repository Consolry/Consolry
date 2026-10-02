package minecraft

import (
	"regexp"
	"strings"
)

// Finding is one thing the log says went wrong, in plain language.
type Finding struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
	Line   string `json:"line"`
}

type rule struct {
	pattern *regexp.Regexp
	explain func(match []string, lines []string, at int) Finding
}

// classVersions maps Java class-file versions to the Java release that produces them.
var classVersions = map[string]string{"52": "8", "55": "11", "60": "16", "61": "17", "65": "21", "69": "25"}

var pluginJar = regexp.MustCompile(`(?i)could not load '?(?:plugins[\\/])?([^'\\/]+?\.jar)'?`)

// jarBefore finds the plugin file named in the lines just above a stack trace.
func jarBefore(lines []string, at int) string {
	for i := at; i >= 0 && i >= at-3; i-- {
		if match := pluginJar.FindStringSubmatch(lines[i]); match != nil {
			return match[1]
		}
	}
	return ""
}

var rules = []rule{
	{regexp.MustCompile(`(?i)you need to agree to the eula`), func(_ []string, _ []string, _ int) Finding {
		return Finding{
			Title:  "The Minecraft EULA hasn't been accepted",
			Detail: "The server refuses to start until eula.txt says eula=true.",
			Fix:    "Open eula.txt in Files, change eula=false to eula=true, save, and start again.",
		}
	}},
	{regexp.MustCompile(`(?i)(failed to bind to port|address already in use|perhaps a server is already running on that port)`), func(_ []string, _ []string, _ int) Finding {
		return Finding{
			Title:  "The port is already in use",
			Detail: "Another program, often another Minecraft server, is already listening on this server's port.",
			Fix:    "Stop the other server, or change server-port in server.properties to a free port such as 25566.",
		}
	}},
	{regexp.MustCompile(`java\.lang\.OutOfMemoryError`), func(_ []string, _ []string, _ int) Finding {
		return Finding{
			Title:  "The server ran out of memory",
			Detail: "Java used all the memory it was allowed and stopped.",
			Fix:    "Raise the -Xmx value in the start command, or remove plugins and reduce view distance.",
		}
	}},
	{regexp.MustCompile(`UnsupportedClassVersionError.*class file version (\d+)`), func(match []string, _ []string, _ int) Finding {
		needed := classVersions[match[1]]
		if needed == "" {
			needed = "a newer version"
		}
		return Finding{
			Title:  "Java is too old for this server",
			Detail: "This server software was built for Java " + needed + ", and the Java on this machine is older.",
			Fix:    "Install Java " + needed + " or newer on the node, then start the server again.",
		}
	}},
	{regexp.MustCompile(`(?i)unknown/missing dependency plugins: \[([^\]]+)\]`), func(match []string, lines []string, at int) Finding {
		jar := jarBefore(lines, at)
		who := "A plugin"
		if jar != "" {
			who = jar
		}
		return Finding{
			Title:  who + " is missing a plugin it depends on",
			Detail: who + " did not load because it needs " + match[1] + ", which isn't installed.",
			Fix:    "Install " + match[1] + " from the Plugins tab, or remove " + who + ".",
		}
	}},
	{regexp.MustCompile(`(?i)error occurred while enabling (\S+) v?(\S+)`), func(match []string, _ []string, _ int) Finding {
		return Finding{
			Title:  match[1] + " failed while starting",
			Detail: "The plugin " + match[1] + " (version " + strings.TrimSuffix(match[2], ")") + ") threw an error as it was enabled.",
			Fix:    "Update " + match[1] + " to a version made for this Minecraft version, or remove it. Its own config may also be invalid.",
		}
	}},
	{regexp.MustCompile(`(?i)(unable to access jarfile|could not find or load main class)\s*(\S*)`), func(match []string, _ []string, _ int) Finding {
		return Finding{
			Title:  "The server file is missing",
			Detail: "Java could not find the file named in the start command" + ifAny(" (", match[2], ")") + ".",
			Fix:    "Check Files for the server jar. Its name must match the start command exactly.",
		}
	}},
	{regexp.MustCompile(`(?i)\[consolry\] could not start: .*executable file not found`), func(_ []string, _ []string, _ int) Finding {
		return Finding{
			Title:  "The program in the start command isn't installed",
			Detail: "The node could not find the program to run. For a Minecraft server this is usually Java.",
			Fix:    "Install Java on the node, or fix the start command.",
		}
	}},
	{regexp.MustCompile(`(?i)session\.lock.*(already locked|in use)|already locked \(possibly by other minecraft instance`), func(_ []string, _ []string, _ int) Finding {
		return Finding{
			Title:  "The world is open in another server",
			Detail: "Another copy of this server is still running and has the world locked.",
			Fix:    "Use Kill to end the leftover process, then start again.",
		}
	}},
	{regexp.MustCompile(`(?i)failed to load properties from file|invalid or corrupt jarfile`), func(_ []string, _ []string, _ int) Finding {
		return Finding{
			Title:  "A server file is damaged",
			Detail: "A file the server needs could not be read. A download or upload may have been cut short.",
			Fix:    "Replace the file named in the line below, or restore a backup.",
		}
	}},
}

func ifAny(before, value, after string) string {
	if value == "" {
		return ""
	}
	return before + value + after
}

// Explain reads a server's recent console output and reports what went wrong in its latest run.
// It only matches known patterns, so an empty result means "nothing recognised", not "nothing wrong".
func Explain(lines []string) []Finding {
	// Only the latest run matters: start after the last "Server started" marker.
	start := 0
	for i, line := range lines {
		if strings.HasPrefix(line, "[consolry] Server started") {
			start = i
		}
	}
	lines = lines[start:]

	findings := []Finding{}
	seen := map[string]bool{}
	for i, line := range lines {
		for _, rule := range rules {
			match := rule.pattern.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			finding := rule.explain(match, lines, i)
			if seen[finding.Title] {
				continue
			}
			seen[finding.Title] = true
			finding.Line = strings.TrimSpace(line)
			findings = append(findings, finding)
		}
	}
	return findings
}
