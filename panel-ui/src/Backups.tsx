import { useCallback, useEffect, useState } from "react";
import {
  api,
  formatBytes,
  formatTime,
  isLive,
  type Backup,
  type ServerInfo,
} from "./api";
import { CopyOffsite, OffsiteBackups } from "./Offsite";
import { Empty, ErrorNote, useAction } from "./ui";

export default function Backups({ server }: { server: ServerInfo }) {
  const [backups, setBackups] = useState<Backup[] | null>(null);
  const [confirming, setConfirming] = useState<{
    name: string;
    action: "restore" | "delete";
  } | null>(null);
  const [notice, setNotice] = useState("");
  const [offsiteReady, setOffsiteReady] = useState(false);
  const [offsiteKey, setOffsiteKey] = useState(0);
  const { error, busy, run, setError } = useAction();
  const live = isLive(server.state);

  const load = useCallback(() => {
    api
      .backups(server.id)
      .then(setBackups, (problem) => setError((problem as Error).message));
  }, [server.id, setError]);
  useEffect(load, [load]);
  useEffect(() => {
    api.offsite(server.id).then(
      (result) => setOffsiteReady(result.available),
      () => {},
    );
  }, [server.id]);

  function done(message: string) {
    setConfirming(null);
    setNotice(message);
    load();
  }

  return (
    <section className="backups">
      <header className="bar">
        <p className="grow dim">
          A backup is a copy of every file in the server's folder. One is taken
          automatically before plugins or the version change; the newest 5 of
          those are kept.
        </p>
        <button
          className="primary small"
          disabled={busy !== ""}
          onClick={() =>
            run("create", async () => {
              const backup = await api.createBackup(server.id);
              const skipped = backup.skipped?.length ?? 0;
              done(
                skipped
                  ? `Backup created. ${skipped} file${skipped === 1 ? " was" : "s were"} in use by the running server and left out: ${backup.skipped!.slice(0, 3).join(", ")}${skipped > 3 ? "…" : ""}`
                  : "Backup created.",
              );
            })
          }
        >
          {busy === "create" ? "Backing up…" : "Back up now"}
        </button>
      </header>
      {live && (
        <p className="note warn">
          The server is running. A backup taken now may catch the world
          mid-save; stop the server first for a clean copy.
        </p>
      )}
      {notice && <p className="note">{notice}</p>}
      <ErrorNote message={error} />

      {backups === null ? null : backups.length === 0 ? (
        <Empty title="No backups yet">
          Take one before changing plugins or updating the server.
        </Empty>
      ) : (
        <ul className="rows boxed">
          {backups.map((backup) => (
            <li key={backup.name}>
              <span className="grow">
                <strong>{formatTime(backup.created)}</strong>
                <small>
                  {formatBytes(backup.size)}
                  {backup.kind === "auto" &&
                    " · taken automatically before a change"}
                  {backup.kind === "scheduled" && " · taken by a schedule"}
                </small>
              </span>
              {confirming?.name === backup.name ? (
                <>
                  <span>
                    {confirming.action === "restore"
                      ? "Replace every file on the server with this backup?"
                      : "Delete this backup for good?"}
                  </span>
                  <button
                    className="danger small"
                    disabled={busy !== ""}
                    onClick={() =>
                      confirming.action === "restore"
                        ? run(
                            "restore",
                            () => api.restoreBackup(server.id, backup.name),
                            () => done("Backup restored."),
                          )
                        : run(
                            "delete",
                            () => api.deleteBackup(server.id, backup.name),
                            () => done("Backup deleted."),
                          )
                    }
                  >
                    {confirming.action === "restore"
                      ? busy === "restore"
                        ? "Restoring…"
                        : "Restore"
                      : "Delete"}
                  </button>
                  <button className="small" onClick={() => setConfirming(null)}>
                    Cancel
                  </button>
                </>
              ) : (
                <>
                  <button
                    className="small"
                    disabled={live}
                    title={
                      live ? "Stop the server before restoring" : undefined
                    }
                    onClick={() =>
                      setConfirming({ name: backup.name, action: "restore" })
                    }
                  >
                    Restore
                  </button>
                  <a
                    className="button small"
                    href={api.backupUrl(server.id, backup.name)}
                  >
                    Download
                  </a>
                  {offsiteReady && backup.kind !== "auto" && (
                    <CopyOffsite
                      server={server}
                      name={backup.name}
                      onCopied={(message) => {
                        setNotice(message);
                        setOffsiteKey((key) => key + 1);
                      }}
                    />
                  )}
                  <button
                    className="small"
                    onClick={() =>
                      setConfirming({ name: backup.name, action: "delete" })
                    }
                  >
                    Delete
                  </button>
                </>
              )}
            </li>
          ))}
        </ul>
      )}

      <OffsiteBackups key={offsiteKey} server={server} onRestored={load} />
    </section>
  );
}
