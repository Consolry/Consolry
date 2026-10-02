import { useCallback, useEffect, useRef, useState } from "react";
import { api, formatBytes, formatTime, type FileEntry } from "./api";
import { Empty, ErrorNote, useAction } from "./ui";

const maxEditable = 1024 * 1024;
const binary = /\.(jar|zip|gz|tar|7z|rar|png|jpe?g|gif|webp|ico|dat|dat_old|mca|mcr|nbt|db|sqlite|lock|class|exe|dll|so|bin|ogg|mp3|wav)$/i;

const join = (folder: string, name: string) => (folder ? `${folder}/${name}` : name);

export default function Files({ serverId }: { serverId: string }) {
  const [folder, setFolder] = useState("");
  const [entries, setEntries] = useState<FileEntry[] | null>(null);
  const [editing, setEditing] = useState<{ path: string; content: string; saved: string } | null>(null);
  const [confirming, setConfirming] = useState("");
  const upload = useRef<HTMLInputElement>(null);
  const { error, busy, run, setError } = useAction();

  const load = useCallback(() => {
    api.files(serverId, folder).then(setEntries, (problem) => {
      setEntries([]);
      setError((problem as Error).message);
    });
  }, [serverId, folder, setError]);

  useEffect(() => {
    setEntries(null);
    setConfirming("");
    load();
  }, [load]);

  const crumbs = folder ? folder.split("/") : [];

  function open(entry: FileEntry) {
    const path = join(folder, entry.name);
    if (entry.dir) {
      setFolder(path);
    } else if (entry.size > maxEditable || binary.test(entry.name)) {
      location.href = api.fileUrl(serverId, path);
    } else {
      run("open", async () => {
        const content = await api.readFile(serverId, path);
        setEditing({ path, content, saved: content });
      });
    }
  }

  function create(kind: "file" | "folder") {
    const name = prompt(kind === "file" ? "Name of the new file" : "Name of the new folder")?.trim();
    if (!name) return;
    const path = join(folder, name);
    run("create", () => (kind === "file" ? api.writeFile(serverId, path, "") : api.makeFolder(serverId, path)), load);
  }

  function rename(entry: FileEntry) {
    const name = prompt("New name", entry.name)?.trim();
    if (!name || name === entry.name) return;
    run("rename", () => api.rename(serverId, join(folder, entry.name), join(folder, name)), load);
  }

  function uploadFiles(files: FileList | null) {
    if (!files?.length) return;
    const list = Array.from(files);
    run(
      `Uploading ${list.length === 1 ? list[0].name : `${list.length} files`}…`,
      async () => {
        for (const file of list) await api.writeFile(serverId, join(folder, file.name), file);
      },
      load,
    );
    if (upload.current) upload.current.value = "";
  }

  if (editing) {
    const dirty = editing.content !== editing.saved;
    return (
      <section className="editor">
        <header className="bar">
          <code className="grow">{editing.path}</code>
          {dirty && <span className="dim">Unsaved changes</span>}
          <button
            className="primary small"
            disabled={!dirty || busy !== ""}
            onClick={() => run("save", () => api.writeFile(serverId, editing.path, editing.content), () => setEditing({ ...editing, saved: editing.content }))}
          >
            Save
          </button>
          <button
            className="small"
            onClick={() => {
              if (!dirty || confirm("Close without saving your changes?")) {
                setEditing(null);
                load();
              }
            }}
          >
            Close
          </button>
        </header>
        <ErrorNote message={error} />
        <textarea
          value={editing.content}
          onChange={(event) => setEditing({ ...editing, content: event.target.value })}
          spellCheck={false}
          aria-label={`Contents of ${editing.path}`}
        />
      </section>
    );
  }

  return (
    <section className="files">
      <header className="bar">
        <nav className="path grow" aria-label="Folder">
          <button className="plain" onClick={() => setFolder("")}>
            server
          </button>
          {crumbs.map((part, index) => (
            <span key={index}>
              <span aria-hidden="true">/</span>
              <button className="plain" onClick={() => setFolder(crumbs.slice(0, index + 1).join("/"))}>
                {part}
              </button>
            </span>
          ))}
        </nav>
        <input ref={upload} type="file" multiple hidden onChange={(event) => uploadFiles(event.target.files)} />
        <button className="primary small" disabled={busy !== ""} onClick={() => upload.current?.click()}>
          Upload
        </button>
        <button className="small" onClick={() => create("file")}>
          New file
        </button>
        <button className="small" onClick={() => create("folder")}>
          New folder
        </button>
      </header>

      {busy.startsWith("Uploading") && <p className="note">{busy}</p>}
      <ErrorNote message={error} />

      {entries === null ? null : entries.length === 0 ? (
        <Empty title="This folder is empty">Upload files, or create one here.</Empty>
      ) : (
        <table className="list">
          <thead>
            <tr>
              <th>Name</th>
              <th>Size</th>
              <th>Changed</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {entries.map((entry) => {
              const path = join(folder, entry.name);
              return (
                <tr key={entry.name}>
                  <td>
                    <button className="plain name" onClick={() => open(entry)}>
                      <span className={entry.dir ? "icon folder" : "icon"} aria-hidden="true" />
                      {entry.name}
                    </button>
                  </td>
                  <td className="dim">{entry.dir ? "" : formatBytes(entry.size)}</td>
                  <td className="dim">{formatTime(entry.modified)}</td>
                  <td className="row-actions">
                    {confirming === entry.name ? (
                      <>
                        <span>Delete{entry.dir ? " this folder and everything in it" : ""}?</span>
                        <button className="danger small" onClick={() => run("delete", () => api.remove(serverId, path), load)}>
                          Delete
                        </button>
                        <button className="small" onClick={() => setConfirming("")}>
                          Cancel
                        </button>
                      </>
                    ) : (
                      <>
                        {!entry.dir && (
                          <a className="button small" href={api.fileUrl(serverId, path)}>
                            Download
                          </a>
                        )}
                        <button className="small" onClick={() => rename(entry)}>
                          Rename
                        </button>
                        <button className="small" onClick={() => setConfirming(entry.name)}>
                          Delete
                        </button>
                      </>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </section>
  );
}
