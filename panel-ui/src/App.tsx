import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import Access from "./Access";
import Accounts from "./Accounts";
import { api, isLive, stateLabel, uptime, type NodeInfo, type PanelState, type ServerInfo, type User } from "./api";
import NewServer from "./NewServer";
import type { UsageHistory } from "./Dashboard";
import ServerPage, { type Tab } from "./ServerPage";
import { Empty, ErrorNote, useAction } from "./ui";
import UpdateBanner from "./Update";

type View = { kind: "overview" } | { kind: "server"; id: string; tab: Tab } | { kind: "new-server" } | { kind: "nodes" } | { kind: "access" } | { kind: "accounts" };

// The address after # decides the page, so a refresh or a shared link lands in the same place.
function parseHash(hash: string): View {
  const parts = hash.replace(/^#\/?/, "").split("/");
  if (parts[0] === "servers" && parts[1]) return { kind: "server", id: parts[1], tab: (parts[2] || "dashboard") as Tab };
  if (parts[0] === "new") return { kind: "new-server" };
  if (parts[0] === "nodes") return { kind: "nodes" };
  if (parts[0] === "access") return { kind: "access" };
  if (parts[0] === "accounts") return { kind: "accounts" };
  return { kind: "overview" };
}

function useView() {
  const [view, setView] = useState(() => parseHash(location.hash));
  useEffect(() => {
    const onChange = () => setView(parseHash(location.hash));
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return view;
}

function useNow() {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  return now;
}

export default function App() {
  const [state, setState] = useState<PanelState | null>(null);
  const [failed, setFailed] = useState(false);

  const refresh = useCallback(() => {
    api.state().then(setState, () => setFailed(true));
  }, []);
  useEffect(refresh, [refresh]);

  if (failed) {
    return (
      <Centered title="Can't reach the panel">
        <p className="dim">Check that the Consolry panel is running, then reload.</p>
      </Centered>
    );
  }
  if (!state) return null;
  if (state.setupNeeded) return <AuthForm mode="setup" onDone={refresh} />;
  if (!state.user) return <SignedOut signupAllowed={state.signupAllowed} onDone={refresh} />;
  return <Panel user={state.user} version={state.version} onSignOut={() => api.logout().then(refresh)} />;
}

function Logo() {
  return (
    <span className="logo">
      <svg viewBox="0 0 32 32" aria-hidden="true" shapeRendering="crispEdges">
        <path className="logo-block" d="M4 0h24v4h4v24h-4v4H4v-4H0V4h4z" />
        <path className="logo-mark" d="M8 10h4v4h4v4h-4v4H8v-4h4v-4H8zM18 18h8v4h-8z" />
      </svg>
      Consolry
    </span>
  );
}

function Centered({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="centered">
      <div className="stars" aria-hidden="true" />
      <div className="card auth">
        <Logo />
        <h1>{title}</h1>
        {children}
      </div>
    </main>
  );
}

/** The sign-in page and the sign-up page, at #/login and #/signup. */
function SignedOut({ signupAllowed, onDone }: { signupAllowed: boolean; onDone: () => void }) {
  const [hash, setHash] = useState(location.hash);
  useEffect(() => {
    const onChange = () => setHash(location.hash);
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  const signup = signupAllowed && hash === "#/signup";
  const done = () => {
    location.hash = "#/";
    onDone();
  };
  return (
    <AuthForm key={signup ? "signup" : "login"} mode={signup ? "signup" : "login"} onDone={done}>
      {signupAllowed &&
        (signup ? (
          <p className="dim switch">
            Already have an account? <a href="#/login">Sign in</a>
          </p>
        ) : (
          <p className="dim switch">
            New here? <a href="#/signup">Create an account</a>
          </p>
        ))}
    </AuthForm>
  );
}

function AuthForm({ mode, onDone, children }: { mode: "setup" | "login" | "signup"; onDone: () => void; children?: ReactNode }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const setup = mode === "setup";
  const creating = mode !== "login";

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await (setup ? api.setup(username, password) : mode === "signup" ? api.signup(username, password) : api.login(username, password));
      onDone();
    } catch (problem) {
      setError((problem as Error).message);
      setBusy(false);
    }
  }

  return (
    <Centered title={setup ? "Create the admin account" : mode === "signup" ? "Create an account" : "Sign in"}>
      {setup && <p className="dim">This is the first run. The account you create here manages the whole panel.</p>}
      {mode === "signup" && (
        <p className="dim">After this, tell the server's owner your username so they can share the server with you.</p>
      )}
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
            autoComplete={creating ? "new-password" : "current-password"}
            minLength={creating ? 10 : undefined}
            required
          />
          {creating && <small>At least 10 characters.</small>}
        </label>
        {error && <p className="error" role="alert">{error}</p>}
        <button className="primary" disabled={busy}>
          {creating ? "Create account" : "Sign in"}
        </button>
      </form>
      {children}
    </Centered>
  );
}

function Panel({ user, version, onSignOut }: { user: User; version: string; onSignOut: () => void }) {
  const { username, admin } = user;
  const [servers, setServers] = useState<ServerInfo[] | null>(null);
  const [nodes, setNodes] = useState<NodeInfo[] | null>(null);
  const [warning, setWarning] = useState("");
  const view = useView();
  const now = useNow();

  // A few minutes of CPU and memory readings per server, kept here so they survive switching tabs.
  const history = useRef(new Map<string, UsageHistory>());

  const loadServers = useCallback(
    () =>
      api.servers().then(
        (list) => {
          for (const server of list) {
            const samples = history.current.get(server.id) ?? { cpu: [], memory: [] };
            if (server.state === "running") {
              samples.cpu = [...samples.cpu, server.cpu].slice(-90);
              samples.memory = [...samples.memory, server.memory].slice(-90);
            } else {
              samples.cpu = [];
              samples.memory = [];
            }
            history.current.set(server.id, samples);
          }
          setServers(list);
        },
        () => {},
      ),
    [],
  );
  // Only the admin manages nodes; everyone else just sees the servers shared with them.
  const loadNodes = useCallback(() => (admin ? api.nodes().then(setNodes, () => {}) : setNodes([])), [admin]);

  useEffect(() => {
    loadServers();
    loadNodes();
    const fast = window.setInterval(loadServers, 2000);
    const slow = window.setInterval(loadNodes, 10000);
    return () => {
      window.clearInterval(fast);
      window.clearInterval(slow);
    };
  }, [loadServers, loadNodes]);

  if (!servers || !nodes) return null;

  // The admin can always create servers; anyone else only up to the number the admin allowed them.
  const owned = servers.filter((server) => server.owner).length;
  const canCreate = admin || owned < user.serverLimit;

  const selected = view.kind === "server" ? servers.find((server) => server.id === view.id) : undefined;
  const crumb =
    view.kind === "server" ? (selected?.name ?? "Unknown server") : view.kind === "new-server" ? "New server" : view.kind === "nodes" ? "Nodes" : view.kind === "access" ? "Remote access" : view.kind === "accounts" ? "Accounts" : "Overview";

  return (
    <div className="shell">
      <header className="topbar">
        <a href="#/" className="logo-link" aria-label="Consolry overview">
          <Logo />
        </a>
        <p className="crumbs">
          {view.kind === "server" && <a href="#/">Servers</a>}
          {view.kind === "server" && <span aria-hidden="true">/</span>}
          <strong>{crumb}</strong>
        </p>
        <span className="chip">v{version}</span>
        <span className="who">{username}</span>
        <button className="small" onClick={onSignOut}>
          Sign out
        </button>
      </header>

      <aside className="sidebar">
        <nav aria-label="Main">
          <a href="#/" className={view.kind === "overview" ? "on" : undefined}>
            Overview
          </a>
          {admin && (
            <a href="#/nodes" className={view.kind === "nodes" ? "on" : undefined}>
              Nodes <span className="count">{nodes.length}</span>
            </a>
          )}
          {admin && (
            <a href="#/accounts" className={view.kind === "accounts" ? "on" : undefined}>
              Accounts
            </a>
          )}
          {admin && (
            <a href="#/access" className={view.kind === "access" ? "on" : undefined}>
              Remote access
            </a>
          )}
        </nav>
        <p className="pixel heading">Servers</p>
        <nav aria-label="Servers">
          {servers.map((server) => (
            <a key={server.id} href={`#/servers/${server.id}`} className={selected?.id === server.id ? "on" : undefined}>
              <span className={`dot ${server.state}`} aria-hidden="true" />
              <span className="grow">
                {server.name}
                <small>{stateLabel(server.state)}</small>
              </span>
            </a>
          ))}
          {servers.length === 0 && <p className="dim empty">None yet.</p>}
        </nav>
        {canCreate && (
          <a href="#/new" className={view.kind === "new-server" ? "add on" : "add"}>
            + New server
          </a>
        )}
      </aside>

      <main className="content">
        {admin && <UpdateBanner />}
        {warning && (
          <p className="note warn">
            {warning}{" "}
            <button className="small" onClick={() => setWarning("")}>
              Dismiss
            </button>
          </p>
        )}
        {view.kind === "overview" && <Overview servers={servers} nodes={nodes} now={now} user={user} canCreate={canCreate} onChanged={loadServers} />}
        {view.kind === "server" && selected && (
          <ServerPage
            key={selected.id}
            server={selected}
            tab={view.tab}
            now={now}
            history={history.current.get(selected.id)}
            onChanged={loadServers}
            onDeleted={() => {
              location.hash = "#/";
              loadServers();
            }}
          />
        )}
        {view.kind === "server" && !selected && (
          <Empty title="That server doesn't exist" action={<a className="button" href="#/">Back to overview</a>}>
            It may have been removed.
          </Empty>
        )}
        {view.kind === "new-server" && !canCreate && (
          <Empty title="You can't create a server" action={<a className="button" href="#/">Back to overview</a>}>
            {user.serverLimit > 0 ? `Your account may create ${user.serverLimit}, and you already have that many.` : "Ask the panel's admin to allow it for your account."}
          </Empty>
        )}
        {view.kind === "new-server" && canCreate && (
          <NewServer
            nodes={nodes}
            user={user}
            onCreated={(id, note) => {
              setWarning(note);
              loadServers().then(() => {
                location.hash = `#/servers/${id}`;
              });
            }}
          />
        )}
        {view.kind === "nodes" && admin && <Nodes nodes={nodes} onChanged={loadNodes} />}
        {view.kind === "access" && admin && <Access />}
        {view.kind === "accounts" && admin && <Accounts />}
      </main>
    </div>
  );
}

function Overview({
  servers,
  nodes,
  now,
  user,
  canCreate,
  onChanged,
}: {
  servers: ServerInfo[];
  nodes: NodeInfo[];
  now: number;
  user: User;
  canCreate: boolean;
  onChanged: () => void;
}) {
  const admin = user.admin;
  const { error, run } = useAction();
  const power = (id: string, action: "start" | "stop") => run(action, () => api.power(id, action), onChanged);
  const running = servers.filter((server) => server.state === "running").length;
  const trouble = servers.filter((server) => server.state === "crashed" || server.state === "unreachable").length;
  const online = nodes.filter((node) => node.online).length;

  if (!admin && servers.length === 0 && canCreate) {
    return (
      <Empty title="No servers yet" action={<a className="button primary" href="#/new">Create a server</a>}>
        Your account may create {user.serverLimit === 1 ? "one server" : `${user.serverLimit} servers`} of its own.
      </Empty>
    );
  }

  if (!admin && servers.length === 0) {
    return (
      <Empty title="No servers shared with you yet">
        Tell the server's owner your username, <strong>{user.username}</strong>, and ask them to invite you from the server's Users tab.
      </Empty>
    );
  }

  if (admin && nodes.length === 0) {
    return (
      <Empty title="Connect your first node" action={<a className="button primary" href="#/nodes">Add a node</a>}>
        A node is a machine running the Consolry daemon. Servers run on nodes, so add one to get started.
      </Empty>
    );
  }

  return (
    <>
      <header className="page-head">
        <h1>Overview</h1>
        {canCreate && (
          <a className="button primary" href="#/new">
            New server
          </a>
        )}
      </header>

      <dl className="tiles">
        <div>
          <dt>Running</dt>
          <dd>
            {running} <small>of {servers.length}</small>
          </dd>
        </div>
        <div className={trouble ? "alert" : undefined}>
          <dt>Need attention</dt>
          <dd>{trouble}</dd>
        </div>
        {admin && (
          <div>
            <dt>Nodes online</dt>
            <dd>
              {online} <small>of {nodes.length}</small>
            </dd>
          </div>
        )}
      </dl>

      <ErrorNote message={error} />

      {servers.length === 0 ? (
        <Empty title="No servers yet" action={<a className="button primary" href="#/new">Create a server</a>}>
          Create one, put its files in its folder on the node, and start it from here.
        </Empty>
      ) : (
        <ul className="servers">
          {servers.map((server) => {
            const live = isLive(server.state);
            return (
              <li key={server.id} className={`server ${server.state}`}>
                <a href={`#/servers/${server.id}`} className="server-main">
                  <span className="server-name">{server.name}</span>
                  <span className={`state ${server.state}`}>{stateLabel(server.state)}</span>
                  <code>{server.kind === "minecraft" ? `${server.software} ${server.mcVersion}` : [server.command, ...server.args].join(" ") || "–"}</code>
                </a>
                <dl>
                  <div>
                    <dt>Node</dt>
                    <dd>{server.nodeName}</dd>
                  </div>
                  <div>
                    <dt>Uptime</dt>
                    <dd>{uptime(server.startedAt, now)}</dd>
                  </div>
                </dl>
                <div className="server-actions">
                  {!server.permissions.includes("power") ? null : live ? (
                    <button disabled={server.state === "stopping"} onClick={() => power(server.id, "stop")}>
                      Stop
                    </button>
                  ) : (
                    <button className="primary" disabled={server.state === "unreachable"} onClick={() => power(server.id, "start")}>
                      Start
                    </button>
                  )}
                  <a className="button" href={`#/servers/${server.id}`}>
                    Open
                  </a>
                </div>
              </li>
            );
          })}
        </ul>
      )}
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
      <header className="page-head">
        <h1>Nodes</h1>
      </header>
      <p className="dim lead">
        A node is a machine that runs servers. This machine is added for you. Add another only if you run the Consolry daemon on a second machine.
      </p>

      {nodes.length > 0 && (
        <ul className="nodes">
          {nodes.map((node) => (
            <li key={node.id}>
              <span className={`dot ${node.online ? "running" : "crashed"}`} aria-hidden="true" />
              <span className="grow">
                <strong>{node.name}</strong>
                <code>{node.url}</code>
              </span>
              <span className="dim">{node.online ? `${node.os} · v${node.version}` : "Offline"}</span>
              <button className="small" onClick={() => remove(node.id)}>
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}

      <form className="card form" onSubmit={submit}>
        <h2>Add another machine</h2>
        <label>
          Name
          <input value={name} onChange={(event) => setName(event.target.value)} required />
        </label>
        <label>
          Daemon address
          <input className="mono" value={url} onChange={(event) => setUrl(event.target.value)} required spellCheck={false} />
        </label>
        <label>
          Daemon token
          <input className="mono" value={token} onChange={(event) => setToken(event.target.value)} required spellCheck={false} autoComplete="off" />
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
