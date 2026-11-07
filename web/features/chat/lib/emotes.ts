import type { ChatEmote } from "../model/types";

export const chatEmotes: Array<{ emote: ChatEmote; label: string; glyph: string }> = [
  { emote: "skull", label: "Skull", glyph: "💀" },
  { emote: "sob", label: "Sob", glyph: "😭" },
  { emote: "thinking", label: "Thinking", glyph: "🤔" },
  { emote: "sunglasses", label: "Sunglasses", glyph: "😎" },
  { emote: "wave", label: "Wave", glyph: "👋" },
];

export function emoteGlyph(emote?: ChatEmote) {
  return chatEmotes.find((item) => item.emote === emote)?.glyph || "";
}
