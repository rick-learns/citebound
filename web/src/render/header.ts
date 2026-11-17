import type { Meta } from "../api";
import { modeText } from "../format";
import { h, setText } from "./dom";

export interface Masthead {
  el: HTMLElement;
  setMeta: (meta: Meta | null) => void;
}

// The README's tagline. It promises only what the verifier checks (docs/limitations.md).
export const TAGLINE = "An RFC assistant that checks every quotation against pinned RFC text.";

const MODE_HELP: Record<Meta["mode"], string> = {
  extractive: "Answers are assembled from retrieved RFC text; no language model is involved.",
  model: "A language model drafts the answer; the server still verifies every quote.",
  replay:
    "Answers are a model's recorded answers, replayed offline; every quote is still verified.",
};

export function createMasthead(): Masthead {
  const badgeText = h("span", {}, "connecting…");
  const badge = h(
    "p",
    { class: "badge", "data-mode": "pending" },
    h("span", { class: "badge__dot", "aria-hidden": "true" }),
    h("span", { class: "visually-hidden" }, "Mode: "),
    badgeText,
  );
  const el = h(
    "header",
    { class: "masthead" },
    h(
      "div",
      { class: "masthead__brand" },
      h("h1", { class: "masthead__name" }, "Citebound"),
      h("p", { class: "masthead__tagline" }, TAGLINE),
    ),
    badge,
  );

  function setMeta(meta: Meta | null): void {
    if (!meta) {
      badge.dataset["mode"] = "offline";
      badge.title = "Could not load /api/meta.";
      setText(badgeText, "server unreachable");
      return;
    }
    badge.dataset["mode"] = meta.mode;
    badge.title = MODE_HELP[meta.mode];
    setText(badgeText, modeText(meta.mode, meta.model));
  }

  return { el, setMeta };
}

/** Footer listing the corpus, filled once /api/meta arrives. */
export function createFooter(): { el: HTMLElement; setMeta: (meta: Meta | null) => void } {
  const corpus = h("p", { class: "footer__corpus" });
  const version = h("p", { class: "footer__version" });
  const el = h("footer", { class: "footer" }, corpus, version);

  function setMeta(meta: Meta | null): void {
    if (!meta) return;
    const items = meta.rfcs.map((r) =>
      h("abbr", { title: r.title, class: "footer__rfc" }, `RFC ${String(r.number)}`),
    );
    corpus.replaceChildren("Corpus: ");
    items.forEach((item, i) => {
      if (i > 0) corpus.append(" · ");
      corpus.append(item);
    });
    setText(version, `Citebound ${meta.version}`);
  }

  return { el, setMeta };
}
