import { AnimatePresence, motion } from 'framer-motion';
import { MessageCircle, Send, VolumeX, X } from 'lucide-react';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useHotkey } from '../../hotkeys/hooks/use-hotkey';
import { useOptionalHotkeys } from '../../hotkeys/components/HotkeyProvider';
import { isChatMuted, setChatMuted } from '../lib/chat-preferences';
import type { ChatAudience, ChatEmote, ChatMessage } from '../model/types';
import { Select } from '../../../components/ui/select';
import { Button } from '../../../components/ui/button';
import { Input } from '../../../components/ui/input';
import { getTeamPresentation } from '../../../lib/team-presentation';
import PlayerProfileLink from '../../players/components/PlayerProfileLink';

const chatEmotes: Array<{ emote: ChatEmote; label: string; glyph: string }> = [
  { emote: 'skull', label: 'Skull', glyph: '💀' },
  { emote: 'sob', label: 'Sob', glyph: '😭' },
  { emote: 'thinking', label: 'Thinking', glyph: '🤔' },
  { emote: 'sunglasses', label: 'Sunglasses', glyph: '😎' },
  { emote: 'wave', label: 'Wave', glyph: '👋' }
];

function emoteGlyph(emote?: ChatEmote) {
  return chatEmotes.find((item) => item.emote === emote)?.glyph || '';
}

function teamChatLabel(teamId?: string) {
  return getTeamPresentation(teamId).name.replace(/^Team\s+/, '');
}

export default function ChatPanel({
  messages,
  selfUserId,
  teamId = "",
  onSendMessage,
  onSendEmote,
  mode = "interactive",
  className = "absolute left-3 top-24 z-game-controls w-[min(calc(100vw-1.5rem),21rem)] md:left-4 md:top-28",
}: {
  messages: ChatMessage[];
  selfUserId: string;
  teamId?: string;
  onSendMessage?: (body: string, audience?: ChatAudience) => boolean;
  onSendEmote?: (emote: ChatEmote, audience?: ChatAudience) => boolean;
  mode?: "interactive" | "review";
  className?: string;
}) {
  const reviewMode = mode === "review";
  const hotkeys = useOptionalHotkeys();
  const [legacyMuted, setLegacyMuted] = useState(() => isChatMuted());
  const [open, setOpen] = useState(false);
  const muted = hotkeys?.preferences.audioMuted ?? legacyMuted;
  const [body, setBody] = useState('');
  const [audience, setAudience] = useState<ChatAudience>('all');
  const [previewMessage, setPreviewMessage] = useState<ChatMessage | null>(null);
  const [previewVisible, setPreviewVisible] = useState(false);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const initialMessagesSeenRef = useRef(false);
  const latestMessageIdRef = useRef<string | null>(null);
  const previewTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!open && !reviewMode) return;
    const node = scrollRef.current;
    if (node) node.scrollTop = node.scrollHeight;
  }, [messages, open, reviewMode]);

  useEffect(() => {
    if (!open || reviewMode) return;
    requestAnimationFrame(() => {
      inputRef.current?.focus();
      inputRef.current?.select();
    });
  }, [open, reviewMode]);

  useEffect(() => {
    if (reviewMode) return;
    const latestMessage = messages[messages.length - 1];
    if (!initialMessagesSeenRef.current) {
      initialMessagesSeenRef.current = true;
      latestMessageIdRef.current = latestMessage?.id ?? null;
      return;
    }

    if (!latestMessage) {
      latestMessageIdRef.current = null;
      return;
    }

    if (latestMessageIdRef.current === latestMessage.id) return;
    latestMessageIdRef.current = latestMessage.id;

    if (latestMessage.senderUserId === selfUserId || muted) return;

    if (previewTimerRef.current) clearTimeout(previewTimerRef.current);
    setPreviewMessage(latestMessage);
    setPreviewVisible(true);
    previewTimerRef.current = setTimeout(() => {
      setPreviewVisible(false);
      previewTimerRef.current = null;
    }, 4200);
  }, [messages, muted, reviewMode, selfUserId]);

  useEffect(() => {
    return () => {
      if (previewTimerRef.current) clearTimeout(previewTimerRef.current);
    };
  }, []);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const sent = teamId ? onSendMessage?.(body, audience) : onSendMessage?.(body);
    if (sent) setBody('');
  };

  useEffect(() => {
    if (!teamId) setAudience('all');
  }, [teamId]);

  const muteChat = () => {
    if (previewTimerRef.current) {
      clearTimeout(previewTimerRef.current);
      previewTimerRef.current = null;
    }
    setPreviewVisible(false);
    setOpen(false);
    if (hotkeys) hotkeys.setAudioMuted(true);
    else {
      setLegacyMuted(true);
      setChatMuted(true);
    }
  };

  const openChat = () => {
    if (muted) {
      if (hotkeys) hotkeys.setAudioMuted(false);
      else {
        setLegacyMuted(false);
        setChatMuted(false);
      }
    }
    setOpen(true);
  };

  useHotkey({
    action: "chat.focus",
    scope: "gameplay",
    enabled: !reviewMode,
    run: openChat,
  });

  return (
    <div className={className}>
      {open || reviewMode ? (
        <div className="relative rounded-lg border border-border-default bg-hud-surface p-3 text-content-primary">
          {!reviewMode ? (
            <>
              <Button
                variant="ghost"
                type="button"
                onClick={() => setOpen(false)}
                aria-label="Close chat"
                className="absolute right-3 top-4 z-content flex h-8 w-8 items-center justify-center rounded-full text-content-secondary transition hover:bg-surface-fill hover:text-content-primary"
              >
                <X size={15} strokeWidth={2.5} />
              </Button>
              <Button
                variant="ghost"
                type="button"
                onClick={muteChat}
                aria-label="Mute chat"
                title="Mute chat"
                className="absolute right-11 top-4 z-content flex h-8 w-8 items-center justify-center rounded-full text-content-secondary transition hover:bg-surface-fill hover:text-content-primary"
              >
                <VolumeX size={15} strokeWidth={2.5} />
              </Button>
            </>
          ) : null}
          <div
            ref={scrollRef}
            className={`scrollbar-hidden flex flex-col gap-1 overflow-y-auto text-body-sm font-semibold leading-heading drop-shadow-sm ${
              reviewMode ? "max-h-80" : "max-h-48 pr-20"
            }`}
          >
            {messages.length === 0 ? (
              <p className="py-8 text-center text-body-sm font-semibold text-content-secondary">No messages yet</p>
            ) : (
              messages.map((message) => {
                const self = message.senderUserId === selfUserId;
                return (
                  <p key={message.id} className="break-words text-content-primary">
                    {reviewMode ? (
              <span className="mr-2 font-mono text-caption font-regular text-content-secondary">
                        {new Date(message.createdAt).toLocaleString()}
                      </span>
                    ) : null}
                    {message.audience === 'team' ? (
                      <span className="mr-1 font-strong">({teamChatLabel(message.teamId)})</span>
                    ) : null}
                    <PlayerProfileLink userId={message.senderUserId} nickname={message.senderDisplayName} stopPropagation className={`mr-1 font-strong hover:underline ${self ? 'text-status-success' : 'text-brand-blue'}`}>
                      {message.senderDisplayName}
                    </PlayerProfileLink>
                    <span className={message.kind === 'emote' ? 'text-heading-sm leading-collapsed' : ''}>
                      {message.kind === 'emote' ? emoteGlyph(message.emote) : message.body}
                    </span>
                  </p>
                );
              })
            )}
          </div>
          {!reviewMode ? (
            <>
              <div className="mt-3 flex gap-2">
                {chatEmotes.map((item) => (
                  <Button
                    variant="ghost"
                    key={item.emote}
                    type="button"
                    onClick={() => teamId ? onSendEmote?.(item.emote, audience) : onSendEmote?.(item.emote)}
                    aria-label={item.label}
                    title={item.label}
                    className="flex h-10 w-10 items-center justify-center rounded-full border border-border-default bg-surface-fill text-heading-sm transition hover:bg-surface-raised"
                  >
                    {item.glyph}
                  </Button>
                ))}
              </div>
              <form onSubmit={submit} className="mt-3 flex gap-2">
                {teamId ? (
                  <Select
                    variant="game"
                    value={audience}
                    onChange={(event) => setAudience(event.target.value as ChatAudience)}
                    aria-label="Message audience"
                    className="w-[5.5rem] px-2"
                  >
                    <option value="all">All</option>
                    <option value="team">{teamChatLabel(teamId)}</option>
                  </Select>
                ) : null}
                <Input
                  variant="game"
                  ref={inputRef}
                  value={body}
                  onChange={(event) => setBody(event.target.value.slice(0, 180))}
                  maxLength={180}
                  className="min-w-0 flex-1 rounded-lg border border-border-default bg-surface-inset px-3 py-2 text-body-sm font-semibold text-content-primary outline-none placeholder:text-content-secondary focus:border-border-focus"
                  placeholder="Message"
                />
                <Button
                  variant="primary"
                  size="icon"
                  type="submit"
                  aria-label="Send message"
                  disabled={!body.trim()}
                  className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-action-primary text-content-on-action transition hover:bg-action-primary-hover disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Send size={16} strokeWidth={2.5} />
                </Button>
              </form>
            </>
          ) : null}
        </div>
      ) : (
        <Button
          variant="ghost"
          type="button"
          onClick={openChat}
          aria-label={muted ? 'Open chat and unmute' : 'Open chat'}
          className="flex h-11 max-w-[min(calc(100vw-1.5rem),19rem)] items-center gap-2 rounded-full border border-border-default bg-hud-surface px-3 text-left text-content-primary transition hover:bg-surface-raised"
        >
          {muted ? (
            <VolumeX size={17} strokeWidth={2.4} className="flex-shrink-0 text-content-secondary" />
          ) : (
            <MessageCircle size={17} strokeWidth={2.4} className="flex-shrink-0 text-content-secondary" />
          )}
          <span className="relative block min-w-0 flex-1 overflow-hidden pr-1 text-body-sm font-semibold leading-collapsed">
            <AnimatePresence mode="wait" initial={false}>
              {previewVisible && previewMessage ? (
                <motion.span
                  key={previewMessage.id}
                  initial={{ opacity: 0, y: 4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
                  transition={{ duration: 0.28, ease: 'easeOut' }}
                  className="block truncate"
                >
                  {previewMessage.kind === 'emote' ? emoteGlyph(previewMessage.emote) : previewMessage.body || ''}
                </motion.span>
              ) : (
                <motion.span
                  key="placeholder"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.2, ease: 'easeOut' }}
                  className="block truncate text-content-secondary"
                >
                  {muted ? 'Chat muted' : 'Message...'}
                </motion.span>
              )}
            </AnimatePresence>
          </span>
        </Button>
      )}
    </div>
  );
}
