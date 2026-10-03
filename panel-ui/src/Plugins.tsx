import { useCallback, useEffect, useState, type FormEvent } from "react";
import { api, formatBytes, isLive, type PluginList, type PluginSearch, type PluginSource, type Project, type ServerInfo } from "./api";
import { Empty, ErrorNote, useAction } from "./ui";

const sourceNames: Record<PluginSource, string> = { modrinth: "Modrinth", hangar: "Hangar", curseforge: "CurseForge" };

function compact(count: number) {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(count >= 10_000_000 ? 0 : 1)}M`;
  if (count >= 1_000) return `${Math.round(count / 1_000)}K`;
  return String(count);
}

export default function Plugins({ server }: { server: ServerInfo }) {
  const [view, setView] = useState<"browse" | "installed">("browse");
  const [list, setList] = useState<PluginList | null>(null);
  const [typed, setTyped] = useState("");
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);
  const [source, setSource] = useState<PluginSource>("modrinth");
  const [results, setResults] = useState<PluginSearch | null>(null);
  const [admin, setAdmin] = useState(false);
  const [key, setKey] = useState("");
  const [notice, setNotice] = useState("");
  const [confirming, setConfirming] = useState("");
  const { error, busy, run, setError } = useAction();
  const running = isLive(server.state);
  const word = server.software === "fabric" ? "mod" : "plugin";

  const loadInstalled = useCallback(() => {
    api.plugins(server.id).then(setList, (problem) => setError((problem as Error).message));
  }, [server.id, setError]);
  useEffect(loadInstalled, [loadInstalled]);
  useEffect(() => {
    api.state().then((state) => setAdmin(Boolean(state.user?.admin)), () => {});
  }, []);

  // With nothing typed this shows the most downloaded; with a search it shows the best matches.
  useEffect(() => {
    let cancelled = false;
    setResults(null);
    api.searchPlugins(server.id, query, page, source).then(
      (result) => !cancelled && setResults(result),
      (problem) => !cancelled && setError((problem as Error).message),
    );
    return () => {
      cancelled = true;
    };
  }, [server.id, query, page, source, setError]);

  function search(event: FormEvent) {
    event.preventDefault();
    setPage(0);
    setQuery(typed.trim());
    setView("browse");
  }

  function install(project: Project) {
    setNotice("");
    run(`install ${project.projectId}`, async () => {
      const { installed } = await api.installPlugin(server.id, project);
      const extra = installed.length - 1;
      setNotice(
        `Installed ${project.title}` +
          (extra > 0 ? ` and ${extra} ${word}${extra === 1 ? "" : "s"} it needs` : "") +
          (running ? ". Restart the server to load it." : "."),
      );
      loadInstalled();
    });
  }

  const installed = new Set(list?.items.map((item) => item.projectId).filter(Boolean));
  const updates = list?.items.filter((item) => item.update).length ?? 0;
  const pages = results ? Math.max(1, Math.ceil(Math.min(results.total, 8000) / results.pageSize)) : 1;

  return (
    <section className="stack">
      <header className="bar">
        <div className="segmented" role="tablist" aria-label={`${word}s`}>
          <button role="tab" aria-selected={view === "browse"} onClick={() => setView("browse")}>
            Browse
          </button>
          <button role="tab" aria-selected={view === "installed"} onClick={() => setView("installed")}>
            Installed {list ? list.items.length : ""}
            {updates > 0 && <span className="badge">{updates}</span>}
          </button>
        </div>
        <form className="bar grow search-form" onSubmit={search}>
          <input
            type="search"
            value={typed}
            onChange={(event) => setTyped(event.target.value)}
            placeholder={`Search ${word}s for ${server.software} ${server.mcVersion}`}
            aria-label={`Search for ${word}s`}
          />
          <button className="primary small">Search</button>
        </form>
      </header>

      {notice && <p className="note">{notice}</p>}
      <ErrorNote message={error} />

      {view === "browse" && (
        <>
          <div className="segmented" role="tablist" aria-label="Where to search">
            {(results?.sources ?? ["modrinth"]).map((item) => (
              <button
                key={item}
                role="tab"
                aria-selected={source === item}
                onClick={() => {
                  setPage(0);
                  setSource(item);
                }}
              >
                {sourceNames[item]}
              </button>
            ))}
          </div>
          <p className="dim">
            {query ? `Results for "${query}"` : `Most downloaded ${word}s`} with a version for this server, from {sourceNames[source]}. Every download is checked
            against the fingerprint {sourceNames[source]} publishes. Anything a {word} requires is installed with it, and a backup is taken first.
          </p>
          {source === "curseforge" && results && !results.curseforgeReady ? (
            <div className="card form wide">
              <h2>Connect CurseForge</h2>
              <p className="dim">
                CurseForge only answers programs that have a key. Keys are free: sign in at{" "}
                <a href="https://console.curseforge.com/" target="_blank" rel="noreferrer">
                  console.curseforge.com
                </a>
                , open API keys, and copy yours.
              </p>
              {admin ? (
                <form
                  className="bar"
                  onSubmit={(event) => {
                    event.preventDefault();
                    run("key", () => api.setCurseForgeKey(key), () => {
                      setKey("");
                      setResults(null);
                      setPage(0);
                      setSource("modrinth");
                      setTimeout(() => setSource("curseforge"));
                    });
                  }}
                >
                  <input className="mono grow" value={key} onChange={(event) => setKey(event.target.value)} placeholder="Paste your CurseForge API key" required spellCheck={false} />
                  <button className="primary small" disabled={busy !== ""}>
                    Save key
                  </button>
                </form>
              ) : (
                <p>Ask the panel's admin to add one.</p>
              )}
            </div>
          ) : results === null ? (
            <ul className="grid" aria-busy="true">
              {Array.from({ length: 16 }, (_, index) => (
                <li key={index} className="tile loading" />
              ))}
            </ul>
          ) : results.projects.length === 0 ? (
            <Empty title="Nothing found">Try a different name. Only {word}s with a version for this server are shown.</Empty>
          ) : (
            <ul className="grid">
              {results.projects.map((project) => (
                <li key={`${project.source}-${project.projectId}`} className="tile">
                  <div className="tile-head">
                    <Icon url={project.iconUrl} name={project.title} />
                    <div>
                      <strong>{project.title}</strong>
                      <small>by {project.author}</small>
                    </div>
                  </div>
                  <p>{project.description}</p>
                  <div className="tile-foot">
                    <span className="dim">{compact(project.downloads)} downloads</span>
                    {installed.has(project.projectId) ? (
                      <span className="state running">Installed</span>
                    ) : (
                      <button className="primary small" disabled={busy !== ""} onClick={() => install(project)}>
                        {busy === `install ${project.projectId}` ? "Installing…" : "Install"}
                      </button>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
          {results && results.total > results.pageSize && (
            <nav className="bar pager" aria-label="Pages">
              <button className="small" disabled={page === 0} onClick={() => setPage(page - 1)}>
                Previous
              </button>
              <span className="dim">
                Page {page + 1} of {pages.toLocaleString()}
              </span>
              <button className="small" disabled={page + 1 >= pages} onClick={() => setPage(page + 1)}>
                Next
              </button>
            </nav>
          )}
        </>
      )}

      {view === "installed" && (
        <>
          {list?.lookupFailed && <p className="note warn">Modrinth could not be reached, so installed files can't be identified right now.</p>}
          {list === null ? null : list.items.length === 0 ? (
            <Empty title={`No ${word}s installed`} action={<button className="primary" onClick={() => setView("browse")}>Browse {word}s</button>}>
              Nothing is in the {list.folder} folder yet.
            </Empty>
          ) : (
            <ul className="grid">
              {list.items.map((item) => (
                <li key={item.file} className="tile">
                  <div className="tile-head">
                    <Icon url={item.iconUrl} name={item.title ?? item.file} />
                    <div>
                      <strong>{item.title ?? item.file}</strong>
                      <small>{item.verified ? `${item.version} · ${sourceNames[item.source ?? "modrinth"]}` : "Not recognised"}</small>
                    </div>
                  </div>
                  <p className="dim file">
                    {item.file} · {formatBytes(item.size)}
                  </p>
                  <div className="tile-foot">
                    <span
                      className={item.verified ? "state running" : "state"}
                      title={
                        item.verified
                          ? `This file matches the one published on ${sourceNames[item.source ?? "modrinth"]}`
                          : "This file can't be checked or updated here"
                      }
                    >
                      {item.verified ? "Verified" : "Unverified"}
                    </span>
                    {confirming === item.file ? (
                      <span className="bar">
                        <button className="danger small" onClick={() => run("remove", () => api.remove(server.id, `${list.folder}/${item.file}`), loadInstalled)}>
                          Remove
                        </button>
                        <button className="small" onClick={() => setConfirming("")}>
                          Cancel
                        </button>
                      </span>
                    ) : (
                      <span className="bar">
                        {item.update && (
                          <button className="primary small" disabled={busy !== ""} onClick={() => run("update", () => api.updatePlugin(server.id, item.file), loadInstalled)}>
                            Update
                          </button>
                        )}
                        <button className="small" onClick={() => setConfirming(item.file)}>
                          Remove
                        </button>
                      </span>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </section>
  );
}

function Icon({ url, name }: { url?: string; name: string }) {
  return url ? <img className="plugin-icon" src={url} alt="" loading="lazy" /> : <span className="plugin-icon">{name[0]?.toUpperCase()}</span>;
}
