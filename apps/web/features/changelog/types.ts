export type ChangelogPost = {
  id: number;
  slug: string;
  title: string;
  markdown: string;
  published: boolean;
  createdAt: string;
  updatedAt: string;
};

export type ChangelogPostInput = {
  slug: string;
  title: string;
  markdown: string;
  published: boolean;
};
