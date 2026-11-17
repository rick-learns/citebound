// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { createFooter, createMasthead, TAGLINE } from "./header";

describe("createMasthead", () => {
  it("names the product and shows the README's tagline", () => {
    const { el } = createMasthead();
    expect(el.querySelector("h1")?.textContent).toBe("Citebound");
    expect(el.querySelector(".masthead__tagline")?.textContent).toBe(TAGLINE);
    expect(TAGLINE).toBe("An RFC assistant that checks every quotation against pinned RFC text.");
  });

  it("says when /api/meta could not be loaded", () => {
    const masthead = createMasthead();
    masthead.setMeta(null);
    const badge = masthead.el.querySelector<HTMLElement>(".badge");
    expect(badge?.dataset["mode"]).toBe("offline");
    expect(badge?.textContent).toBe("Mode: server unreachable");
  });

  it("shows the answer mode from /api/meta, naming the model", () => {
    const masthead = createMasthead();
    const badge = () => masthead.el.querySelector<HTMLElement>(".badge");
    const meta = { version: "v1", rfcs: [] };
    masthead.setMeta({ ...meta, mode: "extractive", model: "" });
    expect(badge()?.textContent).toBe("Mode: extractive");
    masthead.setMeta({ ...meta, mode: "model", model: "anthropic/m-1" });
    expect(badge()?.textContent).toBe("Mode: live model anthropic/m-1");
    masthead.setMeta({ ...meta, mode: "replay", model: "openrouter/m-2" });
    expect(badge()?.textContent).toBe("Mode: replay of openrouter/m-2's recorded answers");
    expect(badge()?.dataset["mode"]).toBe("replay");
  });
});

describe("createFooter", () => {
  it("lists the corpus as text, with each RFC title on hover", () => {
    const footer = createFooter();
    footer.setMeta({
      version: "v1",
      mode: "extractive",
      model: "",
      rfcs: [
        { number: 6749, title: "<b>The OAuth 2.0 Authorization Framework</b>" },
        { number: 7636, title: "PKCE" },
      ],
    });
    expect(footer.el.querySelector(".footer__corpus")?.textContent).toBe(
      "Corpus: RFC 6749 · RFC 7636",
    );
    expect(footer.el.querySelector("b")).toBeNull();
    expect(footer.el.querySelector("abbr")?.title).toBe(
      "<b>The OAuth 2.0 Authorization Framework</b>",
    );
    expect(footer.el.querySelector(".footer__version")?.textContent).toBe("Citebound v1");
  });
});
