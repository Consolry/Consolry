import { useEffect, useState } from "react";
import { api, formatBytes, formatTime, stateLabel, uptime, type Activity, type Network, type Permission, type Players, type ServerInfo } from "./api";

export type UsageHistory = { cpu: number[]; memory: number[] };

function Spark({ values, max }: { values: number[]; max: number }) {
  if (values.length < 2) return <svg className="spark" viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true" />;
  const top = Math.max(max, ...values, 1);
  const points = values.map((value, index) => `${(index / (values.length - 1)) * 100},${27 - (value / top) * 26}`).join(" ");
  return (
    <svg className="spark" viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true">
      <polyline points={points} />
    </svg>
  );
}

type Props = { server: ServerInfo; now: number; history?: UsageHistory };

export default function Dashboard({ server, now, history }: Props) {
  const [network, setNetwork] = useState<Network | null>(null);
  const [players, setPlayers] = useState<Players | null>(null);
  const [activity, setActivity] = useState<Activity[]>([]);
  const running = server.state === "running";
  const minecraft = server.kind === "minecraft";

  useEffect(() => {
    // Each part is shown only to people allowed to see it.
    const can = (permission: Permission) => server.permissions.includes(permission);
    if (can("network")) api.network(server.id).then(setNetwork, () => {});
    if (can("activity")) api.activity(server.id, 6).then(setActivity, () => {});
    // Asking who is online types "list" into the console, so it is done once per visit, not on a timer.
    if (minecraft && running && can("players")) api.players(server.id).then(setPlayers, () => {});
    else setPlayers(null);
  }, [server.id, server.state, server.permissions, minecraft, running]);

  const limit = server.memoryLimitMb * 1024 * 1024;
  const address = network?.addresses[0];
  const port = network && network.port && network.port !== 25565 ? `:${network.port}` : "";

  return (
    <>
      <dl className="tiles dash">
        <div className={server.state === "crashed" ? "alert" : undefined}>
          <dt>Status</dt>
          <dd>{stateLabel(server.state)}</dd>
          <p className="dim">
            {running ? `Up ${uptime(server.startedAt, now)}` : server.state === "starting" ? `Loading for ${uptime(server.startedAt, now)}` : "Not running"}
          </p>
        </div>
        <div>
          <dt>CPU</dt>
          <dd>
            {running ? server.cpu.toFixed(server.cpu < 10 ? 1 : 0) : "–"}
            <small>{running ? "% of this machine" : ""}</small>
          </dd>
          <Spark values={history?.cpu ?? []} max={25} />
        </div>
        <div>
          <dt>Memory</dt>
          <dd>
            {running ? formatBytes(server.memory) : "–"}
            {limit > 0 && <small> in use · Java limit {formatBytes(limit)}</small>}
          </dd>
          <Spark values={history?.memory ?? []} max={limit} />
        </div>
        {minecraft && (
          <div>
            <dt>Players</dt>
            <dd>
              {players ? players.online.length : "–"}
              {players && players.max > 0 && <small> of {players.max}</small>}
            </dd>
            <p className="dim names">{players?.online.length ? players.online.join(", ") : running ? "Nobody online" : "Server is off"}</p>
          </div>
        )}
        {minecraft && (
          <div className="wide">
            <dt>Join address on your network</dt>
            <dd className="mono big">{address ? `${address}${port}` : "–"}</dd>
            <p className="dim">
              {network?.listening ? "Accepting connections." : "Not accepting connections right now."}{" "}
              <a href={`#/servers/${server.id}/network`}>How friends outside your home can join</a>
            </p>
          </div>
        )}
        {minecraft && (
          <div>
            <dt>Version</dt>
            <dd>{server.mcVersion}</dd>
            <p className="dim">{server.software}</p>
          </div>
        )}
      </dl>

      <div className="panel-block">
        <header className="bar">
          <h2 className="grow">Recent activity</h2>
          <a className="button small" href={`#/servers/${server.id}/activity`}>
            See all
          </a>
        </header>
        {activity.length === 0 ? (
          <p className="dim pad">Nothing has happened yet.</p>
        ) : (
          <ul className="rows">
            {activity.map((entry) => (
              <li key={entry.id}>
                <span className="grow">{entry.text}</span>
                <span className="dim">
                  {entry.user} · {formatTime(entry.at)}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </>
  );
}
