import { useState } from "react";
import { plans, pricingNotes, site } from "../content";

/** Sends the buyer to Stripe's checkout for a plan. */
async function buy(plan: string, yearly: boolean, nodes: number) {
  const response = await fetch("/api/checkout", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ plan, period: yearly ? "year" : "month", nodes }),
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error ?? "Checkout could not be started.");
  location.href = data.url;
}

export default function Pricing() {
  const [yearly, setYearly] = useState(false);
  const [nodes, setNodes] = useState(5);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");

  return (
    <>
      <div className="billing" role="group" aria-label="Billing period">
        <button type="button" aria-pressed={!yearly} onClick={() => setYearly(false)}>
          Monthly
        </button>
        <button type="button" aria-pressed={yearly} onClick={() => setYearly(true)}>
          Yearly <span>2 months free</span>
        </button>
      </div>

      <div className="plans">
        {plans.map((plan) => (
          <article key={plan.name} className={`plan${plan.featured ? " featured" : ""}`}>
            <h3>{plan.name}</h3>
            <p className="plan-audience">{plan.audience}</p>
            <p className="price">
              {plan.monthly === 0 ? "Free" : `$${yearly ? plan.monthly * 10 : plan.monthly}`}
              {plan.monthly > 0 && <span>{yearly ? "per year" : "per month"}</span>}
            </p>
            <p className="plan-detail">
              {plan.perNode
                ? `Covers 5 nodes. $${yearly ? plan.perNode * 10 : plan.perNode} for each node after that.`
                : plan.monthly === 0
                  ? "No server or node limit."
                  : "Unlimited servers and nodes."}
            </p>
            <p className="plan-includes">{plan.includes}</p>
            <ul>
              {plan.features.map((feature) => (
                <li key={feature}>{feature}</li>
              ))}
            </ul>
            <p className="plan-support">
              <span>Support</span>
              {plan.support}
            </p>
            {plan.monthly > 0 &&
              (site.paymentsOpen ? (
                <div className="plan-buy">
                  {plan.perNode && (
                    <label>
                      Nodes
                      <input type="number" min={5} max={1000} value={nodes} onChange={(event) => setNodes(Math.max(5, Number(event.target.value) || 5))} />
                    </label>
                  )}
                  <button
                    type="button"
                    className="button"
                    disabled={busy !== ""}
                    onClick={() => {
                      setBusy(plan.name);
                      setError("");
                      buy(plan.name.toLowerCase(), yearly, nodes).catch((problem: Error) => {
                        setError(problem.message);
                        setBusy("");
                      });
                    }}
                  >
                    {busy === plan.name ? "Opening checkout…" : `Get ${plan.name}`}
                  </button>
                </div>
              ) : (
                <p className="plan-soon">Not on sale yet</p>
              ))}
          </article>
        ))}
      </div>

      {error && <p className="warning">{error}</p>}

      <ul className="notes">
        {pricingNotes.map((note) => (
          <li key={note}>{note}</li>
        ))}
      </ul>
    </>
  );
}
