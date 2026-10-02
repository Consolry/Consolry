import { useEffect, useState } from "react";
import Layout from "./components/Layout";
import { routes, site } from "./content";
import DocsPage from "./pages/Docs";
import Features from "./pages/Features";
import Home from "./pages/Home";
import { DownloadPage, FaqPage, NotFound, PricingPage, RoadmapPage } from "./pages/Other";

export function normalise(pathname: string) {
  return pathname.replace(/\/+$/, "") || "/";
}

// The documentation is also served at docs.consolry.com, where "/install-linux" means "/docs/install-linux".
const onDocsHost = () => typeof location !== "undefined" && location.hostname.startsWith("docs.");

/** The page the browser is on, as a path within the site. */
export function currentPath() {
  const path = normalise(location.pathname);
  if (!onDocsHost() || path.startsWith("/docs")) return path;
  return path === "/" ? "/docs" : `/docs${path}`;
}

const known = (path: string) => routes.some((route) => route.path === path);

const pages: Record<string, () => React.JSX.Element> = {
  "/": Home,
  "/features": Features,
  "/download": DownloadPage,
  "/pricing": PricingPage,
  "/roadmap": RoadmapPage,
  "/faq": FaqPage,
};

const structuredData = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: site.name,
  url: `${site.url}/`,
  description: routes[0].description,
  applicationCategory: "GameApplication",
  operatingSystem: "Windows, Linux",
  offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
};

// Internal links swap the page in place instead of reloading. Each page also exists
// as its own HTML file, so a direct visit or a refresh works without this.
function useRoute(initial: string) {
  const [path, setPath] = useState(initial);

  useEffect(() => {
    const onPop = () => setPath(currentPath());

    const onClick = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
        return;
      }
      const link = (event.target as Element).closest("a");
      if (!link || link.target || link.origin !== location.origin) return;
      let next = normalise(link.pathname);
      if (onDocsHost() && !next.startsWith("/docs")) {
        // On the docs address only documentation is served; other links go to the main site.
        if (next !== "/") return;
        next = "/docs";
      }
      if (next === currentPath() || !known(next)) return;
      event.preventDefault();
      const shown = onDocsHost() ? next.slice("/docs".length) || "/" : next;
      history.pushState(null, "", shown + link.hash);
      setPath(next);
      window.scrollTo(0, 0);
    };

    window.addEventListener("popstate", onPop);
    document.addEventListener("click", onClick);
    return () => {
      window.removeEventListener("popstate", onPop);
      document.removeEventListener("click", onClick);
    };
  }, []);

  useEffect(() => {
    const route = routes.find((item) => item.path === path);
    if (route) document.title = route.title;
  }, [path]);

  return path;
}

export default function App({ initialPath }: { initialPath: string }) {
  const path = useRoute(initialPath);
  const Page = pages[path] ?? NotFound;
  const isDoc = !(path in pages) && path.startsWith("/docs");

  return (
    <Layout path={path}>
      {path === "/" && (
        <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }} />
      )}
      {isDoc ? <DocsPage path={path} /> : <Page />}
    </Layout>
  );
}
