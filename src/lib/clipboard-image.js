export function clipboardImagePath(date = new Date(), randomValue = globalThis.crypto?.randomUUID?.() || Math.random().toString(36).slice(2)) {
  const pad = (value) => String(value).padStart(2, "0");
  const year = date.getFullYear();
  const month = pad(date.getMonth() + 1);
  const day = pad(date.getDate());
  const time = `${year}${month}${day}-${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`;
  const suffix = String(randomValue).replace(/[^a-z0-9]/gi, "").slice(0, 8) || "image";
  return `screenshots/${year}/${month}/${day}/${time}-${suffix}.png`;
}

export function clipboardImageMarkdown(url) {
  return `![screenshot](${url})`;
}
