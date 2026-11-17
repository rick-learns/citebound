// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { renderShell } from "./shell";

describe("renderShell", () => {
  it("renders the product name", () => {
    const root = document.createElement("main");
    renderShell(root);
    expect(root.querySelector("h1")?.textContent).toBe("Citebound");
  });
});
