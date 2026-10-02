import { useState } from "react";
import { plans, pricingNotes } from "../content";

export default function Pricing() {
  const [yearly, setYearly] = useState(false);

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
          </article>
        ))}
      </div>

      <ul className="notes">
        {pricingNotes.map((note) => (
          <li key={note}>{note}</li>
        ))}
      </ul>
    </>
  );
}
