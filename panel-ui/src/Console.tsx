import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { consoleSocket } from "./api";

const maxLines = 2000;

export default function Console({ serverId, running }: { serverId: string; running: boolean }) {
  const [lines, setLines] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);
  const [following, setFollowing] = useState(true);
  const [input, setInput] = useState("");
  const socket = useRef<WebSocket | null>(null);
  const output = useRef<HTMLPreElement>(null);
  const history = useRef<string[]>([]);
  const historyAt = useRef(-1);

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
      };
      ws.onmessage = (event) => {
        setLines((current) => {
          const next = [...current, String(event.data)];
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

  // Follow new output unless the reader has scrolled up to look at something.
  useEffect(() => {
    const element = output.current;
    if (element && following) element.scrollTop = element.scrollHeight;
  }, [lines, following]);

  function onScroll() {
    const element = output.current;
    if (element) setFollowing(element.scrollHeight - element.scrollTop - element.clientHeight < 24);
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Enter" && input.trim() && socket.current?.readyState === WebSocket.OPEN) {
      socket.current.send(input);
      history.current = [input, ...history.current.filter((item) => item !== input)].slice(0, 50);
      historyAt.current = -1;
      setInput("");
      setFollowing(true);
    } else if (event.key === "ArrowUp" || event.key === "ArrowDown") {
      const step = event.key === "ArrowUp" ? 1 : -1;
      const next = Math.max(-1, Math.min(history.current.length - 1, historyAt.current + step));
      historyAt.current = next;
      setInput(next === -1 ? "" : history.current[next]);
      event.preventDefault();
    }
  }

  return (
    <section className="console" aria-label="Console">
      <header>
        <span className="pixel">Console</span>
        <span className={connected ? "live on" : "live"}>{connected ? "Live" : "Reconnecting"}</span>
        <span className="count">{lines.length} lines</span>
        {!following && (
          <button className="small" onClick={() => setFollowing(true)}>
            Jump to latest
          </button>
        )}
      </header>
      <pre ref={output} onScroll={onScroll} aria-label="Server output" tabIndex={0}>
        {lines.length === 0 ? (
          <span className="dim">{connected ? "No output yet. Start the server to see its log here." : "Connecting to the console…"}</span>
        ) : (
          lines.map((line, index) => (
            <span key={index} className={lineClass(line)}>
              {line + "\n"}
            </span>
          ))
        )}
      </pre>
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

function lineClass(line: string) {
  if (line.startsWith("[consolry]")) return "system";
  if (/\b(ERROR|SEVERE|FATAL)\b|Exception/.test(line)) return "bad";
  if (/\bWARN(ING)?\b/.test(line)) return "warn";
  return undefined;
}
