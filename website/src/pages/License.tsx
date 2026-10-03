import { useEffect, useState, type FormEvent } from "react";
import { PageHead } from "../components/Layout";

/** Shows the licence key after a purchase, and opens Stripe's page for managing a subscription. */
export default function LicensePage() {
  const [key, setKey] = useState("");
  const [status, setStatus] = useState<"idle" | "loading" | "ready" | "failed">("idle");
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  const [manageKey, setManageKey] = useState("");

  useEffect(() => {
    const session = new URLSearchParams(location.search).get("session_id");
    if (!session) return;
    setStatus("loading");
    fetch(`/api/license?session_id=${encodeURIComponent(session)}`)
      .then(async (response) => {
        const data = await response.json();
        if (!response.ok) throw new Error(data.error ?? "Something went wrong.");
        setKey(data.key);
        setStatus("ready");
      })
      .catch((problem: Error) => {
        setError(problem.message);
        setStatus("failed");
      });
  }, []);

  async function manage(event: FormEvent) {
    event.preventDefault();
    setError("");
    try {
      const response = await fetch("/api/portal", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ key: manageKey }) });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error ?? "Something went wrong.");
      location.href = data.url;
    } catch (problem) {
      setError((problem as Error).message);
    }
  }

  return (
    <>
      <PageHead label="Licence" title={status === "ready" ? "Thank you" : "Your licence"} intro="A licence key turns on a paid plan in your own Consolry panel." />
      <section className="section alt">
        <div className="wrap licence">
          {status === "loading" && <p>Getting your licence key…</p>}
          {status === "failed" && <p className="warning">{error} If you were charged, email us and we will sort it out.</p>}
          {status === "ready" && (
            <>
              <p>Your payment went through. This is your licence key:</p>
              <textarea readOnly value={key} rows={4} onFocus={(event) => event.target.select()} aria-label="Licence key" />
              <button
                type="button"
                className="button"
                onClick={() => navigator.clipboard.writeText(key).then(() => setCopied(true), () => {})}
              >
                {copied ? "Copied" : "Copy the key"}
              </button>
              <ol>
                <li>Open your Consolry panel and sign in as the admin.</li>
                <li>
                  Choose <strong>Licence</strong> in the menu, paste the key and press <strong>Save</strong>.
                </li>
              </ol>
              <p>
                Keep the key somewhere safe; you also need it to manage your subscription below. The panel renews it by itself each month while the
                subscription is paid.
              </p>
            </>
          )}
          <form className="licence-manage" onSubmit={manage}>
            <h2>Manage your subscription</h2>
            <p>Change your card, download invoices or cancel. Paste your licence key to open Stripe's billing page.</p>
            <textarea value={manageKey} onChange={(event) => setManageKey(event.target.value)} rows={3} required aria-label="Your licence key" />
            <button className="button">Open billing</button>
            {status !== "failed" && error && <p className="warning">{error}</p>}
          </form>
        </div>
      </section>
    </>
  );
}
