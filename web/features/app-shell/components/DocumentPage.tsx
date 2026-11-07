import type { ReactNode } from "react";
import { DocumentPanel } from "../../../components/ui/compositions";
import { BodyText, Eyebrow, SectionTitle } from "../../../components/ui/typography";
import { AppContentRail } from "./AppContentRail";

/** A page of reading, like a policy or a changelog. */
export function DocumentPage({
  title,
  eyebrow,
  description,
  children,
}: {
  title?: string;
  eyebrow?: string;
  description?: ReactNode;
  children: ReactNode;
}) {
  return (
    <AppContentRail as="main" className="relative z-content max-w-4xl pb-28 pt-4 sm:pb-12 sm:pt-8">
      <DocumentPanel className="p-6 sm:p-10">
        {title || eyebrow || description ? (
          <div className="border-b border-border-default pb-8">
            {eyebrow ? <Eyebrow>{eyebrow}</Eyebrow> : null}
            {title ? <SectionTitle className={eyebrow ? "mt-2" : ""}>{title}</SectionTitle> : null}
            {description ? <BodyText className="mt-5 max-w-3xl">{description}</BodyText> : null}
          </div>
        ) : null}
        {children}
      </DocumentPanel>
    </AppContentRail>
  );
}
