import type { ReactNode } from "react";
import { routes, site } from "../content";
import Waitlist from "./Waitlist";

function Logo() {
  return (
    <a href="/" className="logo">
      <svg viewBox="0 0 32 32" aria-hidden="true" shapeRendering="crispEdges">
        <path className="logo-block" d="M4 0h24v4h4v24h-4v4H4v-4H0V4h4z" />
        <path className="logo-mark" d="M8 10h4v4h4v4h-4v4H8v-4h4v-4H8zM18 18h8v4h-8z" />
      </svg>
      Consolry
    </a>
  );
}

const navRoutes = routes.filter((route) => route.nav);

export default function Layout({ path, children }: { path: string; children: ReactNode }) {
  return (
    <>
      <a href="#main" className="skip">
        Skip to content
      </a>

      <p className="announce">
        <span className="pixel">Early preview</span>
        Consolry now runs real Minecraft servers on Windows and Linux. <a href="/download">Download it</a>
      </p>

      <header className="site-header">
        <div className="wrap">
          <Logo />
          <nav aria-label="Main">
            {navRoutes.map((route) => (
              <a key={route.path} href={route.path} aria-current={route.path === path ? "page" : undefined}>
                {route.nav}
              </a>
            ))}
            <a href={site.github}>GitHub</a>
          </nav>
          <a href="#waitlist" className="button small">
            Join the waitlist
          </a>
        </div>
      </header>

      <main id="main">{children}</main>

      <section className="cta" id="waitlist" aria-labelledby="waitlist-title">
        <div className="wrap cta-grid">
          <div>
            <p className="label">Waitlist</p>
            <h2 id="waitlist-title">Get one email when it's ready.</h2>
            <p>Leave your address and we'll write once, when the first release is out.</p>
          </div>
          <Waitlist />
        </div>
      </section>

      <footer className="site-footer">
        <div className="wrap">
          <div>
            <Logo />
            <p>An open source game server panel, licensed AGPL-3.0.</p>
            <p>
              Maintained by <a href={site.maintainer.url}>{site.maintainer.name}</a>
            </p>
          </div>
          <nav aria-label="Footer">
            <a href="/">Home</a>
            {navRoutes.map((route) => (
              <a key={route.path} href={route.path}>
                {route.nav}
              </a>
            ))}
            <a href={site.github}>Source code</a>
            <a href={`${site.github}/issues`}>Issues</a>
          </nav>
          <p className="legal">
            © 2026 {site.maintainer.name} and Consolry contributors. Not affiliated with Mojang or Microsoft. Minecraft is a trademark of Mojang AB.
          </p>
        </div>
      </footer>
    </>
  );
}

type PageHeadProps = { label: string; title: string; intro: string };

export function PageHead({ label, title, intro }: PageHeadProps) {
  return (
    <section className="page-head">
      <Clouds />
      <div className="wrap">
        <p className="label">{label}</p>
        <h1>{title}</h1>
        <p className="lede">{intro}</p>
      </div>
    </section>
  );
}

export function Clouds() {
  return (
    <div className="clouds" aria-hidden="true">
      <i />
      <i />
      <i />
    </div>
  );
}

type SectionProps = { label: string; title: string; intro?: string; alt?: boolean; children: ReactNode };

export function Section({ label, title, intro, alt, children }: SectionProps) {
  return (
    <section className={`section${alt ? " alt" : ""}`}>
      <div className="wrap">
        <header className="section-head">
          <p className="label">{label}</p>
          <h2>{title}</h2>
          {intro && <p className="intro">{intro}</p>}
        </header>
        {children}
      </div>
    </section>
  );
}
