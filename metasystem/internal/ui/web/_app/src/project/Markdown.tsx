import { ExternalLink } from "lucide-react";
import { createContext, Fragment, useContext, type ReactNode } from "react";
import { NavLink } from "react-router";

import type { Block, Cell, Inline, Item } from "./api";
import { documentIdFor, externalHref, fragmentOf } from "./links";
import { routeFor } from "../routes";
import "./reading.css";

/**
 * The document, rendered as elements.
 *
 * No HTML string exists anywhere on this path. The engine parsed the source
 * into a typed tree, and this file turns each node into a React element, which
 * React escapes. Raw HTML in the source arrives as a node whose text is the
 * source, and it is shown in a code block, captioned, rather than run: a
 * <script> tag in a design document is visible characters.
 *
 * A link becomes an anchor only when links.ts agrees with the target the
 * engine gave it. Everything else — a scheme this interface does not open, a
 * path outside the checkout, a file that is not Markdown — is mono text, so no
 * refusal is hidden behind something that looks clickable.
 */

export const HTML_CAPTION = "HTML in the source is shown, not run";

/**
 * How a run of plain text is rendered.
 *
 * A document renders its words as words, which is the default and the only
 * thing the reader ever needed. An answer renders them through a resolver that
 * turns the goals and records this workspace carries into links — still
 * elements, still built by React, still no HTML string anywhere on the path.
 */
export type RenderText = (words: string) => ReactNode;

const asWritten: RenderText = (words) => words;

const TextContext = createContext<RenderText>(asWritten);

export function Markdown({
  blocks,
  from,
  renderText,
}: {
  blocks: Block[];
  from: string;
  /** How plain text is rendered; omitted, it is rendered as written. */
  renderText?: RenderText;
}) {
  if (renderText === undefined) {
    return <Blocks blocks={blocks} from={from} />;
  }
  return (
    <TextContext.Provider value={renderText}>
      <Blocks blocks={blocks} from={from} />
    </TextContext.Provider>
  );
}

function Blocks({ blocks, from }: { blocks: Block[]; from: string }) {
  return (
    <>
      {blocks.map((block, index) => (
        <BlockNode key={index} block={block} from={from} />
      ))}
    </>
  );
}

function BlockNode({ block, from }: { block: Block; from: string }) {
  switch (block.type) {
    case "heading":
      return <HeadingNode block={block} from={from} />;
    case "paragraph":
      return (
        <p className="ms-md-paragraph">
          <Inlines inlines={block.inlines ?? []} from={from} />
        </p>
      );
    case "code":
      return <CodeBlock lang={block.lang ?? ""} text={block.text ?? ""} />;
    case "html":
      return <CodeBlock lang="" text={block.text ?? ""} caption={HTML_CAPTION} />;
    case "quote":
      return (
        <blockquote className="ms-md-quote">
          <Blocks blocks={block.blocks ?? []} from={from} />
        </blockquote>
      );
    case "list":
      return <ListNode block={block} from={from} />;
    case "table":
      return <TableNode block={block} from={from} />;
    case "rule":
      return <hr className="ms-md-rule" />;
    default:
      return null;
  }
}

function HeadingNode({ block, from }: { block: Block; from: string }) {
  const children = <Inlines inlines={block.inlines ?? []} from={from} />;
  const id = block.id ?? "";
  switch (block.level ?? 1) {
    case 1:
      return <h1 className="ms-md-h1" id={id}>{children}</h1>;
    case 2:
      return <h2 className="ms-md-h2" id={id}>{children}</h2>;
    case 3:
      return <h3 className="ms-md-h3" id={id}>{children}</h3>;
    case 4:
      return <h4 className="ms-md-h4" id={id}>{children}</h4>;
    case 5:
      return <h5 className="ms-md-h4" id={id}>{children}</h5>;
    default:
      return <h6 className="ms-md-h4" id={id}>{children}</h6>;
  }
}

function CodeBlock({ lang, text, caption }: { lang: string; text: string; caption?: string }) {
  return (
    <div className="ms-md-code">
      {caption !== undefined && <p className="ms-md-code-caption">{caption}</p>}
      {caption === undefined && lang !== "" && <p className="ms-md-code-language">{lang}</p>}
      <pre className="ms-md-pre">
        <code>{text}</code>
      </pre>
    </div>
  );
}

function ListNode({ block, from }: { block: Block; from: string }) {
  const items = block.items ?? [];
  const children = items.map((item, index) => <ItemNode key={index} item={item} from={from} />);
  if (block.ordered === true) {
    return (
      <ol className="ms-md-list" start={block.start ?? 1}>
        {children}
      </ol>
    );
  }
  return <ul className="ms-md-list">{children}</ul>;
}

function ItemNode({ item, from }: { item: Item; from: string }) {
  return (
    <li className="ms-md-item">
      {item.checked !== null && (
        <input className="ms-md-check" type="checkbox" checked={item.checked} disabled readOnly />
      )}
      <Blocks blocks={item.blocks} from={from} />
    </li>
  );
}

function TableNode({ block, from }: { block: Block; from: string }) {
  const align = block.align ?? [];
  const head = block.head ?? [];
  const rows = block.rows ?? [];
  return (
    <div className="ms-md-table-scroll">
      <table className="ms-md-table">
        {head.length > 0 && (
          <thead>
            <tr>
              {head.map((cell, column) => (
                <th key={column} className={alignmentClass(align[column])}>
                  <Inlines inlines={cell} from={from} />
                </th>
              ))}
            </tr>
          </thead>
        )}
        <tbody>
          {rows.map((row, index) => (
            <tr key={index}>
              {row.map((cell: Cell, column) => (
                <td key={column} className={alignmentClass(align[column])}>
                  <Inlines inlines={cell} from={from} />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function alignmentClass(alignment: string | undefined): string {
  switch (alignment) {
    case "left":
      return "ms-md-left";
    case "center":
      return "ms-md-center";
    case "right":
      return "ms-md-right";
    default:
      return "";
  }
}

function Inlines({ inlines, from }: { inlines: Inline[]; from: string }) {
  return (
    <>
      {inlines.map((inline, index) => (
        <Fragment key={index}>
          <InlineNode inline={inline} from={from} />
        </Fragment>
      ))}
    </>
  );
}

function InlineNode({ inline, from }: { inline: Inline; from: string }): ReactNode {
  const children = <Inlines inlines={inline.inlines ?? []} from={from} />;
  const renderText = useContext(TextContext);
  switch (inline.type) {
    case "text":
      return renderText(inline.text ?? "");
    case "emph":
      return <em>{children}</em>;
    case "strong":
      return <strong>{children}</strong>;
    case "strike":
      return <s>{children}</s>;
    case "code":
      // Inline code is where an answer writes an id, so it is prose for the
      // resolver's purposes: a name inside it links exactly as one outside it.
      return <code className="ms-md-inline-code">{renderText(inline.text ?? "")}</code>;
    case "break":
      return <br />;
    case "html":
      return <code className="ms-md-inline-code">{inline.text ?? ""}</code>;
    case "image":
      // No <img>: an image is named, never fetched, so a source document
      // cannot make this page ask another host for anything.
      return <span className="ms-md-image">{`[image: ${inline.alt ?? ""}] ${inline.src ?? ""}`}</span>;
    case "link":
      return (
        <LinkNode inline={inline} from={from}>
          {children}
        </LinkNode>
      );
    default:
      return null;
  }
}

function LinkNode({ inline, from, children }: { inline: Inline; from: string; children: ReactNode }) {
  const href = inline.href ?? "";
  if (inline.target === "external") {
    const safe = externalHref(href);
    if (safe === null) {
      return <span className="ms-md-unresolved">{children}</span>;
    }
    return (
      <a className="ms-md-link" href={safe} target="_blank" rel="noopener noreferrer">
        {children}
        <ExternalLink className="ms-md-external" size={12} strokeWidth={1.75} aria-hidden="true" />
      </a>
    );
  }
  if (inline.target === "fragment") {
    const fragment = fragmentOf(href);
    if (fragment === null) {
      return <span className="ms-md-unresolved">{children}</span>;
    }
    return (
      <a className="ms-md-link" href={`#${fragment}`}>
        {children}
      </a>
    );
  }
  if (inline.target === "document") {
    const id = inline.id !== undefined && inline.id !== "" ? inline.id : documentIdFor(from, href);
    const to = id === null ? null : routeFor({ kind: "document", id });
    if (to !== null) {
      return (
        <NavLink className="ms-md-link" to={to}>
          {children}
        </NavLink>
      );
    }
  }
  return <span className="ms-md-unresolved">{children}</span>;
}
