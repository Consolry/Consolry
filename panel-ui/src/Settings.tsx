import { useEffect, useState, type FormEvent } from "react";
import { api, type ServerInfo, type Software } from "./api";
import { ErrorNote, useAction } from "./ui";

type Props = { server: ServerInfo; onChanged: () => void; onDeleted: () => void };

export default function Settings({ server, onChanged, onDeleted }: Props) {
  const live = server.state === "running" || server.state === "stopping";
  return (
    <section className="settings">
      {live && <p className="note">Some settings can only be changed while the server is stopped.</p>}
      <General server={server} live={live} onChanged={onChanged} />
      {server.kind === "minecraft" && <VersionSwitch server={server} live={live} onChanged={onChanged} />}
      <Remove server={server} live={live} onDeleted={onDeleted} />
    </section>
  );
}

function General({ server, onChanged }: { server: ServerInfo; live: boolean; onChanged: () => void }) {
  const [name, setName] = useState(server.name);
  const [saved, setSaved] = useState(false);
  const { error, busy, run } = useAction();

  function submit(event: FormEvent) {
    event.preventDefault();
    setSaved(false);
    if (name.trim() === server.name) return;
    run("save", () => api.updateServer(server.id, { name: name.trim() }), () => {
      setSaved(true);
      onChanged();
    });
  }

  return (
    <form className="card form" onSubmit={submit}>
      <h2>Name</h2>
      <label>
        Server name
        <input value={name} onChange={(event) => setName(event.target.value)} required />
        <small>Only shown in this panel. Memory and the start command are on the Startup page.</small>
      </label>
      <ErrorNote message={error} />
      {saved && <p className="note">Saved.</p>}
      <button className="primary" disabled={busy !== "" || name.trim() === server.name}>
        Save
      </button>
    </form>
  );
}

function VersionSwitch({ server, live, onChanged }: { server: ServerInfo; live: boolean; onChanged: () => void }) {
  const [softwares, setSoftwares] = useState<Software[]>([]);
  const [software, setSoftware] = useState(server.software);
  const [versions, setVersions] = useState<string[] | null>(null);
  const [version, setVersion] = useState(server.mcVersion);
  const [notice, setNotice] = useState("");
  const { error, busy, run, setError } = useAction();

  useEffect(() => {
    api.software().then(setSoftwares, () => {});
  }, []);
  useEffect(() => {
    let cancelled = false;
    setVersions(null);
    api.versions(software).then(
      (list) => {
        if (cancelled) return;
        setVersions(list);
        setVersion((current) => (list.includes(current) ? current : (list[0] ?? "")));
      },
      (problem) => !cancelled && setError((problem as Error).message),
    );
    return () => {
      cancelled = true;
    };
  }, [software, setError]);

  const unchanged = software === server.software && version === server.mcVersion;
  const folderChanges = softwares.find((item) => item.id === software)?.folder !== softwares.find((item) => item.id === server.software)?.folder;

  function submit(event: FormEvent) {
    event.preventDefault();
    setNotice("");
    run("switch", async () => {
      const { warning } = await api.switchVersion(server.id, software, version);
      setNotice(warning || `Switched to ${software} ${version}. A backup was taken first.`);
      onChanged();
    });
  }

  return (
    <form className="card form" onSubmit={submit}>
      <h2>Version</h2>
      <p className="dim">
        Currently {server.software} {server.mcVersion}. Switching downloads the new server, picks the Java it needs, and takes a backup first.
      </p>
      <div className="pair">
        <label>
          Server software
          <select value={software} onChange={(event) => setSoftware(event.target.value)} disabled={live}>
            {softwares.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Minecraft version
          <select value={version} onChange={(event) => setVersion(event.target.value)} disabled={live || !versions}>
            {versions ? versions.map((item) => <option key={item}>{item}</option>) : <option>Loading…</option>}
          </select>
        </label>
      </div>
      {!unchanged && folderChanges && softwares.length > 0 && (
        <p className="note warn">Plugins and mods are different things. What you have installed now will not load on {software}.</p>
      )}
      {!unchanged && <p className="note warn">Moving a world to an older Minecraft version can damage it. Plugins may also need updating afterwards.</p>}
      <ErrorNote message={error} />
      {notice && <p className="note">{notice}</p>}
      <button className="primary" disabled={live || unchanged || !version || busy !== ""}>
        {busy === "switch" ? "Switching…" : "Switch version"}
      </button>
    </form>
  );
}

type Field =
  | { key: string; label: string; hint?: string; type: "text" }
  | { key: string; label: string; hint?: string; type: "number"; min: number; max: number }
  | { key: string; label: string; hint?: string; type: "bool" }
  | { key: string; label: string; hint?: string; type: "choice"; options: string[] };

// The settings most people change. Everything else in server.properties is left exactly as it is.
const fields: Field[] = [
  { key: "motd", label: "Message in the server list", type: "text" },
  { key: "max-players", label: "Player limit", type: "number", min: 1, max: 1000 },
  { key: "gamemode", label: "Game mode", type: "choice", options: ["survival", "creative", "adventure", "spectator"] },
  { key: "difficulty", label: "Difficulty", type: "choice", options: ["peaceful", "easy", "normal", "hard"] },
  { key: "pvp", label: "Players can hurt each other", type: "bool" },
  { key: "white-list", label: "Only whitelisted players can join", type: "bool" },
  { key: "online-mode", label: "Check players own Minecraft", type: "bool", hint: "Turning this off lets anyone join under any name. Leave it on unless you know why you need it off." },
  { key: "hardcore", label: "Hardcore", type: "bool" },
  { key: "allow-flight", label: "Allow flying", type: "bool", hint: "Stops players being kicked for flying with mods or elytra glitches." },
  { key: "enable-command-block", label: "Command blocks", type: "bool" },
  { key: "view-distance", label: "View distance, in chunks", type: "number", min: 2, max: 32, hint: "Lower values reduce lag." },
  { key: "simulation-distance", label: "Simulation distance, in chunks", type: "number", min: 2, max: 32 },
  { key: "spawn-protection", label: "Spawn protection, in blocks", type: "number", min: 0, max: 256 },
  { key: "server-port", label: "Port", type: "number", min: 1024, max: 65535, hint: "Players add this to the address unless it is 25565." },
  { key: "level-seed", label: "World seed", type: "text", hint: "Only used when a new world is created." },
];

function parseProperties(text: string) {
  const values: Record<string, string> = {};
  for (const line of text.split(/\r?\n/)) {
    if (line.startsWith("#") || !line.includes("=")) continue;
    const index = line.indexOf("=");
    values[line.slice(0, index).trim()] = line.slice(index + 1);
  }
  return values;
}

/** Rewrites only the changed lines, keeping comments, order and every other setting. */
function applyProperties(text: string, changes: Record<string, string>) {
  const pending = { ...changes };
  const lines = text.split(/\r?\n/).map((line) => {
    if (line.startsWith("#") || !line.includes("=")) return line;
    const key = line.slice(0, line.indexOf("=")).trim();
    if (!(key in pending)) return line;
    const value = pending[key];
    delete pending[key];
    return `${key}=${value}`;
  });
  while (lines.length && lines[lines.length - 1] === "") lines.pop();
  for (const [key, value] of Object.entries(pending)) lines.push(`${key}=${value}`);
  return lines.join("\n") + "\n";
}

export function GameSettings({ server }: { server: ServerInfo }) {
  const [original, setOriginal] = useState<string | null>(null);
  const [missing, setMissing] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [saved, setSaved] = useState(false);
  const { error, busy, run } = useAction();
  const running = server.state === "running";

  useEffect(() => {
    api.readFile(server.id, "server.properties").then(
      (text) => {
        setOriginal(text);
        setValues(parseProperties(text));
      },
      () => setMissing(true),
    );
  }, [server.id]);

  if (missing) {
    return (
      <div className="card form">
        <h2>Game settings</h2>
        <p className="dim">Start the server once. Minecraft creates its settings file on the first run, and it will appear here.</p>
      </div>
    );
  }
  if (original === null) return null;

  const before = parseProperties(original);
  // Minecraft versions differ in which settings exist, so only offer the ones this server has.
  const shown = fields.filter((field) => field.key in before);
  const changes: Record<string, string> = {};
  const problems: string[] = [];
  for (const field of shown) {
    const value = values[field.key];
    if (value === undefined || value === before[field.key]) continue;
    if (field.type === "number" && (!/^\d+$/.test(value) || Number(value) < field.min || Number(value) > field.max)) {
      problems.push(`${field.label} must be a whole number from ${field.min} to ${field.max}.`);
      continue;
    }
    changes[field.key] = value;
  }
  const changed = Object.keys(changes);

  function submit(event: FormEvent) {
    event.preventDefault();
    setSaved(false);
    const next = applyProperties(original!, changes);
    run("save", () => api.writeFile(server.id, "server.properties", next), () => {
      setOriginal(next);
      setSaved(true);
    });
  }

  return (
    <form className="card form wide" onSubmit={submit}>
      <h2>Game settings</h2>
      <div className="fields">
        {shown.map((field) => {
          const value = values[field.key] ?? "";
          const set = (next: string) => {
            setSaved(false);
            setValues({ ...values, [field.key]: next });
          };
          return (
            <label key={field.key} className={field.type === "bool" ? "check" : undefined}>
              {field.type === "bool" ? (
                <>
                  <input type="checkbox" checked={value === "true"} onChange={(event) => set(String(event.target.checked))} />
                  <span>
                    {field.label}
                    {field.hint && <small>{field.hint}</small>}
                  </span>
                </>
              ) : (
                <>
                  {field.label}
                  {field.type === "choice" ? (
                    <select value={value} onChange={(event) => set(event.target.value)}>
                      {!field.options.includes(value) && <option value={value}>{value || "Not set"}</option>}
                      {field.options.map((option) => (
                        <option key={option}>{option}</option>
                      ))}
                    </select>
                  ) : (
                    <input value={value} onChange={(event) => set(event.target.value)} inputMode={field.type === "number" ? "numeric" : undefined} />
                  )}
                  {field.hint && <small>{field.hint}</small>}
                </>
              )}
            </label>
          );
        })}
      </div>

      {problems.map((problem) => (
        <p key={problem} className="error" role="alert">
          {problem}
        </p>
      ))}
      {changed.length > 0 && (
        <div className="diff">
          <p className="pixel">
            {changed.length} line{changed.length === 1 ? "" : "s"} will change in server.properties
          </p>
          {changed.map((key) => (
            <pre key={key}>
              {before[key] !== undefined && <del>{`${key}=${before[key]}`}</del>}
              <ins>{`${key}=${changes[key]}`}</ins>
            </pre>
          ))}
        </div>
      )}
      <ErrorNote message={error} />
      {saved && <p className="note">Saved.{running ? " Restart the server for the changes to take effect." : ""}</p>}
      <button className="primary" disabled={changed.length === 0 || problems.length > 0 || busy !== ""}>
        Save game settings
      </button>
    </form>
  );
}

function Remove({ server, live, onDeleted }: { server: ServerInfo; live: boolean; onDeleted: () => void }) {
  const [confirming, setConfirming] = useState(false);
  const { error, run } = useAction();
  return (
    <div className="card form">
      <h2>Remove server</h2>
      <p className="dim">Takes the server off the panel. Its files and backups stay on disk.</p>
      <ErrorNote message={error} />
      {confirming ? (
        <div className="bar">
          <span>Remove {server.name}?</span>
          <button className="danger" onClick={() => run("remove", () => api.deleteServer(server.id), onDeleted)}>
            Remove
          </button>
          <button onClick={() => setConfirming(false)}>Cancel</button>
        </div>
      ) : (
        <button className="danger" disabled={live} onClick={() => setConfirming(true)} title={live ? "Stop the server first" : undefined}>
          Remove server
        </button>
      )}
    </div>
  );
}
