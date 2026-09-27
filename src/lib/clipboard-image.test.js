import assert from "node:assert/strict";
import { test } from "node:test";
import { clipboardImageMarkdown, clipboardImagePath } from "./clipboard-image.js";

test("clipboard image paths use a stable date hierarchy and safe suffix", () => {
  const date = new Date(2026, 8, 26, 12, 34, 56);
  assert.equal(clipboardImagePath(date, "abc-123_DEF"), "screenshots/2026/09/26/20260926-123456-abc123DE.png");
});

test("clipboard image markdown is paste-ready", () => {
  assert.equal(clipboardImageMarkdown("https://example.com/image.png"), "![screenshot](https://example.com/image.png)");
});
