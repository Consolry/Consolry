import { useEffect, useState } from "react";
import { api, stateLabel, uptime, type Finding, type PowerAction, type ServerInfo } from "./api";
import Backups from "./Backups";
import Console from "./Console";
import Files from "./Files";
import Plugins from "./Plugins";
import { ErrorNote, useAction } from "./ui";

export type Tab = "console" | "files" | "plugins" | "backups";

type Props = { server: ServerInfo; tab: Tab; now: number; onChanged: () => void; onDeleted: () => void };

export default function ServerPage({ server, tab, now, onChanged, onDeleted }: Props) {
  const { error, run } = useAction();
  const [confirming, setConfirming] = useState(false);
  const running = server.state === "running";
  const live = running || server.state === "stopping";
  const minecraft = server.kind === "minecraft";
  const power = (action: PowerAction) => run(action, () => api.power(server.id, action), onChanged);

  const tabs: { id: Tab; label: string }[] = [
    { id: "console", label: "Console" },
    { id: "files", label: "Files" },
    ...(minecraft ? [{ id: "plugins" as Tab, label: server.software === "fabric" ? "Mods" : "Plugins" }] : []),
    { id: "backups", label: "Backups" },
  ];
  const current = tabs.some((item) => item.id === tab) ? tab : "console";

  return (
    <>
      <header className="page-head">
        <div className="title">
          <h1>{server.name}</h1>
          <span className={`state ${server.state}`}>{stateLabel(server.state)}</span>
          {minecraft && (
            <span className="chip">
              {server.software} {server.mcVersion}
            </span>
          )}
        </div>
        <div className="actions">
          <button className="primary" disabled={live || server.state === "unreachable"} onClick={() => power("start")}>
            Start
          </button>
          <button disabled={!running} onClick={() => power("stop")}>
            Stop
          </button>
          <button className="danger" disabled={!live} onClick={() => power("kill")}>
            Kill
          </button>
        </div>
      </header>
      <ErrorNote message={error} />

      <nav className="tabs" aria-label="Server sections">
        {tabs.map((item) => (
          <a key={item.id} href={`#/servers/${server.id}${item.id === "console" ? "" : `/${item.id}`}`} aria-current={item.id === current ? "page" : undefined}>
            {item.label}
          </a>
        ))}
      </nav>

      {current === "console" && (
        <>
          <dl className="tiles">
            <div>
              <dt>Uptime</dt>
              <dd>{uptime(server.startedAt, now)}</dd>
            </div>
            <div>
              <dt>Node</dt>
              <dd>{server.nodeName}</dd>
            </div>
            <div className="wide">
              <dt>Start command</dt>
              <dd className="mono">{[server.command, ...server.args].join(" ") || "–"}</dd>
            </div>
          </dl>
          <Diagnosis server={server} />
          <Console serverId={server.id} running={running} />
          <footer className="remove">
            {confirming ? (
              <>
                <span>Remove {server.name} from the panel? Its files stay on the node.</span>
                <button className="danger" onClick={() => run("remove", () => api.deleteServer(server.id), onDeleted)}>
                  Remove
                </button>
                <button onClick={() => setConfirming(false)}>Cancel</button>
              </>
            ) : (
              <button className="small" disabled={live} onClick={() => setConfirming(true)} title={live ? "Stop the server first" : undefined}>
                Remove server
              </button>
            )}
          </footer>
        </>
      )}
      {current === "files" && <Files serverId={server.id} />}
      {current === "plugins" && <Plugins server={server} />}
      {current === "backups" && <Backups server={server} />}
    </>
  );
}

/** Reads the latest run's log for known problems whenever the server stops or crashes. */
function Diagnosis({ server }: { server: ServerInfo }) {
  const [findings, setFindings] = useState<Finding[]>([]);

  useEffect(() => {
    if (server.state !== "crashed" && server.state !== "offline") {
      setFindings([]);
      return;
    }
    let cancelled = false;
    api.diagnosis(server.id).then(
      (result) => !cancelled && setFindings(result),
      () => {},
    );
    return () => {
      cancelled = true;
    };
  }, [server.id, server.state]);

  if (findings.length === 0) return null;
  return (
    <section className="diagnosis" aria-label="What went wrong">
      {findings.map((finding) => (
        <article key={finding.title}>
          <p className="pixel">What went wrong</p>
          <h2>{finding.title}</h2>
          <p>{finding.detail}</p>
          <p className="fix">
            <strong>To fix it:</strong> {finding.fix}
          </p>
          <code>{finding.line}</code>
        </article>
      ))}
    </section>
  );
}
