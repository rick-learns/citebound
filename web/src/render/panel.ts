import { ApiError, type Section, type SectionParagraph } from "../api";
import { friendlyError, parseRef, rfcEditorUrl } from "../format";
import { highlightQuote } from "../highlight";
import type { Citation } from "./claim";
import { h, setText } from "./dom";

export type SectionLoader = (rfc: number, section: string, signal: AbortSignal) => Promise<Section>;

export interface SourcePanel {
  el: HTMLElement;
  /** Click-to-close scrim shown behind the drawer on narrow screens. Mount next to el. */
  backdrop: HTMLElement;
  open: (citation: Citation, opener: HTMLElement | null) => Promise<void>;
  close: () => void;
  isOpen: () => boolean;
}

function renderParagraph(p: SectionParagraph, cited: boolean, quote: string): HTMLElement {
  const { nodes, matched } = cited ? highlightQuote(p.text, quote) : { nodes: [], matched: false };
  const content = cited ? nodes : [p.text];
  const body = p.figure
    ? h("pre", { class: "para__figure" }, ...content)
    : h("p", { class: "para__text" }, ...content);
  const label = `¶${String(p.paragraph)}`;
  return h(
    "div",
    { class: cited ? "para para--cited" : "para", "data-paragraph": String(p.paragraph) },
    h("span", { class: "para__num" }, label),
    h(
      "div",
      { class: "para__content" },
      cited && h("span", { class: "para__badge" }, "Cited"),
      body,
    ),
    cited && quote !== "" && !matched
      ? h("p", { class: "para__note" }, "The exact quote could not be located in this paragraph.")
      : null,
  );
}

/**
 * The source viewer. A side column on wide screens, a drawer on narrow ones; while it is a
 * drawer, `background` is made inert so keyboard focus stays inside.
 */
export function createSourcePanel(load: SectionLoader, background?: HTMLElement): SourcePanel {
  const kicker = h("p", { class: "panel__kicker" });
  const title = h("h2", { id: "panel-title", class: "panel__title", tabindex: "-1" });
  const sub = h("p", { class: "panel__section" });
  const link = h("a", { class: "panel__link", target: "_blank", rel: "noopener noreferrer" });
  const closeBtn = h(
    "button",
    { type: "button", class: "panel__close", "aria-label": "Close source" },
    "×",
  );
  const status = h("p", { class: "panel__status", role: "status" });
  const paras = h("div", { class: "panel__paras" });
  const el = h(
    "aside",
    { class: "panel", "aria-labelledby": "panel-title", hidden: true },
    h(
      "div",
      { class: "panel__head" },
      h("div", { class: "panel__heading" }, kicker, title, sub),
      closeBtn,
    ),
    h("div", { class: "panel__scroll" }, status, paras, h("p", { class: "panel__out" }, link)),
  );
  const backdrop = h("div", { class: "panel-backdrop", hidden: true });

  let opener: HTMLElement | null = null;
  let inflight: AbortController | null = null;

  function setDrawerMode(on: boolean): void {
    backdrop.hidden = !on;
    if (background) background.inert = on;
  }

  // The panel is a drawer below this width. Follow the breakpoint while open, so resizing never
  // leaves the page inert behind a side column, or interactive behind a drawer.
  const narrow = window.matchMedia("(max-width: 959px)");
  narrow.addEventListener("change", () => {
    if (!el.hidden) setDrawerMode(narrow.matches);
  });

  async function open(citation: Citation, from: HTMLElement | null): Promise<void> {
    const ref = parseRef(citation.ref);
    if (!ref) return;
    inflight?.abort();
    const ctrl = new AbortController();
    inflight = ctrl;
    opener = from;

    el.hidden = false;
    setDrawerMode(narrow.matches);
    setText(kicker, `RFC ${String(ref.rfc)}`);
    setText(title, citation.ref);
    setText(sub, "");
    setText(status, `Loading ${citation.ref}…`);
    status.hidden = false;
    paras.replaceChildren();
    link.href = rfcEditorUrl(ref.rfc, ref.section);
    setText(link, `Read §${ref.section} on rfc-editor.org ↗`);
    title.focus();

    let section: Section;
    try {
      section = await load(ref.rfc, ref.section, ctrl.signal);
    } catch (err) {
      if (ctrl.signal.aborted) return;
      const code = err instanceof ApiError ? err.code : "network";
      const msg = friendlyError({ code, message: "" });
      setText(status, code === "not_found" ? msg.hint : `Couldn't load this section. ${msg.hint}`);
      return;
    }
    if (ctrl.signal.aborted) return;

    setText(title, section.rfcTitle || `RFC ${String(section.rfc)}`);
    setText(sub, `§${section.section} ${section.sectionTitle}`.trim());
    status.hidden = true;
    setText(status, "");
    const nodes = section.paragraphs.map((p) =>
      renderParagraph(p, p.paragraph === ref.paragraph, citation.quote),
    );
    paras.replaceChildren(...nodes);
    const cited = paras.querySelector(".para--cited");
    if (!cited) {
      // The server only verifies refs it can resolve, but a passage or stale link may not.
      setText(status, `This section has no ¶${String(ref.paragraph)}.`);
      status.hidden = false;
    }
    if (cited) {
      const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      cited.scrollIntoView({ block: "center", behavior: reduce ? "auto" : "smooth" });
    }
  }

  function close(): void {
    if (el.hidden) return;
    inflight?.abort();
    inflight = null;
    el.hidden = true;
    setDrawerMode(false);
    if (opener?.isConnected) opener.focus();
    opener = null;
  }

  closeBtn.addEventListener("click", close);
  backdrop.addEventListener("click", close);
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && !el.hidden) {
      e.preventDefault();
      close();
    }
  });

  return { el, backdrop, open, close, isOpen: () => !el.hidden };
}
