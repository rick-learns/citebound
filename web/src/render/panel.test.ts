// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, type Section } from "../api";
import { createSourcePanel } from "./panel";

const XSS = "<img src=x onerror=alert(1)>";

afterEach(() => {
  document.body.replaceChildren();
});

describe("createSourcePanel", () => {
  const SECTION: Section = {
    rfc: 6749,
    rfcTitle: "The OAuth 2.0 Authorization Framework",
    section: "10.4",
    sectionTitle: "Refresh Tokens",
    paragraphs: [
      { ref: "RFC6749 §10.4 ¶1", paragraph: 1, text: 'First, a "quoted" word.', figure: false },
      {
        ref: "RFC6749 §10.4 ¶2",
        paragraph: 2,
        text: "The server MUST   maintain the\nbinding. " + XSS,
        figure: false,
      },
      { ref: "RFC6749 §10.4 ¶3", paragraph: 3, text: "+--+\n|  |\n+--+", figure: true },
    ],
  };

  function setup() {
    const load = vi.fn(() => Promise.resolve(SECTION));
    const panel = createSourcePanel(load);
    const opener = document.createElement("button");
    document.body.append(opener, panel.backdrop, panel.el);
    return { panel, load, opener };
  }

  it("shows the section with the cited paragraph and quote highlighted", async () => {
    const { panel, load, opener } = setup();
    await panel.open({ ref: "RFC6749 §10.4 ¶2", quote: "MUST maintain the binding." }, opener);
    expect(load).toHaveBeenCalledWith(6749, "10.4", expect.any(AbortSignal));
    expect(panel.el.hidden).toBe(false);
    expect(panel.el.querySelector(".panel__kicker")?.textContent).toBe("RFC 6749");
    expect(panel.el.querySelector(".panel__title")?.textContent).toBe(SECTION.rfcTitle);
    expect(panel.el.querySelector(".panel__section")?.textContent).toBe("§10.4 Refresh Tokens");
    const cited = panel.el.querySelector(".para--cited");
    expect(cited?.getAttribute("data-paragraph")).toBe("2");
    // Trailing punctuation in a quote is ignored, as by the verifier.
    expect(cited?.querySelector("mark")?.textContent).toBe("MUST   maintain the\nbinding");
    expect(panel.el.querySelector("img")).toBeNull();
    expect(panel.el.querySelector("pre")?.textContent).toBe("+--+\n|  |\n+--+");
    expect(panel.el.querySelector("a")?.getAttribute("href")).toBe(
      "https://www.rfc-editor.org/rfc/rfc6749#section-10.4",
    );
  });

  it("highlights a quote whose double quotes the model escaped", async () => {
    const { panel, opener } = setup();
    await panel.open({ ref: "RFC6749 §10.4 ¶1", quote: String.raw`a \"quoted\" word` }, opener);
    const cited = panel.el.querySelector(".para--cited");
    expect(cited?.querySelector("mark")?.textContent).toBe('a "quoted" word');
    expect(cited?.querySelector(".para__note")).toBeNull();
  });

  it("highlights only the paragraph when the quote is not found", async () => {
    const { panel, opener } = setup();
    await panel.open({ ref: "RFC6749 §10.4 ¶2", quote: "not in there" }, opener);
    const cited = panel.el.querySelector(".para--cited");
    expect(cited).not.toBeNull();
    expect(cited?.querySelector("mark")).toBeNull();
    expect(cited?.querySelector(".para__note")).not.toBeNull();
  });

  it("closes on Escape and returns focus to the opener", async () => {
    const { panel, opener } = setup();
    await panel.open({ ref: "RFC6749 §10.4 ¶1", quote: "" }, opener);
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(panel.isOpen()).toBe(false);
    expect(document.activeElement).toBe(opener);
  });

  it("closes with the close button", async () => {
    const { panel, opener } = setup();
    await panel.open({ ref: "RFC6749 §10.4 ¶1", quote: "" }, opener);
    panel.el.querySelector<HTMLButtonElement>(".panel__close")?.click();
    expect(panel.isOpen()).toBe(false);
    expect(document.activeElement).toBe(opener);
  });

  it("says so when the cited paragraph is not in the section", async () => {
    const { panel, opener } = setup();
    await panel.open({ ref: "RFC6749 §10.4 ¶9", quote: "" }, opener);
    expect(panel.el.querySelector(".para--cited")).toBeNull();
    const status = panel.el.querySelector<HTMLElement>(".panel__status");
    expect(status?.hidden).toBe(false);
    expect(status?.textContent).toBe("This section has no ¶9.");
  });

  it("explains a section that fails to load", async () => {
    const panel = createSourcePanel(() =>
      Promise.reject(new ApiError("not_found", "no such section")),
    );
    document.body.append(panel.el);
    await panel.open({ ref: "RFC6749 §99 ¶1", quote: "" }, null);
    expect(panel.el.querySelector(".panel__status")?.textContent).toBe(
      "That section isn't in the corpus.",
    );
    expect(panel.el.querySelector(".para")).toBeNull();
  });

  it("shows only the latest citation when a slower earlier load finishes last", async () => {
    let resolveFirst: (s: Section) => void = () => undefined;
    const load = vi
      .fn<(rfc: number, section: string, signal: AbortSignal) => Promise<Section>>()
      .mockImplementationOnce(() => new Promise((resolve) => (resolveFirst = resolve)))
      .mockImplementationOnce(() => Promise.resolve(SECTION));
    const panel = createSourcePanel(load);
    document.body.append(panel.el);
    const first = panel.open({ ref: "RFC7636 §4.1 ¶1", quote: "" }, null);
    await panel.open({ ref: "RFC6749 §10.4 ¶2", quote: "" }, null);
    expect(load.mock.calls[0]?.[2].aborted).toBe(true);
    resolveFirst({ ...SECTION, rfc: 7636, rfcTitle: "PKCE", section: "4.1" });
    await first;
    expect(panel.el.querySelector(".panel__title")?.textContent).toBe(SECTION.rfcTitle);
    expect(panel.el.querySelector(".para--cited")?.getAttribute("data-paragraph")).toBe("2");
  });
});
