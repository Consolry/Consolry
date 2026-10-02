import type { CSSProperties, ReactNode } from "react";

/** How a plugin is doing, judged from what the log says about it. */
export type PluginHealth = "ok" | "warn" | "bad" | "off";

type Level = "info" | "warn" | "error" | "";

/** A stretch of a line that the server itself coloured. */
type Run = { start: number; end: number; style: CSSProperties };

/** One log line with the server's colour codes taken out and remembered. */
export type Parsed = { plain: string; runs: Run[] };

// The 16 standard terminal colours, tuned to read well on the console's dark background.
const palette = [
  "#5f7691", "#ff7b7b", "#7ddc8c", "#f2c14e", "#6aa8ff", "#d49bff", "#6fe0e6", "#d5dfec",
  "#8496ab", "#ff9a9a", "#a9f0b3", "#ffe08a", "#9cc6ff", "#e5c2ff", "#a8f1f5", "#ffffff",
];

function colour256(n: number): string {
  if (n < 16) return palette[n];
  if (n >= 232) {
    const grey = 8 + (n - 232) * 10;
    return `rgb(${grey},${grey},${grey})`;
  }
  const value = n - 16;
  const step = (index: number) => (index === 0 ? 0 : 55 + index * 40);
  return `rgb(${step(Math.floor(value / 36))},${step(Math.floor(value / 6) % 6)},${step(value % 6)})`;
}

const escape = /\x1b\[([0-9;]*)m/g;

/** Splits a raw console line into its text and the colours the server gave parts of it. */
export function parseLine(raw: string): Parsed {
  if (!raw.includes("\x1b")) return { plain: raw, runs: [] };

  const runs: Run[] = [];
  let plain = "";
  let style: CSSProperties = {};
  let last = 0;
  const flush = (text: string) => {
    if (!text) return;
    if (Object.keys(style).length) runs.push({ start: plain.length, end: plain.length + text.length, style });
    plain += text;
  };

  for (const match of raw.matchAll(escape)) {
    flush(raw.slice(last, match.index));
    last = match.index + match[0].length;
    const codes = match[1] === "" ? [0] : match[1].split(";").map(Number);
    style = { ...style };
    for (let i = 0; i < codes.length; i++) {
      const code = codes[i];
      if (code === 0) style = {};
      else if (code === 1) style.fontWeight = 600;
      else if (code === 3) style.fontStyle = "italic";
      else if (code === 4) style.textDecoration = "underline";
      else if (code === 22) delete style.fontWeight;
      else if (code === 39) delete style.color;
      else if (code >= 30 && code <= 37) style.color = palette[code - 30];
      else if (code >= 90 && code <= 97) style.color = palette[code - 90 + 8];
      else if (code === 38 && codes[i + 1] === 5) {
        style.color = colour256(codes[i + 2] ?? 7);
        i += 2;
      } else if (code === 38 && codes[i + 1] === 2) {
        style.color = `rgb(${codes[i + 2] ?? 0},${codes[i + 3] ?? 0},${codes[i + 4] ?? 0})`;
        i += 4;
      } else if (code === 48 && codes[i + 1] === 5) i += 2;
      else if (code === 48 && codes[i + 1] === 2) i += 4;
    }
  }
  flush(raw.slice(last));
  return { plain, runs };
}

/** True if the server coloured any part of the line from `from` onwards. */
function coloured(line: Parsed, from: number) {
  return line.runs.some((run) => run.end > from && run.style.color);
}

/** The part of a line from `from` onwards, drawn in the server's own colours. */
function serverColours(line: Parsed, from: number): ReactNode[] {
  const out: ReactNode[] = [];
  let at = from;
  for (const run of line.runs) {
    if (run.end <= at) continue;
    const start = Math.max(run.start, at);
    if (start > at) out.push(line.plain.slice(at, start));
    out.push(
      <span key={start} style={run.style}>
        {line.plain.slice(start, run.end)}
      </span>,
    );
    at = run.end;
  }
  if (at < line.plain.length) out.push(line.plain.slice(at));
  return out;
}

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

const tagTitle: Record<PluginHealth, string> = {
  ok: "This plugin enabled without problems",
  warn: "This plugin has logged a warning",
  bad: "This plugin has logged an error",
  off: "This plugin has been disabled",
};

const listHeader = /Plugins \(\d+\):\s*$/;

/**
 * A line from a plugin list, such as " - AxAuctions, AxKoth (2.21.0), Essentials", with each
 * name coloured by that plugin's state. Names the log has said nothing about stay plain.
 */
function pluginList(text: string, health: Map<string, PluginHealth>): ReactNode[] {
  return text.split(/(, )/).map((part, index) => {
    if (part === ", ") return part;
    const lead = /^\s*(?:- )?/.exec(part)![0];
    const rest = part.slice(lead.length);
    const name = rest.split(" (")[0].trim();
    const state = health.get(name.toLowerCase());
    return (
      <span key={index}>
        {lead}
        <span className={`c-tag ${state ?? "unknown"}`} title={state ? tagTitle[state] : undefined}>
          {name}
        </span>
        {rest.slice(name.length)}
      </span>
    );
  });
}

type Coloured = { node: ReactNode; level: Level; list: boolean };

/**
 * One log line as coloured pieces. Where the server coloured the text itself, its colours are kept;
 * otherwise Consolry colours it by meaning. `inherited` is the level of the line above, for stack
 * traces; `inList` says the line above was part of a plugin list, which can run over several lines.
 */
export function colourLine(line: Parsed, health: Map<string, PluginHealth>, inherited: Level, inList = false): Coloured {
  const text = line.plain;
  if (text.startsWith("[consolry]")) return { node: <span className="c-system">{text}</span>, level: "", list: false };

  const head = prefix.exec(text);
  if (!head) {
    if (coloured(line, 0)) return { node: serverColours(line, 0), level: inherited, list: false };
    // The list Paper prints while starting has no timestamps: " - Name (1.2.3), Other (4.5)".
    if (/^\s+- \S/.test(text) && /\(\S+\)/.test(text)) return { node: pluginList(text, health), level: "", list: true };
    // No timestamp: a stack trace or a continuation of the line above.
    if (/^\s+at |^Caused by:|^\s*\.\.\. \d+ more|Exception|^java\.|^org\.|^com\./.test(text) || inherited === "error") {
      return { node: <span className="c-trace">{text}</span>, level: "error", list: false };
    }
    if (text.startsWith("WARNING:") || inherited === "warn") return { node: <span className="c-warn">{text}</span>, level: "warn", list: false };
    return { node: <span className="c-plain">{text}</span>, level: "", list: false };
  }

  const level = levelOf(head[2]);
  let at = head[0].length;
  let body = text.slice(at);
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

  // The answer to /plugins: a heading, then names on lines that start with a space.
  // Minecraft colours those names itself (green for enabled, red for disabled) when it can.
  if (level !== "error" && level !== "warn") {
    const heading = listHeader.test(body);
    if (heading || (inList && /^\s+\S/.test(body))) {
      if (coloured(line, at)) pieces.push(<span key="body">{serverColours(line, at)}</span>);
      else if (heading) {
        pieces.push(
          <span key="body" className="c-heading">
            {body}
          </span>,
        );
      } else pieces.push(<span key="body">{pluginList(body, health)}</span>);
      return { node: pieces, level, list: true };
    }
  }

  const named = tag.exec(body);
  if (named) {
    const state = health.get(named[1].toLowerCase());
    pieces.push(
      <span key="tag" className={`c-tag ${state ?? "plain"}`} title={state ? tagTitle[state] : undefined}>
        [{named[1]}]
      </span>,
      " ",
    );
    at += named[0].length;
    body = body.slice(named[0].length);
  }

  pieces.push(
    coloured(line, at) ? (
      <span key="body" className={level === "error" ? "c-error" : level === "warn" ? "c-warn" : undefined}>
        {serverColours(line, at)}
      </span>
    ) : (
      <span key="body" className={bodyClass(body, level)}>
        {body}
      </span>
    ),
  );
  return { node: pieces, level, list: false };
}

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
