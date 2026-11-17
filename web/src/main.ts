import { renderShell } from "./shell";

const root = document.querySelector<HTMLElement>("#app");
if (root) {
  renderShell(root);
}
