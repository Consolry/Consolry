import { useEffect, useState, type FormEvent } from "react";
import { api, formatTime, type LicenceInfo } from "./api";
import { ErrorNote, useAction } from "./ui";

const planNames: Record<string, string> = {
  community: "Community (free)",
  plus: "Plus",
  pro: "Pro",
  host: "Host",
};

/** The admin's Licence page: which plan this panel has, and where to paste a key. */
export default function Licence() {
  const [licence, setLicence] = useState<LicenceInfo | null>(null);
  const [key, setKey] = useState("");
  const { error, busy, run } = useAction();

  useEffect(() => {
    api.licence().then(setLicence, () => {});
  }, []);
  if (!licence) return null;

  function save(event: FormEvent) {
    event.preventDefault();
    run("save", async () => {
      setLicence(await api.setLicence(key));
      setKey("");
    });
  }

  const expired = licence.hasKey && licence.plan === "community";

  return (
    <>
      <header className="page-head">
        <h1>Licence</h1>
      </header>
      <p className="dim lead">
        Consolry is free to run. A paid plan adds extra features to this panel;
        it is bought on{" "}
        <a
          href="https://www.consolry.com/pricing"
          target="_blank"
          rel="noreferrer"
        >
          consolry.com
        </a>
        .
      </p>

      <section className="card form wide">
        <h2>This panel's plan</h2>
        <dl className="addresses">
          <div>
            <dt>Plan</dt>
            <dd>
              <strong>{planNames[licence.plan] ?? licence.plan}</strong>
            </dd>
          </div>
          {licence.hasKey && licence.expires && (
            <div>
              <dt>{expired ? "Ran out" : "Paid until"}</dt>
              <dd>
                {formatTime(licence.expires)}
                {!expired && (
                  <small className="dim">
                    {" "}
                    · renewed automatically each day while the subscription is
                    paid
                  </small>
                )}
              </dd>
            </div>
          )}
          {licence.licensedPlan === "host" && (
            <div>
              <dt>Nodes covered</dt>
              <dd>{licence.nodes}</dd>
            </div>
          )}
        </dl>
        {expired && (
          <p className="note warn">
            The {planNames[licence.licensedPlan ?? ""] ?? ""} licence has run
            out, so this panel is back on the free plan. Check the subscription
            on{" "}
            <a
              href="https://www.consolry.com/license"
              target="_blank"
              rel="noreferrer"
            >
              consolry.com/license
            </a>
            .
          </p>
        )}
      </section>

      <form className="card form wide" onSubmit={save}>
        <h2>
          {licence.hasKey ? "Replace the licence key" : "Add a licence key"}
        </h2>
        <label>
          Licence key
          <textarea
            className="mono"
            value={key}
            onChange={(event) => setKey(event.target.value)}
            rows={3}
            placeholder="CONSOLRY-…"
            required
            spellCheck={false}
          />
          <small>
            You get it right after paying, and can manage the subscription at
            consolry.com/license.
          </small>
        </label>
        <ErrorNote message={error} />
        <span className="row-actions">
          <button className="primary" disabled={busy !== ""}>
            Save
          </button>
          {licence.hasKey && (
            <button
              type="button"
              disabled={busy !== ""}
              onClick={() =>
                run("remove", async () => setLicence(await api.setLicence("")))
              }
            >
              Remove the key
            </button>
          )}
        </span>
      </form>
    </>
  );
}
