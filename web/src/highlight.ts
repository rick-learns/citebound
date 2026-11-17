export interface Span {
  start: number;
  end: number;
}

// Typographic stand-ins the verifier maps to ASCII (internal/verify Normalize). Each is one
// character for one, so offsets into the original text still line up.
const TYPOGRAPHIC: Record<string, string> = {
  "\u2018": "'",
  "\u2019": "'",
  "\u201c": '"',
  "\u201d": '"',
  "\u2013": "-",
  "\u2014": "-",
  "\u2011": "-",
};

/**
 * Finds quote inside text the way the verifier compares them: every run of whitespace (including
 * the line breaks in figures) counts as one space, typographic quotes and dashes as ASCII, and
 * trailing .,;: in the quote is ignored. The quote is tried as written, then with each \" read
 * as " (decision 0008). Returns offsets into the original text, or null.
 */
export function findQuote(text: string, quote: string): Span | null {
  const unescaped = quote.replaceAll('\\"', '"');
  return locate(text, quote) ?? (unescaped === quote ? null : locate(text, unescaped));
}

function locate(text: string, quote: string): Span | null {
  const needle = quote
    .replace(/[‘’“”–—‑]/gu, (ch) => TYPOGRAPHIC[ch] ?? ch)
    .trim()
    .replace(/\s+/gu, " ")
    .replace(/[.,;:]+$/u, "")
    .trimEnd();
  if (needle === "") return null;

  // Collapsed copy of text, plus the original index of each collapsed character.
  let hay = "";
  const origin: number[] = [];
  let prevSpace = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text.charAt(i);
    const space = /\s/u.test(ch);
    if (space && prevSpace) continue;
    prevSpace = space;
    hay += space ? " " : (TYPOGRAPHIC[ch] ?? ch);
    origin.push(i);
  }

  const at = hay.indexOf(needle);
  if (at < 0) return null;
  const start = origin[at];
  const last = origin[at + needle.length - 1];
  if (start === undefined || last === undefined) return null;
  return { start, end: last + 1 };
}

/**
 * Returns text as DOM nodes with the quoted span wrapped in <mark>. Always text nodes,
 * never parsed markup. matched is false when the quote could not be located.
 */
export function highlightQuote(text: string, quote: string): { nodes: Node[]; matched: boolean } {
  const span = findQuote(text, quote);
  if (!span) return { nodes: [document.createTextNode(text)], matched: false };
  const mark = document.createElement("mark");
  mark.textContent = text.slice(span.start, span.end);
  const nodes: Node[] = [mark];
  if (span.start > 0) nodes.unshift(document.createTextNode(text.slice(0, span.start)));
  if (span.end < text.length) nodes.push(document.createTextNode(text.slice(span.end)));
  return { nodes, matched: true };
}
