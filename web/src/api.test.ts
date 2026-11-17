import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, askStream, errorFromBody, parseAskEvent, parseMeta, type AskEvent } from "./api";

function streamResponse(chunks: string[], init: ResponseInit = {}): Response {
  const enc = new TextEncoder();
  const body = new ReadableStream<Uint8Array>({
    start(ctrl) {
      for (const c of chunks) ctrl.enqueue(enc.encode(c));
      ctrl.close();
    },
  });
  return new Response(body, { headers: { "Content-Type": "text/event-stream" }, ...init });
}

async function collect(question: string, signal = new AbortController().signal) {
  const out: AskEvent[] = [];
  for await (const e of askStream(question, signal)) out.push(e);
  return out;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("parseAskEvent", () => {
  it("drops malformed and unknown events", () => {
    expect(parseAskEvent("claim", "not json")).toBeNull();
    expect(parseAskEvent("claim", '{"text":"no id"}')).toBeNull();
    expect(parseAskEvent("mystery", "{}")).toBeNull();
  });

  it("never treats an unexpected verdict status as verified", () => {
    expect(parseAskEvent("verdict", '{"id":1,"status":"maybe"}')).toEqual({
      type: "verdict",
      data: { id: 1, status: "struck", reason: "" },
    });
  });

  it("reads the abstain code and reason", () => {
    expect(
      parseAskEvent("abstain", '{"code":"too-many-struck","reason":"Too few claims."}'),
    ).toEqual({ type: "abstain", data: { code: "too-many-struck", reason: "Too few claims." } });
  });
});

describe("parseMeta", () => {
  it("keeps the three server modes and never labels a model as extractive", () => {
    const mode = (o: object) => parseMeta({ version: "v1", rfcs: [], ...o }).mode;
    expect(mode({ mode: "extractive", model: "" })).toBe("extractive");
    expect(mode({ mode: "model", model: "m-1" })).toBe("model");
    expect(mode({ mode: "replay", model: "m-1" })).toBe("replay");
    expect(mode({ mode: "future", model: "m-1" })).toBe("model");
    expect(mode({})).toBe("extractive");
  });
});

describe("errorFromBody", () => {
  it("prefers the server's code and falls back to the status", () => {
    expect(errorFromBody({ error: { code: "not_found", message: "no such RFC" } }, 404)).toEqual(
      new ApiError("not_found", "no such RFC"),
    );
    expect(errorFromBody(null, 415).code).toBe("bad_request");
    expect(errorFromBody(null, 405).code).toBe("bad_request");
    expect(errorFromBody(null, 500).code).toBe("internal");
  });
});

describe("askStream", () => {
  it("posts the question and yields events across split chunks, including multi-byte text", async () => {
    const fetchMock = vi.fn(() =>
      Promise.resolve(
        streamResponse([
          'event: claim\ndata: {"id":0,"text":"Ref § and ¶","ref":"","quote":""}\n',
          "\nevent: abst",
          'ain\ndata: {"code":"no-passages","reason":"none"}\n\n',
        ]),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);
    const events = await collect("hello?");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/ask",
      expect.objectContaining({ method: "POST", body: '{"question":"hello?"}' }),
    );
    expect(events).toEqual([
      { type: "claim", data: { id: 0, text: "Ref § and ¶", ref: "", quote: "" } },
      { type: "abstain", data: { code: "no-passages", reason: "none" } },
    ]);
  });

  it("throws ApiError from a non-200 JSON body", async () => {
    vi.stubGlobal("fetch", () =>
      Promise.resolve(
        new Response(JSON.stringify({ error: { code: "rate_limited", message: "slow" } }), {
          status: 429,
        }),
      ),
    );
    await expect(collect("q")).rejects.toEqual(new ApiError("rate_limited", "slow"));
  });

  it("falls back to the HTTP status when the error body is not JSON", async () => {
    vi.stubGlobal("fetch", () => Promise.resolve(new Response("oops", { status: 503 })));
    await expect(collect("q")).rejects.toMatchObject({ code: "unavailable" });
  });
});
