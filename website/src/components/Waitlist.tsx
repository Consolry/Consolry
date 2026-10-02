import { useId, useState, type FormEvent } from "react";
import { site } from "../content";

// EmailOctopus's sign-up form. Submitting opens EmailOctopus's own confirmation
// page in a new tab, which also handles the "check your inbox" step.
const action = site.emailOctopusFormId ? `https://eocampaign1.com/form/${site.emailOctopusFormId}` : undefined;

export default function Waitlist() {
  const inputId = useId();
  const [status, setStatus] = useState<"idle" | "sent" | "closed">("idle");

  function submit(event: FormEvent) {
    if (!action) {
      event.preventDefault();
      setStatus("closed");
      return;
    }
    setStatus("sent");
  }

  return (
    <form className="waitlist" action={action} method="post" target="_blank" onSubmit={submit}>
      <label htmlFor={inputId}>Email address</label>
      <div className="waitlist-row">
        <input id={inputId} type="email" name="field_0" required autoComplete="email" placeholder="you@example.com" />
        <button type="submit" className="button">
          Join the waitlist
        </button>
      </div>
      {/* EmailOctopus's spam trap: people never see it, and sign-ups that fill it in are dropped. */}
      <input type="text" name="hpc4b27b6e-eb38-11e9-be00-06b4694bee2a" tabIndex={-1} autoComplete="nope" aria-hidden="true" hidden />
      <p className="waitlist-note" role="status">
        {status === "sent" && "Almost done. Confirm in the tab that just opened, then check your inbox."}
        {status === "closed" && "The waitlist isn't open yet. Check back soon."}
      </p>
    </form>
  );
}
