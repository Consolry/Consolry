import { useEffect, useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { consoleSocket } from "./api";
import { colourLine, parseLine, pluginHealth } from "./colour";

const maxLines = 2000;

export default function Console({ serverId, running }: { serverId: string; running: boolean }) {
  const [lines, setLines] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);
  const [following, setFollowing] = useState(true);
  const [unseen, setUnseen] = useState(0);
  const [input, setInput] = useState("");
  const [filter, setFilter] = useState("");
  const socket = useRef<WebSocket | null>(null);
  const output = useRef<HTMLPreElement>(null);
  const followingNow = useRef(true);
  const lastTop = useRef(0);
  const history = useRef<string[]>([]);
  const historyAt = useRef(-1);

  function follow(on: boolean) {
    followingNow.current = on;
    setFollowing(on);
    if (on) setUnseen(0);
  }

  useEffect(() => {
    let closed = false;
    let retry: number | undefined;

    function connect() {
      const ws = consoleSocket(serverId);
      socket.current = ws;
      ws.onopen = () => {
        // The daemon replays its history on connect, so start from empty each time.
        setLines([]);
        setConnected(true);
        follow(true);
      };
      ws.onmessage = (event) => {
        const line = String(event.data);
        // A fresh start is worth seeing even if the reader had scrolled up through the last run.
        if (line.startsWith("[consolry] Server started")) follow(true);
        else if (!followingNow.current) setUnseen((count) => count + 1);
        setLines((current) => {
          const next = [...current, line];
          return next.length > maxLines ? next.slice(-maxLines) : next;
        });
      };
      ws.onclose = () => {
        setConnected(false);
        if (!closed) retry = window.setTimeout(connect, 2000);
      };
    }

    setLines([]);
    connect();
    return () => {
      closed = true;
      window.clearTimeout(retry);
      socket.current?.close();
    };
  }, [serverId]);

  const pin = () => {
    const element = output.current;
    if (element && followingNow.current) {
      element.scrollTop = element.scrollHeight;
      lastTop.current = element.scrollTop;
    }
  };

  // Stay at the newest line as output arrives. This runs before the browser paints,
  // so there is no moment where the view sits above the bottom.
  useLayoutEffect(pin, [lines, following, filter]);

  // The console is resized by things around it (a banner appearing, the window changing).
  // That must keep it at the bottom, not count as the reader scrolling away.
  useEffect(() => {
    const element = output.current;
    if (!element || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(pin);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  // Only the reader moving up stops the console following. Reaching the bottom resumes it.
  function onScroll() {
    const element = output.current;
    if (!element) return;
    const atBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 24;
    if (atBottom) {
      if (!followingNow.current) follow(true);
    } else if (element.scrollTop < lastTop.current - 4 && followingNow.current) {
      follow(false);
    }
    lastTop.current = element.scrollTop;
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Enter" && input.trim() && socket.current?.readyState === WebSocket.OPEN) {
      socket.current.send(input);
      history.current = [input, ...history.current.filter((item) => item !== input)].slice(0, 50);
      historyAt.current = -1;
      setInput("");
      follow(true);
    } else if (event.key === "ArrowUp" || event.key === "ArrowDown") {
      const step = event.key === "ArrowUp" ? 1 : -1;
      const next = Math.max(-1, Math.min(history.current.length - 1, historyAt.current + step));
      historyAt.current = next;
      setInput(next === -1 ? "" : history.current[next]);
      event.preventDefault();
    }
  }

  const needle = filter.trim().toLowerCase();

  // Colour every line. A line with no timestamp (a stack trace) takes the level of the line above it,
  // so the levels are worked out over the whole log before any search filter is applied.
  const rendered = useMemo(() => {
    const parsed = lines.map(parseLine);
    const health = pluginHealth(parsed.map((line) => line.plain));
    let level: ReturnType<typeof colourLine>["level"] = "";
    let list = false;
    const out: { text: string; node: ReactNode }[] = [];
    for (const line of parsed) {
      const result = colourLine(line, health, level, list);
      level = result.level;
      list = result.list;
      out.push({ text: line.plain, node: result.node });
    }
    return out;
  }, [lines]);
  const shown = needle ? rendered.filter((line) => line.text.toLowerCase().includes(needle)) : rendered;

  return (
    <section className="console" aria-label="Console">
      <header>
        <span className="pixel">Console</span>
        <span className={connected ? "live on" : "live"}>{connected ? "Live" : "Reconnecting"}</span>
        <span className="count">{lines.length} lines</span>
        <input
          className="filter"
          type="search"
          value={filter}
          onChange={(event) => setFilter(event.target.value)}
          placeholder="Search the log"
          aria-label="Search the log"
        />
      </header>
      <div className="console-body">
        <pre ref={output} onScroll={onScroll} aria-label="Server output" tabIndex={0}>
          {lines.length === 0 ? (
            <span className="dim">{connected ? "No output yet. Start the server to see its log here." : "Connecting to the console…"}</span>
          ) : shown.length === 0 ? (
            <span className="dim">No lines contain "{filter}".</span>
          ) : (
            shown.map((line, index) => (
              <span key={index}>
                {line.node}
                {"\n"}
              </span>
            ))
          )}
        </pre>
        {!following && (
          <button className="jump" onClick={() => follow(true)}>
            {unseen > 0 ? `${unseen} new line${unseen === 1 ? "" : "s"} below. Jump to latest` : "Jump to latest"}
          </button>
        )}
      </div>
      <div className="console-input">
        <span aria-hidden="true">&gt;</span>
        <input
          value={input}
          onChange={(event) => setInput(event.target.value)}
          onKeyDown={onKeyDown}
          disabled={!running || !connected}
          placeholder={running ? "Type a command and press Enter" : "Start the server to send commands"}
          aria-label="Console command"
          spellCheck={false}
          autoComplete="off"
        />
      </div>
    </section>
  );
}
