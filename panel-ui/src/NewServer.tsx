import { useEffect, useState, type FormEvent } from "react";
import { api, type NodeInfo, type Software } from "./api";
import { Empty, ErrorNote } from "./ui";

const memoryChoices = [1, 2, 4, 6, 8, 12, 16];

export default function NewServer({ nodes, onCreated }: { nodes: NodeInfo[]; onCreated: (id: string, warning: string) => void }) {
  const [mode, setMode] = useState<"minecraft" | "custom">("minecraft");
  const [name, setName] = useState("");
  const [nodeId, setNodeId] = useState(nodes[0]?.id ?? 0);
  const [softwares, setSoftwares] = useState<Software[]>([]);
  const [software, setSoftware] = useState("paper");
  const [versions, setVersions] = useState<string[] | null>(null);
  const [version, setVersion] = useState("");
  const [memory, setMemory] = useState(2);
  const [eula, setEula] = useState(false);
  const [startCommand, setStartCommand] = useState("");
  const [stopCommand, setStopCommand] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

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
        setVersion(list[0] ?? "");
      },
      (problem) => !cancelled && setError((problem as Error).message),
    );
    return () => {
      cancelled = true;
    };
  }, [software]);

  if (nodes.length === 0) {
    return (
      <Empty title="Add a node first" action={<a className="button primary" href="#/nodes">Add a node</a>}>
        A server needs a node to run on.
      </Empty>
    );
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const target = nodeId || nodes[0].id;
      const result = await api.createServer(
        mode === "minecraft"
          ? { name, nodeId: target, minecraft: { software, version, memoryMb: memory * 1024, acceptEula: eula } }
          : { name, nodeId: target, startCommand, stopCommand },
      );
      onCreated(result.id, result.warning);
    } catch (problem) {
      setError((problem as Error).message);
      setBusy(false);
    }
  }

  return (
    <>
      <header className="page-head">
        <h1>New server</h1>
      </header>

      <div className="tabs" role="tablist" aria-label="Kind of server">
        <button role="tab" aria-selected={mode === "minecraft"} onClick={() => setMode("minecraft")}>
          Minecraft
        </button>
        <button role="tab" aria-selected={mode === "custom"} onClick={() => setMode("custom")}>
          Custom command
        </button>
      </div>

      <form className="card form" onSubmit={submit}>
        <label>
          Name
          <input value={name} onChange={(event) => setName(event.target.value)} required autoFocus />
        </label>
        {nodes.length > 1 && (
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
        )}

        {mode === "minecraft" ? (
          <>
            <fieldset>
              <legend>Server software</legend>
              <div className="choices">
                {softwares.map((item) => (
                  <label key={item.id} className={software === item.id ? "choice on" : "choice"}>
                    <input type="radio" name="software" checked={software === item.id} onChange={() => setSoftware(item.id)} />
                    <strong>{item.name}</strong>
                    <small>{item.about}</small>
                  </label>
                ))}
              </div>
            </fieldset>
            <div className="pair">
              <label>
                Minecraft version
                <select value={version} onChange={(event) => setVersion(event.target.value)} disabled={!versions}>
                  {versions ? versions.map((item) => <option key={item}>{item}</option>) : <option>Loading…</option>}
                </select>
              </label>
              <label>
                Memory
                <select value={memory} onChange={(event) => setMemory(Number(event.target.value))}>
                  {memoryChoices.map((gb) => (
                    <option key={gb} value={gb}>
                      {gb} GB
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <label className="check">
              <input type="checkbox" checked={eula} onChange={(event) => setEula(event.target.checked)} required />
              <span>
                I accept the{" "}
                <a href="https://aka.ms/MinecraftEULA" target="_blank" rel="noreferrer">
                  Minecraft EULA
                </a>
                . Minecraft servers won't start without it.
              </span>
            </label>
            <small>Consolry downloads the newest build for that version, checks it, and sets up the start command.</small>
          </>
        ) : (
          <>
            <label>
              Start command
              <input className="mono" value={startCommand} onChange={(event) => setStartCommand(event.target.value)} required spellCheck={false} />
              <small>Runs inside the server's own folder on the node. Upload its files in the Files tab after creating it.</small>
            </label>
            <label>
              Stop command
              <input className="mono" value={stopCommand} onChange={(event) => setStopCommand(event.target.value)} spellCheck={false} />
              <small>Typed into the console to shut down cleanly. Leave empty to end the process directly.</small>
            </label>
          </>
        )}

        <ErrorNote message={error} />
        <button className="primary" disabled={busy || (mode === "minecraft" && !version)}>
          {busy ? (mode === "minecraft" ? "Downloading the server…" : "Creating…") : "Create server"}
        </button>
      </form>
    </>
  );
}
