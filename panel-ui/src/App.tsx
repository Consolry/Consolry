import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
import { api, type NodeInfo, type PanelState, type ServerInfo } from "./api";
import Console from "./Console";

type View = { kind: "server"; id: string } | { kind: "new-server" } | { kind: "nodes" };

export default function App() {
  const [state, setState] = useState<PanelState | null>(null);
  const [failed, setFailed] = useState(false);

  const refresh = useCallback(() => {
    api.state().then(setState, () => setFailed(true));
  }, []);
  useEffect(refresh, [refresh]);

  if (failed) return <Centered title="Can't reach the panel">Check that the Consolry panel is running, then reload.</Centered>;
  if (!state) return null;
  if (state.setupNeeded) return <AuthForm mode="setup" onDone={refresh} />;
  if (!state.user) return <AuthForm mode="login" onDone={refresh} />;
  return <Panel username={state.user.username} version={state.version} onSignOut={() => api.logout().then(refresh)} />;
}

function Centered({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="centered">
      <div className="card">
        <p className="brand">Consolry</p>
        <h1>{title}</h1>
        {children}
      </div>
    </main>
  );
}

function AuthForm({ mode, onDone }: { mode: "setup" | "login"; onDone: () => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const setup = mode === "setup";

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await (setup ? api.setup(username, password) : api.login(username, password));
      onDone();
    } catch (problem) {
      setError((problem as Error).message);
      setBusy(false);
    }
  }

  return (
    <Centered title={setup ? "Create the admin account" : "Sign in"}>
      {setup && <p className="dim">This is the first run. The account you create here manages the whole panel.</p>}
      <form onSubmit={submit}>
        <label>
          Username
          <input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" required autoFocus />
        </label>
        <label>
          Password
          <input
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete={setup ? "new-password" : "current-password"}
            minLength={setup ? 10 : undefined}
            required
          />
          {setup && <small>At least 10 characters.</small>}
        </label>
        {error && <p className="error" role="alert">{error}</p>}
        <button className="primary" disabled={busy}>
          {setup ? "Create account" : "Sign in"}
        </button>
      </form>
    </Centered>
  );
}

function Panel({ username, version, onSignOut }: { username: string; version: string; onSignOut: () => void }) {
  const [servers, setServers] = useState<ServerInfo[] | null>(null);
  const [nodes, setNodes] = useState<NodeInfo[]>([]);
  const [view, setView] = useState<View | null>(null);

  const loadServers = useCallback(() => api.servers().then(setServers, () => {}), []);
  const loadNodes = useCallback(() => api.nodes().then(setNodes, () => {}), []);

  useEffect(() => {
    loadServers();
    loadNodes();
    const timer = window.setInterval(loadServers, 2000);
    return () => window.clearInterval(timer);
  }, [loadServers, loadNodes]);

  if (!servers) return null;

  const current: View =
    view ?? (nodes.length === 0 ? { kind: "nodes" } : servers.length > 0 ? { kind: "server", id: servers[0].id } : { kind: "new-server" });
  const selected = current.kind === "server" ? servers.find((server) => server.id === current.id) : undefined;

  return (
    <div className="layout">
      <aside className="sidebar">
        <p className="brand">Consolry</p>
        <p className="heading">Servers</p>
        <nav aria-label="Servers">
          {servers.map((server) => (
            <button
              key={server.id}
              className={selected?.id === server.id ? "on" : undefined}
              onClick={() => setView({ kind: "server", id: server.id })}
            >
              <span className={`dot ${server.state}`} aria-hidden="true" />
              <span>
                {server.name}
                <small>{server.state}</small>
              </span>
            </button>
          ))}
          {servers.length === 0 && <p className="dim empty">No servers yet.</p>}
        </nav>
        <button className={current.kind === "new-server" ? "link on" : "link"} onClick={() => setView({ kind: "new-server" })}>
          New server
        </button>
        <button className={current.kind === "nodes" ? "link on" : "link"} onClick={() => setView({ kind: "nodes" })}>
          Nodes
        </button>
        <div className="account">
          <span>{username}</span>
          <button className="link" onClick={onSignOut}>
            Sign out
          </button>
          <small>v{version}</small>
        </div>
      </aside>

      <main className="content">
        {current.kind === "server" && selected && (
          <ServerPage
            key={selected.id}
            server={selected}
            onChanged={loadServers}
            onDeleted={() => {
              setView(null);
              loadServers();
            }}
          />
        )}
        {current.kind === "server" && !selected && <p className="dim">That server no longer exists.</p>}
        {current.kind === "new-server" && (
          <NewServer
            nodes={nodes}
            onCreated={(id) => {
              loadServers().then(() => setView({ kind: "server", id }));
            }}
            onNeedNode={() => setView({ kind: "nodes" })}
          />
        )}
        {current.kind === "nodes" && <Nodes nodes={nodes} onChanged={loadNodes} />}
      </main>
    </div>
  );
}

function ServerPage({ server, onChanged, onDeleted }: { server: ServerInfo; onChanged: () => void; onDeleted: () => void }) {
  const [error, setError] = useState("");
  const [confirming, setConfirming] = useState(false);
  const running = server.state === "running";
  const active = running || server.state === "stopping";

  async function run(action: () => Promise<unknown>, after: () => void) {
    setError("");
    try {
      await action();
      after();
    } catch (problem) {
      setError((problem as Error).message);
    }
  }

  return (
    <>
      <header className="page-head">
        <div>
          <h1>{server.name}</h1>
          <p className="dim">
            <span className={`dot ${server.state}`} aria-hidden="true" /> {server.state} · {server.nodeName} ·{" "}
            <code>{[server.command, ...server.args].join(" ")}</code>
          </p>
        </div>
        <div className="actions">
          <button className="primary" disabled={active || server.state === "unreachable"} onClick={() => run(() => api.power(server.id, "start"), onChanged)}>
            Start
          </button>
          <button disabled={!running} onClick={() => run(() => api.power(server.id, "stop"), onChanged)}>
            Stop
          </button>
          <button disabled={!active} onClick={() => run(() => api.power(server.id, "kill"), onChanged)}>
            Kill
          </button>
        </div>
      </header>
      {error && <p className="error" role="alert">{error}</p>}

      <Console serverId={server.id} running={running} />

      <section className="danger">
        {confirming ? (
          <>
            <span>Remove {server.name} from the panel? Its files stay on the node.</span>
            <button className="danger-button" onClick={() => run(() => api.deleteServer(server.id), onDeleted)}>
              Remove
            </button>
            <button onClick={() => setConfirming(false)}>Cancel</button>
          </>
        ) : (
          <button disabled={active} onClick={() => setConfirming(true)} title={active ? "Stop the server first" : undefined}>
            Remove server
          </button>
        )}
      </section>
    </>
  );
}

function NewServer({ nodes, onCreated, onNeedNode }: { nodes: NodeInfo[]; onCreated: (id: string) => void; onNeedNode: () => void }) {
  const [name, setName] = useState("");
  const [nodeId, setNodeId] = useState(nodes[0]?.id ?? 0);
  const [startCommand, setStartCommand] = useState("java -Xmx2G -jar server.jar nogui");
  const [stopCommand, setStopCommand] = useState("stop");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  if (nodes.length === 0) {
    return (
      <>
        <h1>New server</h1>
        <p className="dim">A server needs a node to run on. Add one first.</p>
        <button className="primary" onClick={onNeedNode}>
          Add a node
        </button>
      </>
    );
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const { id } = await api.createServer({ name, nodeId: nodeId || nodes[0].id, startCommand, stopCommand });
      onCreated(id);
    } catch (problem) {
      setError((problem as Error).message);
      setBusy(false);
    }
  }

  return (
    <>
      <h1>New server</h1>
      <form className="form" onSubmit={submit}>
        <label>
          Name
          <input value={name} onChange={(event) => setName(event.target.value)} required autoFocus />
        </label>
        <label>
          Node
          <select value={nodeId || nodes[0].id} onChange={(event) => setNodeId(Number(event.target.value))}>
            {nodes.map((node) => (
              <option key={node.id} value={node.id}>
                {node.name}
                {node.online ? "" : " (offline)"}
              </option>
            ))}
          </select>
        </label>
        <label>
          Start command
          <input value={startCommand} onChange={(event) => setStartCommand(event.target.value)} required spellCheck={false} />
          <small>Runs inside the server's own folder on the node. Put the server files there before starting.</small>
        </label>
        <label>
          Stop command
          <input value={stopCommand} onChange={(event) => setStopCommand(event.target.value)} spellCheck={false} />
          <small>Typed into the console to shut down cleanly. Leave empty to end the process directly.</small>
        </label>
        {error && <p className="error" role="alert">{error}</p>}
        <button className="primary" disabled={busy}>
          Create server
        </button>
      </form>
    </>
  );
}

function Nodes({ nodes, onChanged }: { nodes: NodeInfo[]; onChanged: () => void }) {
  const [name, setName] = useState("");
  const [url, setUrl] = useState("http://127.0.0.1:8750");
  const [token, setToken] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.createNode(name, url, token);
      setName("");
      setToken("");
      onChanged();
    } catch (problem) {
      setError((problem as Error).message);
    }
    setBusy(false);
  }

  async function remove(id: number) {
    setError("");
    try {
      await api.deleteNode(id);
      onChanged();
    } catch (problem) {
      setError((problem as Error).message);
    }
  }

  return (
    <>
      <h1>Nodes</h1>
      <p className="dim">A node is a machine running the Consolry daemon. Servers run on nodes.</p>
      {nodes.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Address</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {nodes.map((node) => (
              <tr key={node.id}>
                <td>{node.name}</td>
                <td>
                  <code>{node.url}</code>
                </td>
                <td>
                  <span className={`dot ${node.online ? "running" : "crashed"}`} aria-hidden="true" />{" "}
                  {node.online ? `Online · ${node.os} · v${node.version}` : "Offline"}
                </td>
                <td>
                  <button onClick={() => remove(node.id)}>Remove</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h2>Add a node</h2>
      <form className="form" onSubmit={submit}>
        <label>
          Name
          <input value={name} onChange={(event) => setName(event.target.value)} required />
        </label>
        <label>
          Daemon address
          <input value={url} onChange={(event) => setUrl(event.target.value)} required spellCheck={false} />
        </label>
        <label>
          Daemon token
          <input value={token} onChange={(event) => setToken(event.target.value)} required spellCheck={false} autoComplete="off" />
          <small>
            The daemon prints this the first time it starts, and keeps it in the <code>token</code> file in its data folder.
          </small>
        </label>
        {error && <p className="error" role="alert">{error}</p>}
        <button className="primary" disabled={busy}>
          Add node
        </button>
      </form>
    </>
  );
}
