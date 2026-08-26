import { PageShell } from "../../components/ui/PageShell";

export type LegalSection = {
  title: string;
  body: string[];
};

type LegalDocumentPageProps = {
  title: string;
  updatedAt: string;
  description: string;
  sections: LegalSection[];
};

export function LegalDocumentPage({
  title,
  updatedAt,
  description,
  sections,
}: LegalDocumentPageProps) {
  return (
    <PageShell
      variant="operational"
      title={title}
      description={description}
      backHref="/"
      maxWidthClassName="max-w-4xl"
    >
      <p className="-mt-7 border-b border-border-default pb-7 text-body-sm text-content-secondary">
        Last updated: {updatedAt}
      </p>
      <div className="mt-8 space-y-8">
        {sections.map((section) => (
          <section key={section.title}>
            <h2 className="text-heading-sm font-strong text-content-primary">{section.title}</h2>
            <div className="mt-3 space-y-3 text-body leading-prose-lg text-content-secondary">
              {section.body.map((paragraph) => (
                <p key={paragraph}>{paragraph}</p>
              ))}
            </div>
          </section>
        ))}
      </div>
    </PageShell>
  );
}
