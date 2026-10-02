import { useEffect, useState } from "react";
import { api, stateLabel, type Finding, type PowerAction, type ServerInfo } from "./api";
import Backups from "./Backups";
import Console from "./Console";
import Dashboard, { type UsageHistory } from "./Dashboard";
import Files from "./Files";
import { ActivityPage, NetworkPage, StartupPage } from "./Pages";
import Players from "./Players";
import Plugins from "./Plugins";
import Schedules from "./Schedules";
import Settings, { GameSettings } from "./Settings";
import { ErrorNote, useAction } from "./ui";

export type Tab =
  | "dashboard"
  | "console"
  | "activity"
  | "plugins"
  | "players"
  | "game"
  | "files"
  | "backups"
  | "network"
  | "schedules"
  | "startup"
  | "settings";

type Props = { server: ServerInfo; tab: Tab; now: number; history?: UsageHistory; onChanged: () => void; onDeleted: () => void };

export default function ServerPage({ server, tab, now, history, onChanged, onDeleted }: Props) {
  const { error, run } = useAction();
  const running = server.state === "running";
  const live = running || server.state === "stopping";
  const minecraft = server.kind === "minecraft";
  const power = (action: PowerAction) => run(action, () => api.power(server.id, action), onChanged);

  const tabs: { id: Tab; label: string }[] = [
    { id: "dashboard", label: "Dashboard" },
    { id: "console", label: "Console" },
    ...(minecraft
      ? [
          { id: "plugins" as Tab, label: server.software === "fabric" ? "Mods" : "Plugins" },
          { id: "players" as Tab, label: "Players" },
          { id: "game" as Tab, label: "Game settings" },
        ]
      : []),
    { id: "files", label: "Files" },
    { id: "backups", label: "Backups" },
    { id: "schedules", label: "Schedules" },
    { id: "network", label: "Network" },
    { id: "startup", label: "Startup" },
    { id: "activity", label: "Activity" },
    { id: "settings", label: "Settings" },
  ];
  const current = tabs.some((item) => item.id === tab) ? tab : "dashboard";

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
          <a key={item.id} href={`#/servers/${server.id}${item.id === "dashboard" ? "" : `/${item.id}`}`} aria-current={item.id === current ? "page" : undefined}>
            {item.label}
          </a>
        ))}
      </nav>

      {(current === "dashboard" || current === "console") && <Diagnosis server={server} />}
      {current === "dashboard" && <Dashboard server={server} now={now} history={history} />}
      {current === "console" && <Console serverId={server.id} running={running} />}
      {current === "activity" && <ActivityPage server={server} />}
      {current === "plugins" && <Plugins server={server} />}
      {current === "players" && <Players server={server} />}
      {current === "game" && <GameSettings server={server} />}
      {current === "files" && <Files serverId={server.id} />}
      {current === "backups" && <Backups server={server} />}
      {current === "network" && <NetworkPage server={server} />}
      {current === "schedules" && <Schedules server={server} />}
      {current === "startup" && <StartupPage server={server} onChanged={onChanged} />}
      {current === "settings" && <Settings server={server} onChanged={onChanged} onDeleted={onDeleted} />}
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
