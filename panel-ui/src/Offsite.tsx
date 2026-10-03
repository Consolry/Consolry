import { useCallback, useEffect, useState } from "react";
import {
  api,
  formatBytes,
  formatTime,
  isLive,
  type Offsite,
  type ServerInfo,
  type StorageSettings,
} from "./api";
import { ErrorNote, useAction } from "./ui";

/** The off-site part of a server's Backups tab. */
export function OffsiteBackups({
  server,
  onRestored,
}: {
  server: ServerInfo;
  onRestored: () => void;
}) {
  const [offsite, setOffsite] = useState<Offsite | null>(null);
  const [confirming, setConfirming] = useState("");
  const [notice, setNotice] = useState("");
  const { error, busy, run } = useAction();
  const live = isLive(server.state);

  const load = useCallback(
    () => api.offsite(server.id).then(setOffsite, () => {}),
    [server.id],
  );
  useEffect(() => {
    load();
  }, [load]);
  if (!offsite) return null;

  return (
    <section className="card form wide">
      <h2>Off-site copies</h2>
      {!offsite.available ? (
        <p className="dim">
          Keep copies of your backups somewhere other than this computer, so a
          broken disk does not take every copy with it. The panel's admin sets
          up storage on the Storage page first.
        </p>
      ) : (
        <>
          <label className="check">
            <input
              type="checkbox"
              checked={offsite.enabled}
              disabled={busy !== ""}
              onChange={(event) =>
                run("toggle", async () =>
                  setOffsite(
                    await api.setOffsite(server.id, {
                      enabled: event.target.checked,
                    }),
                  ),
                )
              }
            />
            <span>
              <strong>Copy every backup off-site</strong>
              <small>
                Backups taken by hand or by a schedule are copied as soon as
                they are made. The newest {offsite.keep} copies are kept.
              </small>
            </span>
          </label>
          {offsite.problem && <p className="note warn">{offsite.problem}</p>}
          {notice && <p className="note">{notice}</p>}
          <ErrorNote message={error} />
          {offsite.copies.length > 0 ? (
            <ul className="rows boxed">
              {offsite.copies.map((copy) => (
                <li key={copy.name}>
                  <span className="grow">
                    <strong>{formatTime(copy.created)}</strong>
                    <small>{formatBytes(copy.size)} · off-site</small>
                  </span>
                  {confirming === copy.name ? (
                    <>
                      <span>
                        Replace every file on the server with this copy?
                      </span>
                      <button
                        className="danger small"
                        disabled={busy !== ""}
                        onClick={() =>
                          run(
                            "restore",
                            () => api.restoreOffsite(server.id, copy.name),
                            () => {
                              setConfirming("");
                              setNotice(
                                "Restored from the off-site copy. It was also put back in the list of backups above.",
                              );
                              onRestored();
                            },
                          )
                        }
                      >
                        {busy === "restore" ? "Bringing it back…" : "Restore"}
                      </button>
                      <button
                        className="small"
                        onClick={() => setConfirming("")}
                      >
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
                        onClick={() => setConfirming(copy.name)}
                      >
                        Restore
                      </button>
                      <a
                        className="button small"
                        href={api.offsiteUrl(server.id, copy.name)}
                      >
                        Download
                      </a>
                    </>
                  )}
                </li>
              ))}
            </ul>
          ) : (
            <p className="dim">No off-site copies yet.</p>
          )}
        </>
      )}
    </section>
  );
}

/** "Copy off-site" for one local backup, shown when off-site storage is set up. */
export function CopyOffsite({
  server,
  name,
  onCopied,
}: {
  server: ServerInfo;
  name: string;
  onCopied: (message: string) => void;
}) {
  const { busy, run, error } = useAction();
  return (
    <>
      <button
        className="small"
        disabled={busy !== ""}
        onClick={() =>
          run(
            "copy",
            () => api.setOffsite(server.id, { copyNow: name }),
            () => onCopied("Copied off-site."),
          )
        }
      >
        {busy ? "Copying…" : "Copy off-site"}
      </button>
      {error && <span className="error">{error}</span>}
    </>
  );
}

/** The admin's Storage page: where off-site copies go. */
export default function StoragePage() {
  const [storage, setStorage] = useState<StorageSettings | null>(null);
  const [secretSet, setSecretSet] = useState(false);
  const [saved, setSaved] = useState(false);
  const { error, busy, run } = useAction();

  useEffect(() => {
    api.storage().then(
      (result) => {
        setStorage({ ...result.storage, secret: "" });
        setSecretSet(result.secretSet);
      },
      () => {},
    );
  }, []);
  if (!storage) return null;
  const field = (key: keyof StorageSettings) => ({
    value: String(storage[key] ?? ""),
    onChange: (event: { target: { value: string } }) =>
      setStorage({
        ...storage,
        [key]: key === "keep" ? Number(event.target.value) : event.target.value,
      }),
  });

  return (
    <>
      <header className="page-head">
        <h1>Storage</h1>
      </header>
      <p className="dim lead">
        Somewhere other than this computer to keep copies of backups. Anything
        that works like Amazon S3 will do: Backblaze B2, Cloudflare R2, Wasabi,
        Amazon S3 or your own MinIO. Each server's owner then switches copying
        on in its Backups tab.
      </p>
      <form
        className="card form wide"
        onSubmit={(event) => {
          event.preventDefault();
          setSaved(false);
          run("save", async () => {
            const result = await api.setStorage(storage);
            setStorage({ ...result.storage, secret: "" });
            setSecretSet(result.secretSet);
            setSaved(true);
          });
        }}
      >
        <h2>S3-compatible storage</h2>
        <label>
          Address
          <input
            className="mono"
            {...field("endpoint")}
            placeholder="https://s3.us-west-004.backblazeb2.com"
            spellCheck={false}
          />
          <small>
            Your provider shows this as the "endpoint" or "S3 API" address.
          </small>
        </label>
        <div className="pair">
          <label>
            Bucket
            <input {...field("bucket")} spellCheck={false} />
          </label>
          <label>
            Region
            <input
              {...field("region")}
              placeholder="Leave empty if unsure"
              spellCheck={false}
            />
          </label>
        </div>
        <div className="pair">
          <label>
            Key ID
            <input
              className="mono"
              {...field("accessKey")}
              autoComplete="off"
              spellCheck={false}
            />
          </label>
          <label>
            Secret key
            <input
              className="mono"
              type="password"
              {...field("secret")}
              placeholder={secretSet ? "Saved. Type to change it" : ""}
              autoComplete="new-password"
            />
          </label>
        </div>
        <div className="pair">
          <label>
            Folder in the bucket
            <input
              {...field("folder")}
              placeholder="consolry"
              spellCheck={false}
            />
          </label>
          <label>
            Copies kept per server
            <input type="number" min={1} max={365} {...field("keep")} />
          </label>
        </div>
        <ErrorNote message={error} />
        {saved && (
          <p className="note">
            Saved. Consolry wrote and removed a small test file, so the details
            work.
          </p>
        )}
        <button className="primary" disabled={busy !== ""}>
          {busy ? "Checking…" : "Save and check"}
        </button>
      </form>
    </>
  );
}
