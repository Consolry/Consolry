// Writes every page out as its own HTML file, with its own title and description,
// so search engines and link previews see the full content without running JavaScript.
import { mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const entry = readdirSync("dist-ssr").find((name) => name.startsWith("entry-server."));
const { render, routes, site } = await import(pathToFileURL(resolve("dist-ssr", entry)).href);

const template = readFileSync("dist/index.html", "utf8");
if (!template.includes("<!--app-->")) {
  throw new Error("dist/index.html has no <!--app--> placeholder");
}

const escape = (text) => text.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");

for (const route of routes) {
  const url = site.url + route.path;
  const html = template
    .replace("<!--app-->", render(route.path))
    .replace(/<title>.*?<\/title>/s, `<title>${escape(route.title)}</title>`)
    .replace(/(<meta\s+name="description"\s+content=")[^"]*"/s, `$1${escape(route.description)}"`)
    .replace(/(<meta\s+property="og:title"\s+content=")[^"]*"/s, `$1${escape(route.title)}"`)
    .replace(/(<meta\s+property="og:description"\s+content=")[^"]*"/s, `$1${escape(route.description)}"`)
    .replace(/(<meta\s+property="og:url"\s+content=")[^"]*"/s, `$1${url}"`)
    .replace(/(<link\s+rel="canonical"\s+href=")[^"]*"/s, `$1${url}"`);

  const dir = resolve("dist", `.${route.path}`);
  mkdirSync(dir, { recursive: true });
  writeFileSync(resolve(dir, "index.html"), html);
}

// Vercel serves this file for any address that does not exist.
writeFileSync(
  "dist/404.html",
  template
    .replace("<!--app-->", render("/404"))
    .replace(/<title>.*?<\/title>/s, "<title>Page not found | Consolry</title>"),
);

const sitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${routes.map((route) => `  <url><loc>${site.url}${route.path}</loc></url>`).join("\n")}
</urlset>
`;
writeFileSync("dist/sitemap.xml", sitemap);

rmSync("dist-ssr", { recursive: true, force: true });
