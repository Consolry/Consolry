import { useCallback, useEffect, useState, type FormEvent } from "react";
import { api, type Players as PlayerData, type ServerInfo } from "./api";
import { ErrorNote, useAction } from "./ui";

export default function Players({ server }: { server: ServerInfo }) {
  const [data, setData] = useState<PlayerData | null>(null);
  const { error, busy, run, setError } = useAction();
  const running = server.state === "running";

  const load = useCallback(() => {
    api.players(server.id).then(setData, (problem) => setError((problem as Error).message));
  }, [server.id, setError]);
  // Reload when the server starts or stops, since most of this page depends on it.
  useEffect(load, [load, running]);

  const act = (action: string, name = "", reason = "") => run(`${action} ${name}`, () => api.playerAction(server.id, action, name, reason), load);

  if (!data) return <ErrorNote message={error} />;
  const operators = new Set(data.operators.map((name) => name.toLowerCase()));

  return (
    <section className="players">
      <ErrorNote message={error} />
      {!running && <p className="note">The server is off. These lists are read from its files; start it to add, remove, kick or ban players.</p>}

      <div className="panel-block">
        <header className="bar">
          <h2 className="grow">
            Online{" "}
            <span className="dim">
              {running ? data.online.length : "–"}
              {data.max > 0 && ` of ${data.max}`}
            </span>
          </h2>
          <button className="small" onClick={load}>
            Refresh
          </button>
        </header>
        {data.online.length === 0 ? (
          <p className="dim pad">{running ? "Nobody is online right now." : "The server is not running."}</p>
        ) : (
          <ul className="rows">
            {data.online.map((name) => (
              <li key={name}>
                <span className="grow">
                  <strong>{name}</strong>
                  {operators.has(name.toLowerCase()) && <small>Operator</small>}
                </span>
                {operators.has(name.toLowerCase()) ? (
                  <button className="small" disabled={busy !== ""} onClick={() => act("deop", name)}>
                    Remove operator
                  </button>
                ) : (
                  <button className="small" disabled={busy !== ""} onClick={() => act("op", name)}>
                    Make operator
                  </button>
                )}
                <button className="small" disabled={busy !== ""} onClick={() => act("kick", name)}>
                  Kick
                </button>
                <button className="danger small" disabled={busy !== ""} onClick={() => act("ban", name)}>
                  Ban
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      <NameList
        title="Whitelist"
        names={data.whitelist}
        empty="Nobody is on the whitelist."
        addLabel="Add to whitelist"
        disabled={!running || busy !== ""}
        onAdd={(name) => act("whitelist-add", name)}
        onRemove={(name) => act("whitelist-remove", name)}
        header={
          <>
            <span className={data.whitelistEnabled ? "state running" : "state"}>{data.whitelistEnabled ? "On" : "Off"}</span>
            <button className="small" disabled={!running || busy !== ""} onClick={() => act(data.whitelistEnabled ? "whitelist-off" : "whitelist-on")}>
              Turn {data.whitelistEnabled ? "off" : "on"}
            </button>
          </>
        }
        note={data.whitelistEnabled ? "Only these players can join." : "The whitelist is off, so anyone can join."}
      />

      <NameList
        title="Operators"
        names={data.operators}
        empty="There are no operators."
        addLabel="Make operator"
        disabled={!running || busy !== ""}
        onAdd={(name) => act("op", name)}
        onRemove={(name) => act("deop", name)}
        note="Operators can run every command in the game."
      />

      <NameList
        title="Banned"
        names={data.banned.map((entry) => entry.name)}
        details={Object.fromEntries(data.banned.map((entry) => [entry.name, entry.reason]))}
        empty="Nobody is banned."
        addLabel="Ban"
        removeLabel="Unban"
        disabled={!running || busy !== ""}
        onAdd={(name) => act("ban", name)}
        onRemove={(name) => act("pardon", name)}
      />
    </section>
  );
}

type NameListProps = {
  title: string;
  names: string[];
  details?: Record<string, string>;
  empty: string;
  addLabel: string;
  removeLabel?: string;
  disabled: boolean;
  onAdd: (name: string) => void;
  onRemove: (name: string) => void;
  header?: React.ReactNode;
  note?: string;
};

function NameList({ title, names, details, empty, addLabel, removeLabel = "Remove", disabled, onAdd, onRemove, header, note }: NameListProps) {
  const [name, setName] = useState("");

  function submit(event: FormEvent) {
    event.preventDefault();
    if (name.trim()) onAdd(name.trim());
    setName("");
  }

  return (
    <div className="panel-block">
      <header className="bar">
        <h2 className="grow">
          {title} <span className="dim">{names.length}</span>
        </h2>
        {header}
      </header>
      {note && <p className="dim pad">{note}</p>}
      {names.length === 0 ? (
        <p className="dim pad">{empty}</p>
      ) : (
        <ul className="rows">
          {names.map((item) => (
            <li key={item}>
              <span className="grow">
                <strong>{item}</strong>
                {details?.[item] && <small>{details[item]}</small>}
              </span>
              <button className="small" disabled={disabled} onClick={() => onRemove(item)}>
                {removeLabel}
              </button>
            </li>
          ))}
        </ul>
      )}
      <form className="bar search" onSubmit={submit}>
        <input
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Player name"
          aria-label={`${addLabel}: player name`}
          pattern="[A-Za-z0-9_.*]{1,32}"
          title="Letters, digits, underscores and dots"
          disabled={disabled}
        />
        <button className="primary small" disabled={disabled || !name.trim()}>
          {addLabel}
        </button>
      </form>
    </div>
  );
}
