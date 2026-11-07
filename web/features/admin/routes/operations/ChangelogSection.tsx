import dynamic from "next/dynamic";
import Link from "next/link";
import { useMutation, useQuery } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "../../../../components/ui/button";
import { Badge } from "../../../../components/ui/Badge";
import { Input } from "../../../../components/ui/input";
import { Checkbox } from "../../../../components/ui/Switch";
import { Heading } from "../../../../components/ui/typography";
import type { ChangelogPost, ChangelogPostInput } from "../../../changelog/types";
import { AdminPanel as Panel } from "../../components/admin-primitives";
import { formatAdminDate, sanitizeSlugInput, slugify } from "../../lib/admin-format";
import {
  requestAdminCreateChangelogPost,
  requestAdminGetChangelog,
  requestAdminUpdateChangelogPost,
} from "../../lib/admin-client";
import type { OperationsSectionProps } from "./types";

const SimpleMDE = dynamic(() => import("react-simplemde-editor"), {
  ssr: false,
});

const changelogMarkdownOptions = {
  autofocus: false,
  spellChecker: false,
  status: false,
  minHeight: "460px",
  previewClass: ["editor-preview", "markdown-content"],
};

export function ChangelogSection(props: OperationsSectionProps) {
  const [selectedChangelogId, setSelectedChangelogId] = useState<number | "new">("new");
  const [changelogDraft, setChangelogDraft] = useState<ChangelogPostInput>({
    slug: "",
    title: "",
    markdown: "",
    published: true,
  });

  const changelogQuery = useQuery({
    queryKey: ["admin-changelog", props.accessToken],
    enabled: props.canManageAdmin && !!props.accessToken,
    queryFn: () => requestAdminGetChangelog(props.accessToken),
  });

  useEffect(() => {
    const posts = changelogQuery.data?.posts || [];
    if (selectedChangelogId !== "new" || posts.length === 0) return;
    const latest = posts[0];
    setSelectedChangelogId(latest.id);
    setChangelogDraft({
      slug: latest.slug,
      title: latest.title,
      markdown: latest.markdown,
      published: latest.published,
    });
  }, [changelogQuery.data]);

  const selectChangelogPost = (post: ChangelogPost) => {
    setSelectedChangelogId(post.id);
    setChangelogDraft({
      slug: post.slug,
      title: post.title,
      markdown: post.markdown,
      published: post.published,
    });
  };

  const startNewChangelogPost = () => {
    setSelectedChangelogId("new");
    setChangelogDraft({
      slug: "",
      title: "",
      markdown: "",
      published: true,
    });
  };

  const saveChangelogPost = useMutation({
    mutationFn: () => {
      const content = {
        ...changelogDraft,
        slug: changelogDraft.slug || slugify(changelogDraft.title),
      };
      if (selectedChangelogId === "new") {
        return requestAdminCreateChangelogPost(props.accessToken, content);
      }
      return requestAdminUpdateChangelogPost(props.accessToken, selectedChangelogId, content);
    },
    onSuccess: async (post) => {
      setSelectedChangelogId(post.id);
      setChangelogDraft({
        slug: post.slug,
        title: post.title,
        markdown: post.markdown,
        published: post.published,
      });
      await props.refreshAdminData();
    },
  });

  if (props.leaf !== "changelog") {
    return null;
  }

  const changelogPosts = changelogQuery.data?.posts || [];
  const selectedChangelogPost =
    selectedChangelogId === "new"
      ? null
      : changelogPosts.find((post) => post.id === selectedChangelogId) || null;

  return (
    <Panel className="p-4 xl:col-span-2">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <Heading as="h3" variant="heading-sm">Changelog</Heading>
          <p className="mt-1 text-body-sm text-content-secondary">
            Write release notes as Markdown. Saving a post updates its modified date automatically.
          </p>
        </div>
        <Button onClick={startNewChangelogPost}>New Post</Button>
      </div>

      <div className="mt-5 grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
        <div className="space-y-2">
          {changelogPosts.length === 0 ? (
            <div className="rounded-md border border-border-default bg-surface-grouped p-3 text-body-sm text-content-secondary">
              No changelog posts yet.
            </div>
          ) : null}
          {changelogPosts.map((post) => {
            const selected = selectedChangelogId === post.id;
            return (
              <Button
                variant="ghost"
                key={post.id}
                type="button"
                onClick={() => selectChangelogPost(post)}
                className={`w-full rounded-md border p-3 text-left transition ${
                  selected
                    ? "border-status-success bg-status-success/10"
                    : "border-border-default bg-surface-grouped hover:border-border-strong"
                }`}
              >
                <div className="flex items-center justify-between gap-2">
                  <p className="line-clamp-2 font-strong text-content-primary">{post.title}</p>
                  <Badge tone={post.published ? "success" : "warning"}>
                    {post.published ? "Live" : "Draft"}
                  </Badge>
                </div>
                <p className="mt-1 truncate text-body-sm text-content-secondary">/{post.slug}</p>
                <p className="mt-2 text-body-sm text-content-secondary">
                  Modified {formatAdminDate(post.updatedAt)}
                </p>
              </Button>
            );
          })}
        </div>

        <div className="min-w-0 space-y-3">
          <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,0.8fr)]">
            <Input
              value={changelogDraft.title}
              onChange={(event) =>
                setChangelogDraft((draft) => ({
                  ...draft,
                  title: event.target.value,
                  slug: draft.slug || slugify(event.target.value),
                }))
              }
              placeholder="Post title"
            />
            <Input
              value={changelogDraft.slug}
              onChange={(event) =>
                setChangelogDraft((draft) => ({
                  ...draft,
                  slug: sanitizeSlugInput(event.target.value),
                }))
              }
              placeholder="url-slug"
            />
          </div>
          <div className="admin-markdown-editor overflow-hidden rounded-lg border border-border-default">
            <SimpleMDE
              value={changelogDraft.markdown}
              onChange={(value) =>
                setChangelogDraft((draft) => ({ ...draft, markdown: value || "" }))
              }
              options={changelogMarkdownOptions}
            />
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <label className="flex items-center gap-2 text-body-sm font-semibold text-content-secondary">
              <Checkbox
                type="checkbox"
                checked={changelogDraft.published}
                onChange={(event) =>
                  setChangelogDraft((draft) => ({ ...draft, published: event.target.checked }))
                }
              />
              Published
            </label>
            <div className="flex items-center gap-2">
              {selectedChangelogPost ? (
                <Link
                  href={`/changelog/${encodeURIComponent(selectedChangelogPost.slug)}`}
                  className="inline-flex items-center gap-2 rounded-md border border-border-strong px-3 py-2 text-body-sm font-semibold text-content-primary hover:border-status-success hover:text-status-success"
                >
                  View Post
                  <ExternalLink className="h-4 w-4" />
                </Link>
              ) : null}
              <Button
                disabled={!changelogDraft.title.trim() || saveChangelogPost.isPending}
                onClick={() => void saveChangelogPost.mutateAsync()}
              >
                {saveChangelogPost.isPending ? "Saving..." : selectedChangelogId === "new" ? "Create Post" : "Save Post"}
              </Button>
            </div>
          </div>
          {saveChangelogPost.error ? (
            <p className="text-body-sm font-semibold text-status-danger">
              {saveChangelogPost.error instanceof Error ? saveChangelogPost.error.message : "Failed to save changelog post"}
            </p>
          ) : null}
        </div>
      </div>
    </Panel>
  );
}
