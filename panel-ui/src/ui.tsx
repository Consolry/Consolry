import { useCallback, useState, type ReactNode } from "react";

export function Empty({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <div className="empty-state">
      <div className="blocks" aria-hidden="true">
        <i />
        <i />
        <i />
      </div>
      <h2>{title}</h2>
      <p className="dim">{children}</p>
      {action}
    </div>
  );
}

/** Runs an action, remembering its error and whether it is still in progress. */
export function useAction() {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");

  const run = useCallback(async (label: string, action: () => Promise<unknown>, after?: () => void) => {
    setError("");
    setBusy(label);
    try {
      await action();
      after?.();
    } catch (problem) {
      setError((problem as Error).message);
    } finally {
      setBusy("");
    }
  }, []);

  return { error, busy, run, setError };
}

export function ErrorNote({ message }: { message: string }) {
  return message ? (
    <p className="error" role="alert">
      {message}
    </p>
  ) : null;
}
