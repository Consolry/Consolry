import { useEffect, useState } from "react";
import { api, type UpdateStatus } from "./api";

/** Tells the admin when a newer Consolry is out, and installs it on request. */
export default function UpdateBanner() {
  const [status, setStatus] = useState<UpdateStatus | null>(null);
  const [stage, setStage] = useState<"idle" | "confirm" | "working">("idle");
  const [error, setError] = useState("");

  useEffect(() => {
    const check = () => api.update().then(setStatus, () => {});
    check();
    const timer = window.setInterval(check, 60 * 60 * 1000);
    return () => window.clearInterval(timer);
  }, []);

  if (!status?.available) return null;

  async function install() {
    setStage("working");
    setError("");
    try {
      await api.installUpdate();
    } catch (problem) {
      setError((problem as Error).message);
      setStage("idle");
      return;
    }
    // The panel goes away while it restarts. Wait for the new version to answer, then load it.
    const started = Date.now();
    const timer = window.setInterval(async () => {
      const state = await api.state().catch(() => null);
      if (state && state.version !== status!.current) {
        window.clearInterval(timer);
        location.reload();
      } else if (Date.now() - started > 3 * 60 * 1000) {
        window.clearInterval(timer);
        setError("The new version has not come back yet. Check that Consolry is running, then reload this page.");
        setStage("idle");
      }
    }, 2000);
  }

  return (
    <div className="note update" role="status">
      {stage === "working" ? (
        <p>
          <strong>Updating to {status.latest}…</strong> Running servers are being saved and stopped. They start again by themselves; this page reloads when
          Consolry is back.
        </p>
      ) : (
        <>
          <p>
            <strong>Consolry {status.latest} is available.</strong> You have {status.current}.{" "}
            {status.notes && (
              <a href={status.notes} target="_blank" rel="noreferrer">
                What's new
              </a>
            )}
          </p>
          {stage === "confirm" ? (
            <>
              <p className="dim">Running servers are saved and stopped for about a minute, then started again. Players are disconnected.</p>
              <span className="row-actions">
                <button className="small primary" onClick={install}>
                  Update and restart
                </button>
                <button className="small" onClick={() => setStage("idle")}>
                  Not now
                </button>
              </span>
            </>
          ) : (
            <button className="small primary" onClick={() => setStage("confirm")}>
              Update now
            </button>
          )}
        </>
      )}
      {error && <p className="error" role="alert">{error}</p>}
    </div>
  );
}
