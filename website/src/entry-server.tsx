import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import App from "./App";

export { routes, site } from "./content";

export function render(path: string) {
  return renderToString(
    <StrictMode>
      <App initialPath={path} />
    </StrictMode>,
  );
}
