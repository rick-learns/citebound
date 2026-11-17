/** One dispatched server-sent event. */
export interface SseMessage {
  event: string;
  data: string;
  id: string;
}

/**
 * Incremental parser for the text/event-stream format (WHATWG HTML, "Server-sent events").
 *
 * Feed decoded text with push(); each call returns the events completed by that chunk.
 * Chunks may be split anywhere, including between the CR and LF of a CRLF pair.
 * Per the spec, an event that is not terminated by a blank line before the stream
 * ends is discarded, so there is nothing to flush at end of stream.
 */
export class SseParser {
  private buffer = "";
  private started = false;
  // The previous chunk ended in CR; a leading LF in the next chunk belongs to it.
  private pendingCR = false;
  private data: string[] = [];
  private eventType = "";
  private lastId = "";

  push(chunk: string): SseMessage[] {
    if (chunk === "") return [];
    let text = chunk;
    if (!this.started) {
      this.started = true;
      if (text.startsWith("﻿")) text = text.slice(1);
    }
    if (this.pendingCR && text.startsWith("\n")) text = text.slice(1);
    this.pendingCR = false;

    this.buffer += text;
    const out: SseMessage[] = [];
    let start = 0;
    for (let i = 0; i < this.buffer.length; i++) {
      const c = this.buffer[i];
      if (c !== "\n" && c !== "\r") continue;
      const line = this.buffer.slice(start, i);
      if (c === "\r") {
        if (i + 1 === this.buffer.length) this.pendingCR = true;
        else if (this.buffer[i + 1] === "\n") i++;
      }
      start = i + 1;
      const msg = this.processLine(line);
      if (msg) out.push(msg);
    }
    this.buffer = this.buffer.slice(start);
    return out;
  }

  private processLine(line: string): SseMessage | null {
    if (line === "") return this.dispatch();
    if (line.startsWith(":")) return null; // comment / keep-alive

    const colon = line.indexOf(":");
    const field = colon < 0 ? line : line.slice(0, colon);
    let value = colon < 0 ? "" : line.slice(colon + 1);
    if (value.startsWith(" ")) value = value.slice(1);

    switch (field) {
      case "event":
        this.eventType = value;
        break;
      case "data":
        this.data.push(value);
        break;
      case "id":
        if (!value.includes("\0")) this.lastId = value;
        break;
      default:
        break; // "retry" and unknown fields are ignored
    }
    return null;
  }

  private dispatch(): SseMessage | null {
    const data = this.data;
    const event = this.eventType || "message";
    this.data = [];
    this.eventType = "";
    if (data.length === 0) return null;
    return { event, data: data.join("\n"), id: this.lastId };
  }
}
