import { useState, type FormEvent } from "react";
import { api, type NodeInfo, type PteroServer } from "./api";
import { ErrorNote, useAction } from "./ui";

/** Copies servers from a Pterodactyl panel, one at a time. */
export default function Import({
  nodes,
  onImported,
}: {
  nodes: NodeInfo[];
  onImported: (id: string) => void;
}) {
  const [address, setAddress] = useState("");
  const [key, setKey] = useState("");
  const [servers, setServers] = useState<PteroServer[] | null>(null);
  const [nodeId, setNodeId] = useState(nodes[0]?.id ?? 0);
  const [result, setResult] = useState<{
    id: string;
    name: string;
    notes: string[];
  } | null>(null);
  const { error, busy, run } = useAction();

  function connect(event: FormEvent) {
    event.preventDefault();
    run("connect", async () =>
      setServers(await api.pteroServers(address, key)),
    );
  }

  return (
    <>
      <header className="page-head">
        <h1>Import from Pterodactyl</h1>
      </header>
      <p className="dim lead">
        Copy a server, with its world, plugins and settings, from a Pterodactyl
        panel. Nothing on Pterodactyl is changed or removed, so you can check
        the copy here before switching over.
      </p>

      <form className="card form wide" onSubmit={connect}>
        <h2>1. Connect</h2>
        <label>
          Pterodactyl address
          <input
            value={address}
            onChange={(event) => setAddress(event.target.value)}
            placeholder="https://panel.example.com"
            required
            spellCheck={false}
          />
        </label>
        <label>
          API key
          <input
            className="mono"
            type="password"
            value={key}
            onChange={(event) => setKey(event.target.value)}
            placeholder="ptlc_…"
            required
            autoComplete="off"
          />
          <small>
            In Pterodactyl, open Account, then API Credentials, and create a
            key. It is only used for this import and is not saved.
          </small>
        </label>
        <ErrorNote message={busy === "" || busy === "connect" ? error : ""} />
        <button className="primary" disabled={busy !== ""}>
          {busy === "connect" ? "Connecting…" : "Show my servers"}
        </button>
      </form>

      {servers && (
        <section className="card form wide">
          <h2>2. Choose a server</h2>
          <p className="note warn">
            Stop the server on Pterodactyl first, so its world is saved and not
            changing while it is copied.
          </p>
          {nodes.length > 1 && (
            <label>
              Copy it to
              <select
                value={nodeId}
                onChange={(event) => setNodeId(Number(event.target.value))}
              >
                {nodes.map((node) => (
                  <option key={node.id} value={node.id}>
                    {node.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          {result && (
            <div className="note">
              <p>
                <strong>{result.name} was copied.</strong>{" "}
                <a href={`#/servers/${result.id}`}>Open it</a>
              </p>
              {result.notes.map((note) => (
                <p key={note} className="dim">
                  {note}
                </p>
              ))}
            </div>
          )}
          {busy.startsWith("import") && (
            <p className="dim">
              Packing and copying the files. A big world can take several
              minutes; keep this page open.
            </p>
          )}
          <ErrorNote
            message={busy.startsWith("import") || busy === "" ? error : ""}
          />
          {servers.length === 0 ? (
            <p className="dim">That account has no servers.</p>
          ) : (
            <ul className="rows boxed">
              {servers.map((server) => (
                <li key={server.identifier}>
                  <span className="grow">
                    <strong>{server.name}</strong>
                    <small>
                      {server.egg || "Unknown egg"} ·{" "}
                      {server.memoryMb
                        ? `${Math.round(server.memoryMb / 102.4) / 10} GB`
                        : "no memory limit"}
                      {server.software
                        ? ""
                        : " · not a server Consolry recognises"}
                    </small>
                  </span>
                  <button
                    className="small primary"
                    disabled={busy !== ""}
                    onClick={() =>
                      run(`import ${server.identifier}`, async () => {
                        setResult(null);
                        const imported = await api.pteroImport(
                          address,
                          key,
                          server.identifier,
                          nodeId,
                        );
                        setResult({
                          id: imported.id,
                          name: server.name,
                          notes: imported.notes,
                        });
                        onImported(imported.id);
                      })
                    }
                  >
                    {busy === `import ${server.identifier}`
                      ? "Copying…"
                      : "Import"}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
    </>
  );
}
