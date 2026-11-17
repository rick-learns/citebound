// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { findQuote, highlightQuote } from "./highlight";

const TEXT =
  "The authorization server MUST maintain the binding\n   between a refresh token and the client.";

describe("findQuote", () => {
  it("matches after collapsing whitespace and maps back to original offsets", () => {
    const span = findQuote(TEXT, "  MUST maintain the binding between a  refresh token ");
    expect(span).not.toBeNull();
    if (!span) return;
    expect(TEXT.slice(span.start, span.end)).toBe(
      "MUST maintain the binding\n   between a refresh token",
    );
  });

  it("matches across figure line breaks", () => {
    const fig = "+--------+\n|  Client |--(A)-- Grant -->\n+--------+";
    const span = findQuote(fig, "| Client |--(A)-- Grant");
    expect(span && fig.slice(span.start, span.end)).toBe("|  Client |--(A)-- Grant");
  });

  it("returns null when the quote is absent, empty or differs in case", () => {
    expect(findQuote(TEXT, "refresh tokens are encrypted")).toBeNull();
    expect(findQuote(TEXT, "   ")).toBeNull();
    expect(findQuote(TEXT, "must maintain")).toBeNull();
  });
});

describe("findQuote, like the verifier", () => {
  // RFC9068 §4 ¶8 and RFC9110 §11.6.1 ¶7, from the corpus.
  const AUD =
    '* The resource server MUST validate that the "aud" claim contains a resource indicator value corresponding to an identifier the resource server expects for itself.';
  const LOGIN = String.raw`WWW-Authenticate: Basic realm="simple", Newauth realm="apps", type=1, title="Login to \"apps\""`;
  const marked = (text: string, quote: string) => {
    const span = findQuote(text, quote);
    return span && text.slice(span.start, span.end);
  };

  it('reads \\" in the quote as " (decision 0008)', () => {
    expect(marked(AUD, String.raw`MUST validate that the \"aud\" claim contains`)).toBe(
      'MUST validate that the "aud" claim contains',
    );
  });

  it('tries the quote as written first, for RFC text with a literal \\"', () => {
    expect(marked(LOGIN, String.raw`title="Login to \"apps\""`)).toBe(
      String.raw`title="Login to \"apps\""`,
    );
    const both = String.raw`the \"aud\" claim, or the "aud" claim`;
    expect(findQuote(both, String.raw`the \"aud\" claim`)?.start).toBe(0);
  });

  it("does not unescape other backslash sequences", () => {
    expect(findQuote(AUD, String.raw`the \u0022aud\u0022 claim`)).toBeNull();
  });

  it("maps typographic quotes and ignores trailing punctuation in the quote", () => {
    expect(marked(AUD, "validate that the “aud” claim contains,")).toBe(
      'validate that the "aud" claim contains',
    );
  });
});

describe("highlightQuote", () => {
  it("wraps the matched span in <mark>", () => {
    const p = document.createElement("p");
    const { nodes, matched } = highlightQuote(TEXT, "refresh token");
    p.append(...nodes);
    expect(matched).toBe(true);
    expect(p.querySelector("mark")?.textContent).toBe("refresh token");
    expect(p.textContent).toBe(TEXT);
  });

  it("returns plain text when there is no match", () => {
    const p = document.createElement("p");
    const { nodes, matched } = highlightQuote(TEXT, "nope");
    p.append(...nodes);
    expect(matched).toBe(false);
    expect(p.querySelector("mark")).toBeNull();
    expect(p.textContent).toBe(TEXT);
  });

  it("never parses markup in the text or quote", () => {
    const text = "a <b>bold</b> <img src=x onerror=alert(1)> claim";
    const p = document.createElement("p");
    p.append(...highlightQuote(text, "<b>bold</b>").nodes);
    expect(p.querySelector("b, img")).toBeNull();
    expect(p.querySelector("mark")?.textContent).toBe("<b>bold</b>");
  });
});
