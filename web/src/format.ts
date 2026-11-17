import type { DoneEvent, ErrorInfo, Mode } from "./api";

export interface RefParts {
  rfc: number;
  section: string;
  paragraph: number;
}

// The same pattern as internal/corpus ParseRef.
const REF_RE = /^RFC ?(\d+) §([0-9A-Za-z.]+?) ¶(\d+)$/u;

const isPositiveInt = (n: number): boolean => Number.isSafeInteger(n) && n > 0;

/** Parses "RFC6749 §10.4 ¶1"; returns null for "" or anything malformed. */
export function parseRef(ref: string): RefParts | null {
  const m = REF_RE.exec(ref);
  if (!m?.[1] || !m[2] || !m[3]) return null;
  const rfc = Number(m[1]);
  const paragraph = Number(m[3]);
  // Go rejects zero and numbers past int64. Past 2^53, this rejects what Go would accept: no RFC
  // is that large, and a rejected ref only loses its chip.
  if (!isPositiveInt(rfc) || !isPositiveInt(paragraph)) return null;
  return { rfc, section: m[2], paragraph };
}

/**
 * Links a corpus section ("10.4", "A.1", "Abstract"; internal/corpus) to rfc-editor.org, which
 * anchors sections as #section-10.4 and appendices as #appendix-A.1 (checked on RFC 6749, 7636,
 * and 9449). The abstract opens the page.
 */
export function rfcEditorUrl(rfc: number, section: string): string {
  const page = `https://www.rfc-editor.org/rfc/rfc${String(rfc)}`;
  if (section === "Abstract") return page;
  const kind = /^[A-Z]/u.test(section) ? "appendix" : "section";
  return `${page}#${kind}-${encodeURIComponent(section)}`;
}

/** "extractive", "live model <name>", or "replay of <name>'s recorded answers". */
export function modeText(mode: Mode, model: string): string {
  switch (mode) {
    case "extractive":
      return "extractive";
    case "model":
      return model ? `live model ${model}` : "live model";
    case "replay":
      return model ? `replay of ${model}'s recorded answers` : "replay of recorded answers";
  }
}

/** Plain-words explanation of why the verifier struck a claim. */
export function reasonText(reason: string, ref: string): string {
  const where = ref || "the cited paragraph";
  switch (reason) {
    case "uncited":
      return "no citation was given";
    case "bad-citation":
      return ref ? `"${ref}" is not a valid citation` : "the citation is malformed";
    case "unknown-ref":
      return `${where} does not exist in the corpus`;
    case "quote-too-short":
      return `quote is too short to verify against ${where}`;
    case "quote-not-found":
      return `quote not found in ${where}`;
    default:
      return "the verifier could not confirm it";
  }
}

export interface FriendlyError {
  title: string;
  hint: string;
}

/** Maps an error code to a friendly title and hint; the raw server message is shown separately. */
export function friendlyError(error: ErrorInfo): FriendlyError {
  switch (error.code) {
    case "rate_limited":
      return {
        title: "Too many questions",
        hint: "Each visitor can ask a few questions a minute. Wait as long as the message below says, then try again.",
      };
    case "bad_request":
      return {
        title: "That question couldn't be processed",
        hint: "Check that it isn't empty or unusually long, then try again.",
      };
    case "unavailable":
      return {
        title: "The assistant is temporarily unavailable",
        hint: "Please try again in a moment.",
      };
    case "not_found":
      return { title: "Not found", hint: "That section isn't in the corpus." };
    case "not_recorded":
      return {
        title: "No recorded answer",
        hint: "This server replays recorded answers, so it can answer only the recorded questions. Try one of the examples.",
      };
    // Client-only codes (api.ts): the request failed, or the stream ended before done.
    case "incomplete":
      return {
        title: "The answer was cut off",
        hint: "The connection closed before the answer finished. Please ask again.",
      };
    case "network":
      return {
        title: "Couldn't reach the server",
        hint: "Check your connection and try again.",
      };
    default:
      return { title: "Something went wrong", hint: "Please try again." };
  }
}

/** "2 of 3 claims verified · 812 ms · extractive", plus tokens and cost when a model reports them. */
export function summaryText(done: DoneEvent): string {
  const noun = done.claims === 1 ? "claim" : "claims";
  const parts = [
    done.claims === 0
      ? "No claims verified"
      : `${String(done.verified)} of ${String(done.claims)} ${noun} verified`,
    `${String(Math.round(done.latencyMs))} ms`,
    done.budgetExhausted
      ? `${modeText(done.mode, done.model)}, because the model's daily budget is used up`
      : modeText(done.mode, done.model),
  ];
  const tokens = done.inputTokens + done.outputTokens;
  if (tokens > 0) parts.push(`${tokens.toLocaleString("en-US")} tokens`);
  if (done.costUsd > 0) parts.push(`$${done.costUsd.toFixed(4)}`);
  return parts.join(" · ");
}
