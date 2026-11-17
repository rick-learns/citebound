import { describe, expect, it } from "vitest";
import { SseParser, type SseMessage } from "./sse";

/** Feeds chunks in order and returns every dispatched event. */
function parse(...chunks: string[]): SseMessage[] {
  const p = new SseParser();
  return chunks.flatMap((c) => p.push(c));
}

const STREAM = 'event: claim\ndata: {"id":0}\n\nevent: verdict\ndata: {"id":0}\n\n';
const EXPECTED = [
  { event: "claim", data: '{"id":0}', id: "" },
  { event: "verdict", data: '{"id":0}', id: "" },
];

describe("SseParser", () => {
  it("parses whole events", () => {
    expect(parse(STREAM)).toEqual(EXPECTED);
  });

  it("handles a chunk boundary at every possible offset", () => {
    for (let i = 0; i <= STREAM.length; i++) {
      expect(parse(STREAM.slice(0, i), STREAM.slice(i))).toEqual(EXPECTED);
    }
  });

  it("handles one character per chunk", () => {
    expect(parse(...STREAM.split(""))).toEqual(EXPECTED);
  });

  it("accepts CRLF and lone CR line endings", () => {
    expect(parse(STREAM.replaceAll("\n", "\r\n"))).toEqual(EXPECTED);
    expect(parse(STREAM.replaceAll("\n", "\r"))).toEqual(EXPECTED);
  });

  it("treats a CRLF split across chunks as one line ending", () => {
    // If the trailing CR and leading LF were counted separately, this would dispatch early
    // with an empty line between "data: a" and "data: b".
    expect(parse("data: a\r", "\ndata: b\r\n\r\n")).toEqual([
      { event: "message", data: "a\nb", id: "" },
    ]);
  });

  it("joins multi-line data with newlines", () => {
    expect(parse("event: x\ndata: line 1\ndata:line 2\ndata:  indented\n\n")).toEqual([
      { event: "x", data: "line 1\nline 2\n indented", id: "" },
    ]);
  });

  it("ignores comments, unknown fields and retry", () => {
    expect(parse(": keep-alive\n\n:another\nretry: 1000\nfoo: bar\ndata: ok\n\n")).toEqual([
      { event: "message", data: "ok", id: "" },
    ]);
  });

  it("does not dispatch events without data", () => {
    expect(parse("event: ping\n\ndata: x\n\n")).toEqual([{ event: "message", data: "x", id: "" }]);
  });

  it("dispatches an empty data field as an empty string", () => {
    expect(parse("data\n\n")).toEqual([{ event: "message", data: "", id: "" }]);
  });

  it("tracks the last event id", () => {
    expect(parse("id: 7\ndata: a\n\ndata: b\n\n").map((m) => m.id)).toEqual(["7", "7"]);
  });

  it("strips a leading byte order mark", () => {
    expect(parse("﻿data: a\n\n")).toEqual([{ event: "message", data: "a", id: "" }]);
  });

  it("discards a trailing event that has no terminating blank line (per spec)", () => {
    expect(parse("data: a\n\ndata: incomplete\n")).toEqual([
      { event: "message", data: "a", id: "" },
    ]);
    expect(parse("data: no newline at all")).toEqual([]);
  });
});
