import { useEffect, useState } from "react";
import Layout from "./components/Layout";
import { routes, site } from "./content";
import Features from "./pages/Features";
import Home from "./pages/Home";
import { FaqPage, NotFound, PricingPage, RoadmapPage } from "./pages/Other";

export function normalise(pathname: string) {
  return pathname.replace(/\/+$/, "") || "/";
}

const pages: Record<string, () => React.JSX.Element> = {
  "/": Home,
  "/features": Features,
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
  license: "https://www.gnu.org/licenses/agpl-3.0.html",
  offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
};

// Internal links swap the page in place instead of reloading. Each page also exists
// as its own HTML file, so a direct visit or a refresh works without this.
function useRoute(initial: string) {
  const [path, setPath] = useState(initial);

  useEffect(() => {
    const onPop = () => setPath(normalise(location.pathname));

    const onClick = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
        return;
      }
      const link = (event.target as Element).closest("a");
      if (!link || link.target || link.origin !== location.origin) return;
      const next = normalise(link.pathname);
      if (next === normalise(location.pathname) || !(next in pages)) return;
      event.preventDefault();
      history.pushState(null, "", next + link.hash);
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

  return (
    <Layout path={path}>
      {path === "/" && (
        <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }} />
      )}
      <Page />
    </Layout>
  );
}
