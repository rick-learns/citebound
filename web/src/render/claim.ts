import { parseRef, reasonText } from "../format";
import type { ClaimState } from "../state";
import { h } from "./dom";

export interface Citation {
  ref: string;
  quote: string;
}

/** Called when a citation chip is activated; opener receives focus back on close. */
export type CiteHandler = (citation: Citation, opener: HTMLElement) => void;

const ICONS: Record<ClaimState["status"], string> = {
  pending: "",
  verified: "✓",
  struck: "✕",
  unverified: "–",
};

function statusText(claim: ClaimState): string {
  switch (claim.status) {
    case "pending":
      return "Checking quote…";
    case "verified":
      return "Verified quote";
    case "struck":
      return `Struck: ${reasonText(claim.reason, claim.ref)}`;
    case "unverified":
      return "Not verified: the answer stopped first";
  }
}

/** Only verified claims with a well-formed ref get an interactive chip. */
function citeChip(claim: ClaimState, onCite: CiteHandler): HTMLElement {
  if (claim.status === "verified" && parseRef(claim.ref)) {
    const button = h(
      "button",
      { type: "button", class: "chip chip--link", "aria-label": `Show source ${claim.ref}` },
      claim.ref,
    );
    button.addEventListener("click", () => {
      onCite({ ref: claim.ref, quote: claim.quote }, button);
    });
    return button;
  }
  return h("span", { class: claim.ref ? "chip" : "chip chip--none" }, claim.ref || "no citation");
}

export function renderClaim(claim: ClaimState, onCite: CiteHandler): HTMLLIElement {
  const text = claim.status === "struck" ? h("s", {}, claim.text) : claim.text;
  return h(
    "li",
    { class: `claim claim--${claim.status}`, "data-claim-id": String(claim.id) },
    h("span", { class: "claim__icon", "aria-hidden": "true" }, ICONS[claim.status]),
    h(
      "div",
      { class: "claim__body" },
      h("p", { class: "claim__text" }, text),
      h(
        "p",
        { class: "claim__meta" },
        citeChip(claim, onCite),
        h("span", { class: "claim__status" }, statusText(claim)),
      ),
    ),
  );
}
