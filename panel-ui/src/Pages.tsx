import { useCallback, useEffect, useState, type FormEvent } from "react";
import { api, formatTime, isLive, type Activity, type Network, type ServerInfo, type Startup } from "./api";
import { Empty, ErrorNote, useAction } from "./ui";

/** Everything that has been done to this server, newest first. */
export function ActivityPage({ server }: { server: ServerInfo }) {
  const [entries, setEntries] = useState<Activity[] | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    api.activity(server.id, 200).then(setEntries, (problem) => setError((problem as Error).message));
  }, [server.id]);
  useEffect(load, [load]);

  return (
    <section className="stack">
      <header className="bar">
        <p className="grow dim">A record of what was done to this server and who did it. The newest 500 entries are kept.</p>
        <button className="small" onClick={load}>
          Refresh
        </button>
      </header>
      <ErrorNote message={error} />
      {entries === null ? null : entries.length === 0 ? (
        <Empty title="Nothing yet">Starting, stopping, installing plugins and changing files all show up here.</Empty>
      ) : (
        <table className="list">
          <thead>
            <tr>
              <th>What happened</th>
              <th>Who</th>
              <th>When</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((entry) => (
              <tr key={entry.id}>
                <td>{entry.text}</td>
                <td className="dim">{entry.user}</td>
                <td className="dim">{formatTime(entry.at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

/** How players reach the server: its port and this machine's addresses. */
export function NetworkPage({ server }: { server: ServerInfo }) {
  const [network, setNetwork] = useState<Network | null>(null);
  const [port, setPort] = useState("");
  const [saved, setSaved] = useState(false);
  const { error, busy, run, setError } = useAction();
  const minecraft = server.kind === "minecraft";
  const live = isLive(server.state);

  const load = useCallback(() => {
    api.network(server.id).then(
      (result) => {
        setNetwork(result);
        setPort(String(result.port || ""));
      },
      (problem) => setError((problem as Error).message),
    );
  }, [server.id, setError]);
  useEffect(load, [load, server.state]);

  if (!network) return <ErrorNote message={error} />;
  if (!minecraft) {
    return (
      <section className="stack">
        <div className="card form">
          <h2>This machine's addresses</h2>
          <p className="dim">Consolry doesn't know which port a custom server uses. These are the addresses other devices on your network can reach this machine at.</p>
          <p className="mono">{network.addresses.join(", ") || "No local network address found."}</p>
        </div>
      </section>
    );
  }

  const suffix = network.port === 25565 ? "" : `:${network.port}`;
  const valid = /^\d+$/.test(port) && Number(port) >= 1024 && Number(port) <= 65535;

  function savePort(event: FormEvent) {
    event.preventDefault();
    setSaved(false);
    run("port", async () => {
      const text = await api.readFile(server.id, "server.properties");
      const lines = text.split(/\r?\n/);
      const index = lines.findIndex((line) => line.startsWith("server-port="));
      if (index >= 0) lines[index] = `server-port=${port}`;
      else lines.push(`server-port=${port}`);
      await api.writeFile(server.id, "server.properties", lines.join("\n"));
      setSaved(true);
      load();
    });
  }

  return (
    <section className="stack">
      <ErrorNote message={error} />
      <dl className="tiles">
        <div className="wide">
          <dt>On your home network</dt>
          <dd className="mono big">{network.addresses.length ? `${network.addresses[0]}${suffix}` : "–"}</dd>
          <p className="dim">Anyone on the same Wi-Fi or network types this into Minecraft.</p>
        </div>
        <div>
          <dt>On this computer</dt>
          <dd className="mono big">localhost{suffix}</dd>
        </div>
        <div className={network.listening ? undefined : "muted"}>
          <dt>Port {network.port}</dt>
          <dd>{network.listening ? "Open" : "Closed"}</dd>
          <p className="dim">{network.listening ? "The server is accepting connections." : "Nothing is listening. Start the server."}</p>
        </div>
      </dl>

      <div className="card form wide">
        <h2>Letting friends outside your home join</h2>
        <ol className="plain-steps">
          <li>
            In your router's settings, forward TCP port <strong>{network.port}</strong> to{" "}
            <strong>{network.addresses[0] ?? "this machine's address"}</strong>.
          </li>
          <li>Find your public address by searching "what is my IP" on this machine.</li>
          <li>
            Friends join with that address{suffix && <>, followed by <strong>{suffix}</strong></>}.
          </li>
        </ol>
        <p className="dim">
          Consolry does not forward ports for you yet. Some internet providers share one address between many customers, in which case forwarding cannot work
          and you would need a tunnel service instead. Turn on the whitelist in Players before sharing your address.
        </p>
      </div>

      <form className="card form" onSubmit={savePort}>
        <h2>Port</h2>
        <label>
          Server port
          <input value={port} onChange={(event) => setPort(event.target.value)} inputMode="numeric" disabled={live} />
          <small>25565 is Minecraft's default. Use a different one, such as 25566, to run a second server at the same time.</small>
        </label>
        {live && <p className="dim">Stop the server to change its port.</p>}
        {!valid && <p className="error">Enter a whole number from 1024 to 65535.</p>}
        {saved && <p className="note">Saved. It applies the next time the server starts.</p>}
        <button className="primary" disabled={live || !valid || Number(port) === network.port || busy !== ""}>
          Save port
        </button>
      </form>
    </section>
  );
}

const memoryChoices = [1, 2, 3, 4, 6, 8, 12, 16, 24, 32];

/** How the server is launched: memory, Java and the command itself. */
export function StartupPage({ server, onChanged }: { server: ServerInfo; onChanged: () => void }) {
  const [startup, setStartup] = useState<Startup | null>(null);
  const [memory, setMemory] = useState(0);
  const [options, setOptions] = useState("");
  const [startCommand, setStartCommand] = useState("");
  const [stopCommand, setStopCommand] = useState("");
  const [saved, setSaved] = useState(false);
  const { error, busy, run, setError } = useAction();
  const live = isLive(server.state);

  const load = useCallback(() => {
    api.startup(server.id).then(
      (result) => {
        setStartup(result);
        setMemory(result.memoryMb);
        setOptions(result.javaOptions.join(" "));
        setStartCommand([result.command, ...result.args].join(" "));
        setStopCommand(result.stopCommand);
      },
      (problem) => setError((problem as Error).message),
    );
  }, [server.id, setError]);
  useEffect(load, [load]);

  if (!startup) return <ErrorNote message={error} />;
  const java = startup.command === "java";
  const original = { options: startup.javaOptions.join(" "), command: [startup.command, ...startup.args].join(" ") };

  function submit(event: FormEvent) {
    event.preventDefault();
    setSaved(false);
    const patch: Parameters<typeof api.updateServer>[1] = {};
    if (java) {
      if (memory !== startup!.memoryMb) patch.memoryMb = memory;
      if (options.trim() !== original.options) patch.javaOptions = options.trim();
    } else if (startCommand !== original.command) {
      patch.startCommand = startCommand;
    }
    if (stopCommand !== startup!.stopCommand) patch.stopCommand = stopCommand;
    if (Object.keys(patch).length === 0) return;
    run("save", () => api.updateServer(server.id, patch), () => {
      setSaved(true);
      load();
      onChanged();
    });
  }

  const sizes = memoryChoices.map((gb) => gb * 1024);
  if (memory && !sizes.includes(memory)) sizes.push(memory);
  sizes.sort((a, b) => a - b);

  const picked =
    startup.javaNeeded === 0
      ? ""
      : startup.systemJava && Number(startup.systemJava.split(".")[0]) >= startup.javaNeeded
        ? `This machine's Java ${startup.systemJava} is used.`
        : startup.managedJava.some((major) => major >= startup.javaNeeded)
          ? `Java ${startup.managedJava.find((major) => major >= startup.javaNeeded)}, installed by Consolry, is used.`
          : `No suitable Java is installed yet. Consolry will not be able to start this server until Java ${startup.javaNeeded} or newer is available.`;

  return (
    <section className="stack">
      {live && <p className="note">Stop the server to change how it starts.</p>}
      <form className="card form wide" onSubmit={submit}>
        <h2>Startup</h2>
        {java ? (
          <>
            <label>
              Memory
              <select value={memory} onChange={(event) => setMemory(Number(event.target.value))} disabled={live}>
                {sizes.map((mb) => (
                  <option key={mb} value={mb}>
                    {mb % 1024 === 0 ? `${mb / 1024} GB` : `${mb} MB`}
                  </option>
                ))}
              </select>
              <small>How much memory Java may use. Leave some free for the rest of this machine.</small>
            </label>
            <label>
              Extra Java options
              <input className="mono" value={options} onChange={(event) => setOptions(event.target.value)} disabled={live} spellCheck={false} placeholder="None" />
              <small>Advanced. Memory and the server file are set for you; anything here is added between them.</small>
            </label>
            {startup.fastStart.length > 0 && (
              <button
                type="button"
                className="small"
                disabled={live}
                onClick={() => setOptions([...startup.fastStart, options].join(" ").trim())}
                title="Java keeps a record of what it loaded and reuses it next time. Measured about 8% faster."
              >
                Add the faster-startup option
              </button>
            )}
            {startup.recommended.length > 0 && (
              <button type="button" className="small" disabled={live} onClick={() => setOptions([...options.split(/\s+/).filter((item) => /SharedArchive|IgnoreUnrecognized/.test(item)), ...startup.recommended].join(" "))}>
                Use Paper's recommended options
              </button>
            )}
          </>
        ) : (
          <label>
            Start command
            <input className="mono" value={startCommand} onChange={(event) => setStartCommand(event.target.value)} disabled={live} required spellCheck={false} />
            <small>Runs inside the server's own folder.</small>
          </label>
        )}
        <label>
          Stop command
          <input className="mono" value={stopCommand} onChange={(event) => setStopCommand(event.target.value)} disabled={live} spellCheck={false} />
          <small>Typed into the console to shut down cleanly. Leave empty to end the process directly.</small>
        </label>
        <ErrorNote message={error} />
        {saved && <p className="note">Saved. It applies the next time the server starts.</p>}
        <button className="primary" disabled={live || busy !== ""}>
          Save
        </button>
      </form>

      <div className="card form wide">
        <h2>What runs</h2>
        <pre className="command">{[startup.command, ...startup.args].join(" ")}</pre>
        {startup.javaNeeded > 0 && (
          <p className="dim">
            This server needs Java {startup.javaNeeded} or newer. {picked}
          </p>
        )}
      </div>
    </section>
  );
}
