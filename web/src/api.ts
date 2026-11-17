import { SseParser } from "./sse";

/** Server answer modes: no model, a live model, or a model's recorded answers (docs/api.md). */
export type Mode = "extractive" | "model" | "replay";

export interface RfcInfo {
  number: number;
  title: string;
}

export interface Meta {
  version: string;
  mode: Mode;
  model: string;
  rfcs: RfcInfo[];
}

export interface SectionParagraph {
  ref: string;
  paragraph: number;
  text: string;
  figure: boolean;
}

export interface Section {
  rfc: number;
  rfcTitle: string;
  section: string;
  sectionTitle: string;
  paragraphs: SectionParagraph[];
}

export interface Passage extends SectionParagraph {
  rfc: number;
  rfcTitle: string;
  section: string;
  sectionTitle: string;
}

export interface ClaimEvent {
  id: number;
  text: string;
  ref: string;
  quote: string;
}

export interface VerdictEvent {
  id: number;
  status: "verified" | "struck";
  /** "" | "uncited" | "bad-citation" | "unknown-ref" | "quote-too-short" | "quote-not-found" */
  reason: string;
}

export interface AbstainEvent {
  /** "no-passages" | "answerer-abstained" | "no-verified-claims" | "too-many-struck" (decision 0006) */
  code: string;
  reason: string;
}

export interface DoneEvent {
  mode: Mode;
  model: string;
  claims: number;
  verified: number;
  struck: number;
  latencyMs: number;
  inputTokens: number;
  outputTokens: number;
  costUsd: number;
  /** The day's model tokens were used up, so the extractive answerer answered instead. */
  budgetExhausted: boolean;
}

/**
 * Error payload. Codes are the server's (docs/api.md), plus two from the client: "network" when
 * the request or the stream fails, and "incomplete" when the stream ends without done or error.
 */
export interface ErrorInfo {
  code: string;
  message: string;
}

export type AskEvent =
  | { type: "retrieval"; passages: Passage[] }
  | { type: "claim"; data: ClaimEvent }
  | { type: "verdict"; data: VerdictEvent }
  | { type: "abstain"; data: AbstainEvent }
  | { type: "done"; data: DoneEvent }
  | { type: "error"; error: ErrorInfo };

export class ApiError extends Error {
  constructor(
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// --- Defensive decoding: server JSON is untrusted, so coerce every field to its type. ---

type Obj = Record<string, unknown>;
const isObj = (v: unknown): v is Obj => typeof v === "object" && v !== null && !Array.isArray(v);
const str = (o: Obj, k: string): string => {
  const v = o[k];
  return typeof v === "string" ? v : "";
};
const num = (o: Obj, k: string): number => {
  const v = o[k];
  return typeof v === "number" && Number.isFinite(v) ? v : 0;
};
const list = (o: Obj, k: string): Obj[] => {
  const v = o[k];
  return Array.isArray(v) ? v.filter(isObj) : [];
};
const isMode = (v: unknown): v is Mode => v === "extractive" || v === "model" || v === "replay";
// Only extractive has no model, so an unknown mode with a model name is not shown as extractive.
const mode = (o: Obj): Mode => {
  const m = o["mode"];
  if (isMode(m)) return m;
  return str(o, "model") ? "model" : "extractive";
};

function paragraphOf(o: Obj): SectionParagraph {
  return {
    ref: str(o, "ref"),
    paragraph: num(o, "paragraph"),
    text: str(o, "text"),
    figure: o["figure"] === true,
  };
}

export function parseMeta(o: unknown): Meta {
  if (!isObj(o)) throw new ApiError("internal", "Malformed /api/meta response.");
  return {
    version: str(o, "version"),
    mode: mode(o),
    model: str(o, "model"),
    rfcs: list(o, "rfcs").map((r) => ({ number: num(r, "number"), title: str(r, "title") })),
  };
}

export function parseSection(o: unknown): Section {
  if (!isObj(o)) throw new ApiError("internal", "Malformed /api/section response.");
  return {
    rfc: num(o, "rfc"),
    rfcTitle: str(o, "rfcTitle"),
    section: str(o, "section"),
    sectionTitle: str(o, "sectionTitle"),
    paragraphs: list(o, "paragraphs").map(paragraphOf),
  };
}

/** Decodes one SSE message into a typed event; unknown or malformed events yield null. */
export function parseAskEvent(name: string, data: string): AskEvent | null {
  let o: unknown;
  try {
    o = JSON.parse(data);
  } catch {
    return null;
  }
  if (!isObj(o)) return null;
  switch (name) {
    case "retrieval":
      return {
        type: "retrieval",
        passages: list(o, "passages").map((p) => ({
          ...paragraphOf(p),
          rfc: num(p, "rfc"),
          rfcTitle: str(p, "rfcTitle"),
          section: str(p, "section"),
          sectionTitle: str(p, "sectionTitle"),
        })),
      };
    case "claim":
      if (typeof o["id"] !== "number") return null;
      return {
        type: "claim",
        data: { id: o["id"], text: str(o, "text"), ref: str(o, "ref"), quote: str(o, "quote") },
      };
    case "verdict":
      if (typeof o["id"] !== "number") return null;
      return {
        type: "verdict",
        // The verifier has only "verified" and "struck" (internal/verify); anything else fails safe.
        data: {
          id: o["id"],
          status: o["status"] === "verified" ? "verified" : "struck",
          reason: str(o, "reason"),
        },
      };
    case "abstain":
      return { type: "abstain", data: { code: str(o, "code"), reason: str(o, "reason") } };
    case "done":
      return {
        type: "done",
        data: {
          mode: mode(o),
          model: str(o, "model"),
          claims: num(o, "claims"),
          verified: num(o, "verified"),
          struck: num(o, "struck"),
          latencyMs: num(o, "latencyMs"),
          inputTokens: num(o, "inputTokens"),
          outputTokens: num(o, "outputTokens"),
          costUsd: num(o, "costUsd"),
          budgetExhausted: o["budgetExhausted"] === true,
        },
      };
    case "error":
      return {
        type: "error",
        error: { code: str(o, "code") || "internal", message: str(o, "message") },
      };
    default:
      return null;
  }
}

// Fallback when a non-200 body has no error code; statuses as in docs/api.md.
const STATUS_CODES: Record<number, string> = {
  400: "bad_request",
  404: "not_found",
  405: "bad_request",
  415: "bad_request",
  429: "rate_limited",
  500: "internal",
  503: "unavailable",
};

/** Builds an ApiError from a non-200 response body of the form {"error":{code,message}}. */
export function errorFromBody(body: unknown, status: number): ApiError {
  const e = isObj(body) ? body["error"] : undefined;
  if (isObj(e) && str(e, "code") !== "") return new ApiError(str(e, "code"), str(e, "message"));
  return new ApiError(STATUS_CODES[status] ?? "internal", `HTTP ${String(status)}`);
}

const isAbort = (err: unknown, signal: AbortSignal | null | undefined): boolean =>
  signal?.aborted === true || (err instanceof Error && err.name === "AbortError");

/** Calls fn, turning any failure but an abort into ApiError "network". */
async function network<T>(
  fn: () => Promise<T>,
  signal: AbortSignal | null | undefined,
): Promise<T> {
  try {
    return await fn();
  } catch (err) {
    if (isAbort(err, signal)) throw err;
    throw new ApiError("network", err instanceof Error ? err.message : String(err));
  }
}

/**
 * Reads a JSON body, or null if it is not JSON. A failure while reading is ApiError "network",
 * and an abort is rethrown.
 */
async function readJson(res: Response, signal: AbortSignal | null | undefined): Promise<unknown> {
  const text = await network(() => res.text(), signal);
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return null;
  }
}

async function getJson(url: string, signal?: AbortSignal): Promise<unknown> {
  const res = await network(
    () => fetch(url, { headers: { Accept: "application/json" }, signal: signal ?? null }),
    signal,
  );
  const body = await readJson(res, signal);
  if (!res.ok) throw errorFromBody(body, res.status);
  return body;
}

export async function fetchMeta(signal?: AbortSignal): Promise<Meta> {
  return parseMeta(await getJson("/api/meta", signal));
}

export async function fetchSection(
  rfc: number,
  section: string,
  signal?: AbortSignal,
): Promise<Section> {
  const q = new URLSearchParams({ rfc: String(rfc), section });
  return parseSection(await getJson(`/api/section?${q.toString()}`, signal));
}

/**
 * POSTs a question and yields typed events as the SSE response streams in. The last event is
 * done or error. Rejects with ApiError on a non-200 response, a network failure, or a stream
 * that ends early ("incomplete"), and with the fetch AbortError on abort.
 */
export async function* askStream(question: string, signal: AbortSignal): AsyncGenerator<AskEvent> {
  const res = await network(
    () =>
      fetch("/api/ask", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
        body: JSON.stringify({ question }),
        signal,
      }),
    signal,
  );
  if (!res.ok) throw errorFromBody(await readJson(res, signal), res.status);
  if (!res.body) throw new ApiError("internal", "The server sent an empty response.");

  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  const parser = new SseParser();
  try {
    for (;;) {
      const { done, value } = await network(() => reader.read(), signal);
      if (done) throw new ApiError("incomplete", "The answer stream ended before it finished.");
      for (const msg of parser.push(value)) {
        const event = parseAskEvent(msg.event, msg.data);
        if (!event) continue;
        yield event;
        // The server sends nothing after done or error (internal/server/events.go).
        if (event.type === "done" || event.type === "error") return;
      }
    }
  } finally {
    // Release the connection if the consumer stops early; harmless after completion.
    reader.cancel().catch(() => undefined);
  }
}
