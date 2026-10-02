import { useId, useState, type FormEvent } from "react";

const endpoint = import.meta.env.VITE_WAITLIST_URL as string | undefined;

type Status = "idle" | "sending" | "done" | "error" | "closed";

export default function Waitlist() {
  const inputId = useId();
  const [email, setEmail] = useState("");
  const [status, setStatus] = useState<Status>("idle");

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!endpoint) {
      setStatus("closed");
      return;
    }
    setStatus("sending");
    try {
      const response = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      setStatus(response.ok ? "done" : "error");
    } catch {
      setStatus("error");
    }
  }

  if (status === "done") {
    return (
      <p className="waitlist-done" role="status">
        You're on the list. We'll email {email} when the first release is ready.
      </p>
    );
  }

  return (
    <form className="waitlist" onSubmit={submit}>
      <label htmlFor={inputId}>Email address</label>
      <div className="waitlist-row">
        <input
          id={inputId}
          type="email"
          required
          autoComplete="email"
          placeholder="you@example.com"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
        <button type="submit" className="button" disabled={status === "sending"}>
          {status === "sending" ? "Joining…" : "Join the waitlist"}
        </button>
      </div>
      <p className="waitlist-note" role="status">
        {status === "error" && "That didn't go through. Check your connection and try again."}
        {status === "closed" && "The waitlist isn't open yet. Check back soon."}
      </p>
    </form>
  );
}
