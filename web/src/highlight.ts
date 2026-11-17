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

// Go's unicode.IsSpace, which strings.Fields uses. JavaScript's \s differs: it lacks U+0085
// and adds U+FEFF.
const GO_SPACE = /[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]/u;

/**
 * Finds quote inside text the way internal/verify does: both are normalized (typographic quotes
 * and dashes as ASCII, each run of whitespace as one space, ends trimmed, trailing .,;: dropped),
 * and the normalized quote is tried as written, then with each \" read as " (decision 0008).
 * Returns offsets into the original text, or null.
 */
export function findQuote(text: string, quote: string): Span | null {
  const needle = collapse(quote)
    .chars.join("")
    .replace(/[.,;:]+$/u, "");
  if (needle === "") return null;
  const unescaped = needle.replaceAll('\\"', '"');
  return locate(text, needle) ?? (unescaped === needle ? null : locate(text, unescaped));
}

/**
 * Normalizes s like Go's strings.Fields after the typographic map, keeping for each output
 * character the original range it stands for: a space covers its whole run of whitespace.
 */
function collapse(s: string): { chars: string[]; from: number[]; to: number[] } {
  const chars: string[] = [];
  const from: number[] = [];
  const to: number[] = [];
  for (let i = 0; i < s.length;) {
    if (GO_SPACE.test(s.charAt(i))) {
      const runStart = i;
      while (i < s.length && GO_SPACE.test(s.charAt(i))) i++;
      // Fields drops leading and trailing whitespace.
      if (chars.length > 0 && i < s.length) {
        chars.push(" ");
        from.push(runStart);
        to.push(i);
      }
      continue;
    }
    const ch = s.charAt(i);
    chars.push(TYPOGRAPHIC[ch] ?? ch);
    from.push(i);
    to.push(i + 1);
    i++;
  }
  return { chars, from, to };
}

function locate(text: string, needle: string): Span | null {
  const { chars, from, to } = collapse(text);
  const at = chars
    .join("")
    .replace(/[.,;:]+$/u, "")
    .indexOf(needle);
  if (at < 0) return null;
  const start = from[at];
  const end = to[at + needle.length - 1];
  if (start === undefined || end === undefined) return null;
  return { start, end };
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
