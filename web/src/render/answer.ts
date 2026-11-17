import type { Passage } from "../api";
import { friendlyError, summaryText } from "../format";
import type { AnswerState, ClaimState } from "../state";
import { renderClaim, type CiteHandler } from "./claim";
import { h, setText } from "./dom";

function progressText(state: AnswerState): string {
  if (state.phase !== "streaming") return "";
  if (state.passages.length === 0) return "Searching the corpus…";
  const n = state.passages.length;
  return `Read ${String(n)} ${n === 1 ? "passage" : "passages"}. Checking each claim against its source…`;
}

function intro(): HTMLElement {
  return h(
    "div",
    { class: "intro" },
    h("h2", { class: "intro__title" }, "How answers are checked"),
    h(
      "ol",
      { class: "intro__steps" },
      h("li", {}, "Every sentence in an answer cites one RFC paragraph and quotes it."),
      h(
        "li",
        {},
        "The server looks for each quote in the pinned text of that paragraph. A quote it can't find is struck out.",
      ),
      // The narrow promise (docs/limitations.md): a real quote can sit beside a false claim.
      h(
        "li",
        {},
        "A verified quote is really in the paragraph, but it may not support the sentence. Open the source to judge.",
      ),
    ),
  );
}

function passageList(passages: readonly Passage[], onCite: CiteHandler): HTMLElement[] {
  return passages.map((p) => {
    const chip = h(
      "button",
      { type: "button", class: "chip chip--link", "aria-label": `Show source ${p.ref}` },
      p.ref,
    );
    chip.addEventListener("click", () => {
      onCite({ ref: p.ref, quote: "" }, chip);
    });
    return h("li", {}, chip, h("span", { class: "passages__title" }, p.sectionTitle));
  });
}

/**
 * The answer area. update() patches the DOM in place: claims are keyed by id and only
 * re-rendered when their state object changes, so streaming doesn't re-announce or
 * steal focus from earlier claims.
 */
export function createAnswerView(onCite: CiteHandler): {
  el: HTMLElement;
  update: (state: AnswerState) => void;
} {
  const introEl = intro();
  const question = h("p", { class: "answer__question" });
  const progress = h("p", { class: "answer__progress" });
  const claims = h("ol", { class: "claims", "aria-label": "Claims" });
  const abstainReason = h("p", {});
  const abstainWithheld = h(
    "p",
    {},
    "The claims below are shown so you can see why; they are not the answer.",
  );
  const abstain = h(
    "div",
    { class: "notice notice--abstain", hidden: true },
    h("span", { class: "notice__icon", "aria-hidden": "true" }, "∅"),
    h(
      "div",
      {},
      h("p", { class: "notice__title" }, "Answer withheld"),
      abstainReason,
      abstainWithheld,
      h(
        "p",
        {},
        "Citebound only answers when it can quote the RFCs directly. Try rephrasing, or name the RFC or mechanism you mean.",
      ),
    ),
  );
  const errorTitle = h("p", { class: "notice__title" });
  const errorHint = h("p", {});
  const errorDetail = h("p", { class: "notice__detail" });
  const error = h(
    "div",
    { class: "notice notice--error", hidden: true },
    h("span", { class: "notice__icon", "aria-hidden": "true" }, "!"),
    h("div", {}, errorTitle, errorHint, errorDetail),
  );
  const summary = h("p", { class: "answer__summary" });
  const live = h(
    "div",
    { class: "answer__live", "aria-live": "polite" },
    progress,
    abstain,
    claims,
    error,
    summary,
  );
  const passageItems = h("ul", { class: "passages__list" });
  const passageCount = h("span", {});
  const passages = h(
    "details",
    { class: "passages", hidden: true },
    h("summary", {}, passageCount),
    passageItems,
  );
  const body = h("div", { class: "answer__body", hidden: true }, question, live, passages);
  const el = h(
    "section",
    { class: "answer", "aria-labelledby": "answer-heading" },
    h("h2", { id: "answer-heading", class: "visually-hidden" }, "Answer"),
    introEl,
    body,
  );

  const rendered = new Map<number, { claim: ClaimState; el: HTMLLIElement }>();
  let renderedPassages: readonly Passage[] | null = null;

  function updateClaims(list: readonly ClaimState[]): void {
    // Drop claims that are no longer in the state, such as the previous question's.
    const ids = new Set(list.map((c) => c.id));
    for (const [id, { el }] of rendered) {
      if (!ids.has(id)) {
        el.remove();
        rendered.delete(id);
      }
    }
    for (const claim of list) {
      const prev = rendered.get(claim.id);
      if (prev?.claim === claim) continue;
      const node = renderClaim(claim, onCite);
      if (prev) {
        const hadFocus = prev.el.contains(document.activeElement);
        prev.el.replaceWith(node);
        if (hadFocus) node.querySelector<HTMLElement>(".chip--link")?.focus();
      } else {
        claims.append(node);
      }
      rendered.set(claim.id, { claim, el: node });
    }
    // Keep the state's order. Nodes already in place are not moved, so a normal stream, which
    // only appends, never moves one.
    list.forEach((claim, i) => {
      const node = rendered.get(claim.id)?.el;
      if (node && claims.children[i] !== node)
        claims.insertBefore(node, claims.children[i] ?? null);
    });
  }

  function update(state: AnswerState): void {
    introEl.hidden = state.phase !== "idle";
    body.hidden = state.phase === "idle";
    setText(question, state.question);
    setText(progress, progressText(state));
    progress.hidden = progress.textContent === "";
    updateClaims(state.claims);
    claims.hidden = state.claims.length === 0;

    // Claims arrive before abstain, so they are marked withheld once it does (docs/api.md).
    const withheld = state.abstain !== null;
    abstain.hidden = !withheld;
    setText(abstainReason, state.abstain?.reason ?? "");
    abstainWithheld.hidden = state.claims.length === 0;
    claims.classList.toggle("claims--withheld", withheld);
    claims.setAttribute("aria-label", withheld ? "Withheld claims" : "Claims");

    error.hidden = state.error === null;
    if (state.error) {
      const friendly = friendlyError(state.error);
      setText(errorTitle, friendly.title);
      setText(errorHint, friendly.hint);
      setText(errorDetail, state.error.message);
    }

    // The summary is the last thing announced, so a withheld answer says so here too, and its
    // counts are not read as the result.
    let line = "";
    if (state.phase === "done" && state.done) {
      line = withheld ? `Answer withheld · ${summaryText(state.done)}` : summaryText(state.done);
    } else if (state.phase === "aborted") {
      line = withheld
        ? "Answer withheld. Stopped before the stream finished."
        : "Stopped before the answer finished.";
    } else if (state.phase === "error" && withheld) {
      line = "Answer withheld. The stream then failed.";
    }
    setText(summary, line);
    summary.hidden = line === "";

    if (state.passages !== renderedPassages) {
      renderedPassages = state.passages;
      passageItems.replaceChildren(...passageList(state.passages, onCite));
      const n = state.passages.length;
      setText(passageCount, `Passages consulted (${String(n)})`);
      passages.hidden = n === 0;
    }
  }

  return { el, update };
}
