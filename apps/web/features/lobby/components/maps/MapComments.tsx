import { ChevronDown, Heart, MessageCircle, MoreVertical, Trash2 } from "lucide-react";
import AvatarBadge from "../../../../components/ui/AvatarBadge";
import PlayerProfileLink from "../../../../components/ui/PlayerProfileLink";
import type { MapDetails } from "../../../maps/lib/maps-client";
import {
  commentAvatarFallback,
  commentDeletedLabel,
  formatCommentAge,
} from "../../lib/lobby-ui";
import { LobbyActionButton, LobbyPanel } from "../lobby-primitives";

export type MapCommentsProps = {
  accessToken: string;
  canInteractWithMaps: boolean;
  commentBody: string;
  commentComposerFocused: boolean;
  comments: MapDetails["comments"];
  createCommentPending: boolean;
  displayName: string;
  expandedCommentIds: Record<string, boolean>;
  isAdmin: boolean;
  isModerator: boolean;
  onCancelComment: () => void;
  onDeleteComment: (commentId: string) => void;
  onPostComment: () => void;
  onPostReply: (commentId: string) => void;
  onSetCommentBody: (body: string) => void;
  onSetCommentComposerFocused: (focused: boolean) => void;
  onSetOpenCommentMenuId: (commentId: string) => void;
  onSetReplyBody: (body: string) => void;
  onSetReplyToCommentId: (commentId: string) => void;
  onToggleCommentLike: (commentId: string, liked: boolean) => void;
  onToggleCommentReplies: (commentId: string) => void;
  openCommentMenuId: string;
  replyBody: string;
  replyToCommentId: string;
  userAvatar?: string;
  userAvatarFallback: string;
  userEmail: string;
};

export function MapComments(props: MapCommentsProps) {
  return (
    <LobbyPanel variant="subtle" className="p-4">
      <h4 className="flex items-center gap-2 text-lg font-extrabold tracking-tight text-white">
        <MessageCircle size={18} /> Comments
      </h4>
      {props.canInteractWithMaps ? <CommentComposer {...props} /> : (
        <p className="mt-3 text-xs text-slate-400">
          {props.accessToken ? "Upgrade your guest profile to comment." : "Sign in to comment."}
        </p>
      )}
      <div className="mt-7 grid gap-6">
        {props.comments.map((comment) => (
          <CommentThread key={comment.id} comment={comment} depth="root" {...props} />
        ))}
      </div>
    </LobbyPanel>
  );
}

function CommentComposer(props: MapCommentsProps) {
  return (
    <div className="mt-5 flex gap-3">
      <CommentAvatar props={props} className="mt-1 h-10 w-10" />
      <div className="min-w-0 flex-1">
        <textarea
          value={props.commentBody}
          onFocus={() => props.onSetCommentComposerFocused(true)}
          onChange={(event) => props.onSetCommentBody(event.target.value)}
          maxLength={1000}
          placeholder="Add a comment"
          rows={props.commentComposerFocused || props.commentBody ? 2 : 1}
          className="min-h-9 w-full resize-none border-0 border-b border-white/25 bg-transparent px-0 py-1.5 text-[15px] font-medium text-white outline-none placeholder:text-slate-400 focus:border-accentPrimary"
        />
        {props.commentComposerFocused || props.commentBody ? (
          <div className="mt-3 flex justify-end gap-2">
            <LobbyActionButton type="button" variant="ghost" size="sm" onClick={props.onCancelComment}>Cancel</LobbyActionButton>
            <LobbyActionButton type="button" size="sm" disabled={!props.commentBody.trim() || props.createCommentPending} onClick={props.onPostComment}>Comment</LobbyActionButton>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function CommentAvatar({ props, className }: { props: MapCommentsProps; className: string }) {
  return (
    <AvatarBadge
      avatarUrl={props.userAvatar}
      fallback={props.userAvatarFallback}
      alt={props.displayName || props.userEmail || "You"}
      size="sm"
      className={`${className} shrink-0 border-white/15 bg-slate-900`}
    />
  );
}

function CommentThread({
  comment,
  depth,
  ...props
}: MapCommentsProps & {
  comment: MapDetails["comments"][number];
  depth: "root" | "reply";
}) {
  const root = depth === "root";
  return (
    <div className="flex gap-3">
      <PlayerProfileLink userId={comment.userId} nickname={comment.userDisplayName} className="shrink-0">
        <AvatarBadge
          avatarUrl={comment.avatarUrl}
          fallback={commentAvatarFallback(comment.userDisplayName)}
          alt={comment.userDisplayName}
          size="sm"
          className={`${root ? "h-10 w-10" : "h-8 w-8"} shrink-0 border-white/15 bg-slate-900 text-xs`}
        />
      </PlayerProfileLink>
      <div className="min-w-0 flex-1">
        <CommentHeader comment={comment} depth={depth} {...props} />
        <div className="mt-3 flex items-center gap-4">
          {props.canInteractWithMaps && comment.status === "visible" ? (
            <button
              type="button"
              onClick={() => props.onToggleCommentLike(comment.id, !comment.liked)}
              aria-label={comment.liked ? "Unlike comment" : "Like comment"}
              aria-pressed={comment.liked}
              className={`inline-flex items-center gap-2 rounded-full text-[13px] font-bold transition ${comment.liked ? "text-accentPrimary" : "text-slate-200 hover:text-white"}`}
            >
              <Heart size={root ? 18 : 16} fill={comment.liked ? "currentColor" : "none"} />
              {comment.likeCount.toLocaleString()}
            </button>
          ) : null}
          {root && props.canInteractWithMaps && comment.status === "visible" ? (
            <button type="button" onClick={() => { props.onSetReplyToCommentId(comment.id); props.onSetReplyBody(""); }} className="rounded-full px-2 py-1 text-[13px] font-extrabold text-white hover:bg-white/10">Reply</button>
          ) : null}
        </div>
        {root && props.replyToCommentId === comment.id ? <ReplyComposer comment={comment} {...props} /> : null}
        {root && comment.replies?.length ? (
          <button type="button" onClick={() => props.onToggleCommentReplies(comment.id)} className="mt-3 inline-flex items-center gap-2 rounded-full px-2 py-1 text-sm font-extrabold text-accentPrimary hover:bg-accentPrimary/10">
            <ChevronDown size={18} className={`transition ${props.expandedCommentIds[comment.id] ? "rotate-180" : ""}`} />
            {comment.replies.length} {comment.replies.length === 1 ? "reply" : "replies"}
          </button>
        ) : null}
        {root && props.expandedCommentIds[comment.id] ? (
          <div className="mt-4 grid gap-4 border-l border-white/10 pl-4">
            {comment.replies?.map((reply) => (
              <CommentThread key={reply.id} comment={reply} depth="reply" {...props} />
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function CommentHeader({
  comment,
  depth,
  ...props
}: MapCommentsProps & {
  comment: MapDetails["comments"][number];
  depth: "root" | "reply";
}) {
  const canDelete = comment.status === "visible" && (comment.canDelete || props.isAdmin || props.isModerator) && props.accessToken;
  return (
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <div className="flex flex-wrap items-baseline gap-2">
          <PlayerProfileLink userId={comment.userId} nickname={comment.userDisplayName} className="truncate text-sm font-extrabold text-white hover:text-emerald-200">{comment.userDisplayName}</PlayerProfileLink>
          <time dateTime={comment.createdAt} className="text-[13px] font-bold text-inkMuted">{formatCommentAge(comment.createdAt)}</time>
          {commentDeletedLabel(comment.status) ? <span className="text-[13px] font-black text-red-300">{commentDeletedLabel(comment.status)}</span> : null}
        </div>
        <p className={`mt-1 font-medium text-slate-100 ${depth === "root" ? "text-[15px] leading-6" : "text-sm leading-5"}`}>{comment.body}</p>
      </div>
      {canDelete ? <CommentActions commentId={comment.id} depth={depth} {...props} /> : null}
    </div>
  );
}

function CommentActions({ commentId, depth, ...props }: MapCommentsProps & { commentId: string; depth: "root" | "reply" }) {
  if (depth === "reply") {
    return (
      <button type="button" onClick={() => props.onDeleteComment(commentId)} className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-inkMuted hover:bg-red-400/10 hover:text-red-200" aria-label="Delete reply">
        <Trash2 size={14} />
      </button>
    );
  }
  return (
    <div className="relative shrink-0">
      <button type="button" onClick={() => props.onSetOpenCommentMenuId(props.openCommentMenuId === commentId ? "" : commentId)} className="flex h-8 w-8 items-center justify-center rounded-full text-inkMuted hover:bg-white/10 hover:text-white" aria-label="Comment actions">
        <MoreVertical size={17} />
      </button>
      {props.openCommentMenuId === commentId ? (
        <div className="absolute right-0 top-9 z-10 w-32 overflow-hidden rounded-xl border border-white/10 bg-slate-950 py-1 shadow-xl">
          <button type="button" onClick={() => { props.onSetOpenCommentMenuId(""); props.onDeleteComment(commentId); }} className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs font-bold text-red-200 hover:bg-red-400/10">
            <Trash2 size={14} /> Delete
          </button>
        </div>
      ) : null}
    </div>
  );
}

function ReplyComposer({ comment, ...props }: MapCommentsProps & { comment: MapDetails["comments"][number] }) {
  return (
    <div className="mt-4 flex gap-3">
      <CommentAvatar props={props} className="mt-1 h-8 w-8" />
      <div className="min-w-0 flex-1">
        <textarea value={props.replyBody} onChange={(event) => props.onSetReplyBody(event.target.value)} maxLength={1000} autoFocus rows={2} placeholder={`Reply to ${comment.userDisplayName}`} className="min-h-11 w-full resize-none border-0 border-b border-white/25 bg-transparent px-0 py-1.5 text-sm font-medium text-white outline-none placeholder:text-slate-400 focus:border-accentPrimary" />
        <div className="mt-3 flex justify-end gap-2">
          <LobbyActionButton type="button" variant="ghost" size="sm" onClick={() => { props.onSetReplyToCommentId(""); props.onSetReplyBody(""); }}>Cancel</LobbyActionButton>
          <LobbyActionButton type="button" size="sm" disabled={!props.replyBody.trim()} onClick={() => props.onPostReply(comment.id)}>Reply</LobbyActionButton>
        </div>
      </div>
    </div>
  );
}
