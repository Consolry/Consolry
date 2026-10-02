import { useEffect, useRef } from "react";
import { site } from "../content";

// EmailOctopus draws the sign-up form itself, including its hidden spam check, so its
// look is set in the EmailOctopus form designer rather than in this site's styles.
export default function Waitlist() {
  const holder = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const element = holder.current;
    if (!element || !site.emailOctopusFormId) return;
    // The script puts the form next to its own tag, so the tag has to sit inside the holder.
    const script = document.createElement("script");
    script.async = true;
    script.src = `https://eocampaign1.com/form/${site.emailOctopusFormId}.js`;
    script.dataset.form = site.emailOctopusFormId;
    element.appendChild(script);
    return () => {
      element.replaceChildren();
    };
  }, []);

  if (!site.emailOctopusFormId) {
    return <p className="waitlist-note">The waitlist isn't open yet. Check back soon.</p>;
  }

  return (
    <div className="waitlist" ref={holder}>
      <noscript>Turn on JavaScript to join the waitlist.</noscript>
    </div>
  );
}
