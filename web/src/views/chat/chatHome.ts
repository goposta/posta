import type { ChatHome, ChatHomeItem, ChatHomeItemKind } from "../../api/types";

export const MAX_HOME_ITEMS = 12;
export const MAX_HELP_LINKS = 30;
const COLOR = /^#[0-9a-fA-F]{6}$/;
const URL_SCHEME = /^(https?:\/\/[^\s/]+|mailto:\S+|tel:\S+)/i;

export function newID(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(4));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

export function newItem(kind: ChatHomeItemKind): ChatHomeItem {
  return { id: newID(), kind, title: kind === "message" ? "Send us a message" : "", url: "", body: "", link_label: "", new_tab: true };
}

// An empty URL is valid here; whether one is required depends on the item kind.
export function urlError(url?: string): string {
  const u = (url ?? "").trim();
  if (!u) return "";
  if (u.length > 2000) return "At most 2000 characters.";
  return URL_SCHEME.test(u) ? "" : "Use an https://, http://, mailto: or tel: URL.";
}

export function homeError(home: ChatHome): string {
  if (home.header_color && !COLOR.test(home.header_color)) return "Header colour must look like #0f172a.";
  const items = home.items ?? [];
  if (items.length > MAX_HOME_ITEMS) return `The home screen can have at most ${MAX_HOME_ITEMS} items.`;
  if (items.filter((i) => i.kind === "message").length > 1) return "The home screen can have only one message item.";
  for (const [n, it] of items.entries()) {
    if (it.kind === "message") continue;
    if (!it.title.trim()) return `Home item ${n + 1} needs a title.`;
    if (it.kind === "link" && !(it.url ?? "").trim()) return `Home item ${n + 1} needs a URL.`;
    if (urlError(it.url)) return `Home item ${n + 1}: ${urlError(it.url)}`;
  }
  const links = home.help_links ?? [];
  if (links.length > MAX_HELP_LINKS) return `The Help tab can have at most ${MAX_HELP_LINKS} links.`;
  for (const [n, l] of links.entries()) {
    if (!l.title.trim() || !l.url.trim()) return `Help link ${n + 1} needs a title and a URL.`;
    if (urlError(l.url)) return `Help link ${n + 1}: ${urlError(l.url)}`;
  }
  return "";
}

export function emptyHome(): ChatHome {
  return { enabled: false, header_color: "", headline: "", items: [], help_title: "", help_links: [] };
}

export function cloneHome(home: ChatHome | null | undefined): ChatHome {
  const h = home ?? emptyHome();
  return {
    enabled: !!h.enabled,
    header_color: h.header_color ?? "",
    headline: h.headline ?? "",
    items: (h.items ?? []).map((i) => ({ ...i, body: i.body ?? "", url: i.url ?? "", link_label: i.link_label ?? "" })),
    help_title: h.help_title ?? "",
    help_links: (h.help_links ?? []).map((l) => ({ ...l, description: l.description ?? "" })),
  };
}

export function move<T>(list: T[], from: number, to: number) {
  if (to < 0 || to >= list.length) return;
  const [x] = list.splice(from, 1);
  list.splice(to, 0, x);
}
