import type { ReactNode } from "react";

/** How a plugin is doing, judged from what the log says about it. */
export type PluginHealth = "ok" | "warn" | "bad" | "off";

type Level = "info" | "warn" | "error" | "";

const prefix = /^\[(\d{2}:\d{2}:\d{2})(?: ([A-Z]+))?\]:? ?/;
const tag = /^\[([^\]\n]{1,40})\] ?/;

function levelOf(word: string | undefined): Level {
  if (!word) return "";
  if (word === "ERROR" || word === "SEVERE" || word === "FATAL") return "error";
  if (word === "WARN" || word === "WARNING") return "warn";
  return "info";
}

const rank: Record<PluginHealth, number> = { ok: 1, off: 2, warn: 3, bad: 4 };

/**
 * Reads the whole log once and works out each plugin's state for the current run:
 * green once it enables, yellow if it has warned, red if it has thrown an error or failed to load.
 */
export function pluginHealth(lines: string[]): Map<string, PluginHealth> {
  // Only the latest run counts, so a plugin fixed since the last start is not still shown red.
  let start = 0;
  lines.forEach((line, index) => {
    if (line.startsWith("[consolry] Server started")) start = index;
  });

  const health = new Map<string, PluginHealth>();
  const set = (name: string, state: PluginHealth) => {
    const key = name.toLowerCase();
    const current = health.get(key);
    // "off" replaces "ok" when a plugin is disabled, but never hides a warning or an error.
    if (!current || rank[state] > rank[current]) health.set(key, state);
  };

  for (let i = start; i < lines.length; i++) {
    const head = prefix.exec(lines[i]);
    const body = head ? lines[i].slice(head[0].length) : lines[i];
    const level = levelOf(head?.[2]);
    const name = tag.exec(body)?.[1];

    const enabling = /Enabling (\S+) v/.exec(body);
    if (enabling) set(enabling[1], "ok");
    const disabling = /Disabling (\S+) v/.exec(body);
    if (disabling) set(disabling[1], "off");
    const failed = /Error occurred while (?:enabling|loading|disabling) (\S+)/.exec(body);
    if (failed) set(failed[1], "bad");
    const unloadable = /Could not load '?plugins[\\/]([A-Za-z0-9_]+)/.exec(body);
    if (unloadable) set(unloadable[1], "bad");

    if (name && level === "error") set(name, "bad");
    else if (name && level === "warn") set(name, "warn");
  }
  return health;
}

/** One log line as coloured pieces. `inherited` is the level of the line above, for stack traces. */
export function colourLine(line: string, health: Map<string, PluginHealth>, inherited: Level): { node: ReactNode; level: Level } {
  if (line.startsWith("[consolry]")) return { node: <span className="c-system">{line}</span>, level: "" };

  const head = prefix.exec(line);
  if (!head) {
    // No timestamp: a stack trace or a continuation of the line above.
    if (/^\s+at |^Caused by:|^\s*\.\.\. \d+ more|Exception|^java\.|^org\.|^com\./.test(line) || inherited === "error") {
      return { node: <span className="c-trace">{line}</span>, level: "error" };
    }
    if (line.startsWith("WARNING:") || inherited === "warn") return { node: <span className="c-warn">{line}</span>, level: "warn" };
    return { node: <span className="c-plain">{line}</span>, level: "" };
  }

  const level = levelOf(head[2]);
  let body = line.slice(head[0].length);
  const pieces: ReactNode[] = [
    <span key="time" className="c-time">
      {head[1]}
    </span>,
    " ",
    <span key="level" className={`c-level l-${level || "info"}`}>
      {head[2] ?? "INFO"}
    </span>,
    " ",
  ];

  const named = tag.exec(body);
  if (named) {
    const state = health.get(named[1].toLowerCase());
    pieces.push(
      <span key="tag" className={`c-tag ${state ?? "plain"}`} title={state ? tagTitle[state] : undefined}>
        [{named[1]}]
      </span>,
      " ",
    );
    body = body.slice(named[0].length);
  }

  pieces.push(
    <span key="body" className={bodyClass(body, level)}>
      {body}
    </span>,
  );
  return { node: pieces, level };
}

const tagTitle: Record<PluginHealth, string> = {
  ok: "This plugin enabled without problems",
  warn: "This plugin has logged a warning",
  bad: "This plugin has logged an error",
  off: "This plugin has been disabled",
};

function bodyClass(body: string, level: Level) {
  if (level === "error") return "c-error";
  if (level === "warn") return "c-warn";
  if (/^Done \([0-9.,]+s\)! For help/.test(body)) return "c-done";
  if (/ joined the game$/.test(body)) return "c-join";
  if (/ left the game$| lost connection: /.test(body)) return "c-leave";
  if (/^<[^>]+> |^\[Not Secure\] /.test(body)) return "c-chat";
  if (/issued server command: /.test(body)) return "c-command";
  if (/^Enabling |^Loading server plugin |^Loaded plugin|[Ee]nabled|successfully/.test(body)) return "c-good";
  if (/^Disabling |^Stopping |^Saving |^Closing /.test(body)) return "c-muted";
  return "c-plain";
}
