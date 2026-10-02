import { useId, useState, type FormEvent } from "react";
import { site } from "../content";

// Buttondown's standard sign-up form. Submitting opens Buttondown's own confirmation
// page in a new tab, which also handles the "check your inbox" step.
const action = site.buttondownUsername
  ? `https://buttondown.com/api/emails/embed-subscribe/${site.buttondownUsername}`
  : undefined;

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
        <input id={inputId} type="email" name="email" required autoComplete="email" placeholder="you@example.com" />
        <button type="submit" className="button">
          Join the waitlist
        </button>
      </div>
      <p className="waitlist-note" role="status">
        {status === "sent" && "Almost done. Confirm in the tab that just opened, then check your inbox."}
        {status === "closed" && "The waitlist isn't open yet. Check back soon."}
      </p>
    </form>
  );
}
