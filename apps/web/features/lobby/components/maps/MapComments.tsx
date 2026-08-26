import { ChevronDown, Heart, MessageCircle, MoreVertical, Trash2 } from "lucide-react";
import AvatarBadge from "../../../players/components/AvatarBadge";
import PlayerProfileLink from "../../../players/components/PlayerProfileLink";
import type { MapDetails } from "../../../maps/lib/maps-client";
import {
  commentAvatarFallback,
  commentDeletedLabel,
  formatCommentAge,
} from "../../lib/lobby-ui";
import { Button, IconButton } from "../../../../components/ui/button";
import { Textarea } from "../../../../components/ui/textarea";
import { SectionCard } from "../../../../components/ui/compositions";

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
    <SectionCard className="rounded-2xl p-4">
      <h4 className="flex items-center gap-2 text-heading-sm font-strong tracking-heading text-content-primary">
        <MessageCircle size={18} /> Comments
      </h4>
      {props.canInteractWithMaps ? <CommentComposer {...props} /> : (
        <p className="mt-3 text-body-sm text-content-secondary">
          Sign in to comment
        </p>
      )}
      <div className="mt-7 grid gap-6">
        {props.comments.map((comment) => (
          <CommentThread key={comment.id} comment={comment} depth="root" {...props} />
        ))}
      </div>
    </SectionCard>
  );
}

function CommentComposer(props: MapCommentsProps) {
  return (
    <div className="mt-5 flex gap-3">
      <CommentAvatar props={props} className="mt-1 h-10 w-10" />
      <div className="min-w-0 flex-1">
        <Textarea
          value={props.commentBody}
          onFocus={() => props.onSetCommentComposerFocused(true)}
          onChange={(event) => props.onSetCommentBody(event.target.value)}
          maxLength={1000}
          placeholder="Add a comment"
          rows={props.commentComposerFocused || props.commentBody ? 2 : 1}
          className="min-h-9 w-full resize-none border-0 border-b border-border-default bg-transparent px-0 py-1.5 text-body-sm font-medium text-content-primary outline-none placeholder:text-content-secondary focus:border-border-focus"
        />
        {props.commentComposerFocused || props.commentBody ? (
          <div className="mt-3 flex justify-end gap-2">
            <Button type="button" variant="ghost" size="sm" onClick={props.onCancelComment}>Cancel</Button>
            <Button type="button" variant="primary" size="sm" disabled={!props.commentBody.trim() || props.createCommentPending} onClick={props.onPostComment}>Comment</Button>
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
      className={`${className} shrink-0 border-border-default bg-surface-inset`}
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
          className={`${root ? "h-10 w-10 text-label" : "h-8 w-8 text-caption"} shrink-0 border-border-default bg-surface-inset`}
        />
      </PlayerProfileLink>
      <div className="min-w-0 flex-1">
        <CommentHeader comment={comment} depth={depth} {...props} />
        <div className="mt-3 flex items-center gap-4">
          {props.canInteractWithMaps && comment.status === "visible" ? (
            <Button
              variant="ghost"
              type="button"
              onClick={() => props.onToggleCommentLike(comment.id, !comment.liked)}
              aria-label={comment.liked ? "Unlike comment" : "Like comment"}
              aria-pressed={comment.liked}
              className={`inline-flex items-center gap-2 rounded-full text-label font-strong transition ${comment.liked ? "text-status-success" : "text-content-secondary hover:text-content-primary"}`}
            >
              <Heart size={root ? 18 : 16} fill={comment.liked ? "currentColor" : "none"} />
              {comment.likeCount.toLocaleString()}
            </Button>
          ) : null}
          {root && props.canInteractWithMaps && comment.status === "visible" ? (
            <Button type="button" variant="ghost" size="sm" onClick={() => { props.onSetReplyToCommentId(comment.id); props.onSetReplyBody(""); }}>Reply</Button>
          ) : null}
        </div>
        {root && props.replyToCommentId === comment.id ? <ReplyComposer comment={comment} {...props} /> : null}
        {root && comment.replies?.length ? (
          <Button type="button" variant="ghost" size="sm" onClick={() => props.onToggleCommentReplies(comment.id)} className="mt-3 text-status-success">
            <ChevronDown size={18} className={`transition ${props.expandedCommentIds[comment.id] ? "rotate-180" : ""}`} />
            {comment.replies.length} {comment.replies.length === 1 ? "reply" : "replies"}
          </Button>
        ) : null}
        {root && props.expandedCommentIds[comment.id] ? (
          <div className="mt-4 grid gap-4 border-l border-border-default pl-4">
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
          <PlayerProfileLink userId={comment.userId} nickname={comment.userDisplayName} className="truncate text-body-sm font-strong text-content-primary hover:text-status-success">{comment.userDisplayName}</PlayerProfileLink>
          <time dateTime={comment.createdAt} className="text-caption font-strong text-content-secondary">{formatCommentAge(comment.createdAt)}</time>
          {commentDeletedLabel(comment.status) ? <span className="text-label font-strong text-status-danger">{commentDeletedLabel(comment.status)}</span> : null}
        </div>
        <p className={`mt-1 text-content-primary font-medium ${depth === "root" ? "text-body leading-body" : "text-body-sm leading-label"}`}>{comment.body}</p>
      </div>
      {canDelete ? <CommentActions commentId={comment.id} depth={depth} {...props} /> : null}
    </div>
  );
}

function CommentActions({ commentId, depth, ...props }: MapCommentsProps & { commentId: string; depth: "root" | "reply" }) {
  if (depth === "reply") {
    return (
      <IconButton onClick={() => props.onDeleteComment(commentId)} className="h-8 min-h-8 w-8 shrink-0 text-status-danger" aria-label="Delete reply">
        <Trash2 size={14} />
      </IconButton>
    );
  }
  return (
    <div className="relative shrink-0">
      <IconButton onClick={() => props.onSetOpenCommentMenuId(props.openCommentMenuId === commentId ? "" : commentId)} className="h-8 min-h-8 w-8" aria-label="Comment actions">
        <MoreVertical size={17} />
      </IconButton>
      {props.openCommentMenuId === commentId ? (
        <div className="absolute right-0 top-9 z-content w-32 overflow-hidden rounded-lg border border-border-default bg-surface-inset py-1 shadow-elev-3">
          <Button type="button" variant="ghost" onClick={() => { props.onSetOpenCommentMenuId(""); props.onDeleteComment(commentId); }} className="flex w-full items-center gap-2 px-3 py-2 text-left text-label font-strong text-status-danger hover:bg-status-danger/10">
            <Trash2 size={14} /> Delete
          </Button>
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
        <Textarea value={props.replyBody} onChange={(event) => props.onSetReplyBody(event.target.value)} maxLength={1000} autoFocus rows={2} placeholder={`Reply to ${comment.userDisplayName}`} className="min-h-11 w-full resize-none border-0 border-b border-border-default bg-transparent px-0 py-1.5 text-body-sm font-medium text-content-primary outline-none placeholder:text-content-secondary focus:border-border-focus" />
        <div className="mt-3 flex justify-end gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={() => { props.onSetReplyToCommentId(""); props.onSetReplyBody(""); }}>Cancel</Button>
          <Button type="button" variant="primary" size="sm" disabled={!props.replyBody.trim()} onClick={() => props.onPostReply(comment.id)}>Reply</Button>
        </div>
      </div>
    </div>
  );
}
