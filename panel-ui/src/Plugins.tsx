import { useCallback, useEffect, useState, type FormEvent } from "react";
import { api, formatBytes, type PluginList, type Project, type ServerInfo } from "./api";
import { Empty, ErrorNote, useAction } from "./ui";

export default function Plugins({ server }: { server: ServerInfo }) {
  const [list, setList] = useState<PluginList | null>(null);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Project[] | null>(null);
  const [notice, setNotice] = useState("");
  const [confirming, setConfirming] = useState("");
  const { error, busy, run, setError } = useAction();
  const running = server.state === "running" || server.state === "stopping";
  const word = server.software === "fabric" ? "mod" : "plugin";

  const load = useCallback(() => {
    api.plugins(server.id).then(setList, (problem) => setError((problem as Error).message));
  }, [server.id, setError]);
  useEffect(load, [load]);

  function search(event?: FormEvent) {
    event?.preventDefault();
    run("search", async () => setResults(await api.searchPlugins(server.id, query)));
  }

  function install(project: Project) {
    setNotice("");
    run(`install ${project.projectId}`, async () => {
      const { installed } = await api.installPlugin(server.id, project.projectId);
      const extra = installed.length - 1;
      setNotice(
        `Installed ${project.title}` +
          (extra > 0 ? ` and ${extra} ${word}${extra === 1 ? "" : "s"} it needs` : "") +
          (running ? ". Restart the server to load it." : "."),
      );
      load();
    });
  }

  const installed = new Set(list?.items.map((item) => item.projectId).filter(Boolean));
  const updates = list?.items.filter((item) => item.update).length ?? 0;

  return (
    <section className="plugins">
      {notice && <p className="note">{notice}</p>}
      <ErrorNote message={error} />
      {list?.lookupFailed && <p className="note warn">Modrinth could not be reached, so installed files can't be identified right now.</p>}

      <div className="panel-block">
        <header className="bar">
          <h2 className="grow">
            Installed <span className="dim">{list ? list.items.length : ""}</span>
          </h2>
          {updates > 0 && <span className="state stopping">{updates} update{updates === 1 ? "" : "s"}</span>}
        </header>
        {list === null ? null : list.items.length === 0 ? (
          <p className="dim pad">
            Nothing in the <code>{list.folder}</code> folder yet. Search below to add something.
          </p>
        ) : (
          <ul className="rows">
            {list.items.map((item) => (
              <li key={item.file}>
                <Icon url={item.iconUrl} name={item.title ?? item.file} />
                <span className="grow">
                  <strong>{item.title ?? item.file}</strong>
                  <small>
                    {item.verified ? `${item.version} · ${item.file}` : "Not on Modrinth, so it can't be checked or updated here"} · {formatBytes(item.size)}
                  </small>
                </span>
                <span className={item.verified ? "state running" : "state"} title={item.verified ? "This file matches the one published on Modrinth" : undefined}>
                  {item.verified ? "Verified" : "Unverified"}
                </span>
                {confirming === item.file ? (
                  <>
                    <button className="danger small" onClick={() => run("remove", () => api.remove(server.id, `${list.folder}/${item.file}`), load)}>
                      Remove
                    </button>
                    <button className="small" onClick={() => setConfirming("")}>
                      Cancel
                    </button>
                  </>
                ) : (
                  <>
                    {item.update && (
                      <button className="primary small" disabled={busy !== ""} onClick={() => run("update", () => api.updatePlugin(server.id, item.file), load)}>
                        Update to {item.update}
                      </button>
                    )}
                    <button className="small" onClick={() => setConfirming(item.file)}>
                      Remove
                    </button>
                  </>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="panel-block">
        <form className="bar search" onSubmit={search}>
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={`Search Modrinth for ${word}s that run on ${server.software} ${server.mcVersion}`}
            aria-label={`Search for ${word}s`}
          />
          <button className="primary small" disabled={busy === "search"}>
            Search
          </button>
        </form>
        {results === null ? (
          <p className="dim pad">Results only include {word}s with a version for this server. Anything a {word} requires is installed with it.</p>
        ) : results.length === 0 ? (
          <Empty title="Nothing found">Try a different name. Only {word}s with a version for this server are shown.</Empty>
        ) : (
          <ul className="rows">
            {results.map((project) => (
              <li key={project.projectId}>
                <Icon url={project.iconUrl} name={project.title} />
                <span className="grow">
                  <strong>{project.title}</strong>
                  <small>{project.description}</small>
                  <small>
                    by {project.author} · {project.downloads.toLocaleString()} downloads
                  </small>
                </span>
                {installed.has(project.projectId) ? (
                  <span className="state running">Installed</span>
                ) : (
                  <button className="primary small" disabled={busy !== ""} onClick={() => install(project)}>
                    {busy === `install ${project.projectId}` ? "Installing…" : "Install"}
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

function Icon({ url, name }: { url?: string; name: string }) {
  return url ? <img className="plugin-icon" src={url} alt="" loading="lazy" /> : <span className="plugin-icon">{name[0]?.toUpperCase()}</span>;
}
