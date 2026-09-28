import { useEffect, useState } from "react";

import { loadChanges, loadDiff, loadSource, type Changes, type FileDiff, type Source } from "./api";
import { deskKey, deskLabel, deskReadKey, reviewedOf, type DeskItem } from "./room";
import { ASK_REVISION, ASK_SOURCE, ASK_SURFACE } from "../partner/AskSelection";
import { usePartner } from "../partner/store";
import { Help } from "../help/Help";
import { loadDocument, type Block, type DocumentPayload } from "../project/api";
import { Markdown } from "../project/Markdown";

/**
 * The desk (g1-s65 §3, D4): the laptop in the room. It shows one thing at a
 * time, large — a file of the candidate's tree at a range of lines with the
 * changed lines marked, the change index, one file's diff, or a record's
 * section — and a strip above it lists what has been on it, newest first.
 *
 * Everything here reads the candidate through the review owner, over the
 * record's Reviewed line, and never this checkout: the colleague and the human
 * read one tree.
 */
export function Desk({ record }: { record: string }) {
  const { room, putOnDesk, showOnDesk, table } = usePartner();
  const item = room.desk.current >= 0 ? room.desk.items[room.desk.current] : undefined;
  const reviewed = reviewedOf(table.source);
  return (
    <div className="ms-desk">
      <nav className="ms-desk-strip" aria-label="What has been on the desk">
        {room.desk.items.length === 0 ? (
          <span className="ms-desk-strip-empty">Nothing has been on the desk yet.</span>
        ) : (
          room.desk.items.map((one, at) => (
            <button
              key={deskKey(one)}
              type="button"
              className={`ms-desk-tab${at === room.desk.current ? " ms-desk-tab--up" : ""}`}
              aria-current={at === room.desk.current ? "true" : undefined}
              onClick={() => {
                showOnDesk(at);
              }}
            >
              {deskLabel(one)}
            </button>
          ))
        )}
        <button
          type="button"
          className="ms-desk-tab ms-desk-tab--changes"
          onClick={() => {
            putOnDesk({ kind: "changes" });
          }}
        >
          The change
        </button>
        <Help id="the-desk" />
      </nav>
      <div className="ms-desk-item">
        {item === undefined ? (
          <p className="ms-desk-empty">
            The desk is empty. Press The change to see every file this work touched, a file in the conversation to
            open it here, or a walk to have your Partner put things here as it explains them.
          </p>
        ) : (
          <DeskView key={deskReadKey(item, reviewed)} record={record} item={item} at={deskReadKey(item, reviewed)} />
        )}
      </div>
    </div>
  );
}

/** `at` is the read's key, the item at the reviewed commit, which every read below is made under. */
function DeskView({ record, item, at }: { record: string; item: DeskItem; at: string }) {
  switch (item.kind) {
    case "source":
      return <SourceView record={record} path={item.path} from={item.from} to={item.to} at={at} />;
    case "changes":
      return <ChangesView record={record} since={item.since === true} at={at} />;
    case "diff":
      return <DiffView record={record} path={item.path} since={item.since === true} at={at} />;
    case "section":
      return <SectionView record={item.record} section={item.section} at={at} />;
  }
}

type Read<T> = { state: "loading" } | { state: "read"; value: T } | { state: "refused"; reason: string };

/** One read of the desk, made when the item is put up and at no other time. */
function useRead<T>(load: (signal: AbortSignal) => Promise<T>, key: string): Read<T> {
  const [read, setRead] = useState<Read<T>>({ state: "loading" });
  useEffect(() => {
    const aborter = new AbortController();
    setRead({ state: "loading" });
    load(aborter.signal)
      .then((value) => {
        setRead({ state: "read", value });
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setRead({ state: "refused", reason: error instanceof Error ? error.message : String(error) });
        }
      });
    return () => {
      aborter.abort();
    };
    // The key is what the read is of; the loader is rebuilt on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  return read;
}

function Refused({ reason }: { reason: string }) {
  return (
    <p className="ms-desk-refused" role="status">
      {reason}
    </p>
  );
}

/**
 * A file of the reviewed tree at a range of lines, the lines the change touched
 * marked. The lines are a selection surface: selecting some offers Ask and
 * Finding, anchored at exactly those lines of exactly this tip (D7).
 */
function SourceView({ record, path, from, to, at }: { record: string; path: string; from: number; to: number; at: string }) {
  const { putOnDesk } = usePartner();
  const read = useRead<Source>((signal) => loadSource(record, path, from, to, signal), at);
  if (read.state === "loading") {
    return <p className="ms-desk-loading">Reading {path} from the reviewed tree…</p>;
  }
  if (read.state === "refused") {
    return <Refused reason={read.reason} />;
  }
  return <SourceShown source={read.value} put={putOnDesk} />;
}

/** A source read as the desk shows it, once it has been read. */
export function SourceShown({ source, put }: { source: Source; put: (item: DeskItem) => void }) {
  const path = source.path;
  const putOnDesk = put;
  const before = source.from > 1;
  const after = source.to < source.total;
  return (
    <section className="ms-desk-source" aria-label={`${path} at the reviewed tip`}>
      <p className="ms-desk-caption">
        <span className="ms-mono">{source.path}</span>
        <span>
          lines {source.from}–{source.to} of {source.total} · at {source.commit.slice(0, 9)}
        </span>
        <button
          type="button"
          className="ms-desk-link"
          onClick={() => {
            putOnDesk({ kind: "diff", path });
          }}
        >
          Its diff
        </button>
      </p>
      {before && (
        <button
          type="button"
          className="ms-desk-more"
          onClick={() => {
            putOnDesk({ kind: "source", path, from: Math.max(1, source.from - 200), to: source.to });
          }}
        >
          Earlier lines
        </button>
      )}
      <div
        className="ms-desk-lines"
        {...{ [ASK_SURFACE]: "desk", [ASK_SOURCE]: path, [ASK_REVISION]: source.commit }}
      >
        {source.lines.map((line) => (
          <div
            key={line.number}
            className={`ms-desk-line${line.touched === true ? " ms-desk-line--touched" : ""}`}
            data-line={line.number}
          >
            <span className="ms-desk-number" aria-hidden="true">
              {line.number}
            </span>
            <span className="ms-desk-code">{line.text === "" ? " " : line.text}</span>
          </div>
        ))}
      </div>
      {after && (
        <button
          type="button"
          className="ms-desk-more"
          onClick={() => {
            putOnDesk({ kind: "source", path, from: source.from, to: Math.min(source.total, source.to + 200) });
          }}
        >
          Later lines
        </button>
      )}
    </section>
  );
}

/**
 * The change as a whole: every file it touched with its counts, each opening
 * its diff. A done goal's index is its landed commits' own changes, each against
 * its first parent, so it is never empty for changed work (Astra S65-02).
 */
function ChangesView({ record, since, at }: { record: string; since: boolean; at: string }) {
  const { putOnDesk } = usePartner();
  const read = useRead<Changes>((signal) => loadChanges(record, since, signal), at);
  if (read.state === "loading") {
    return <p className="ms-desk-loading">Reading the change…</p>;
  }
  if (read.state === "refused") {
    return <Refused reason={read.reason} />;
  }
  return <ChangesShown changes={read.value} since={since} put={putOnDesk} />;
}

/** The change index as the desk shows it, once it has been read. */
export function ChangesShown({ changes, since, put }: { changes: Changes; since: boolean; put: (item: DeskItem) => void }) {
  const putOnDesk = put;
  const compared = changes.comparisons
    .map((pair) => `${pair.from.slice(0, 9)}…${pair.to.slice(0, 9)}`)
    .join(", ");
  return (
    <section className="ms-desk-changes" aria-label={since ? "What changed since the reviewed tip" : "The change index"}>
      <p className="ms-desk-caption">
        <span>{since ? "What changed since the tip you reviewed" : "The change"}</span>
        <span>
          {changes.total} {changes.total === 1 ? "file" : "files"} · {compared}
        </span>
      </p>
      {changes.files.length === 0 ? (
        <p className="ms-desk-empty">Nothing changed between these commits.</p>
      ) : (
        <ul className="ms-desk-files">
          {changes.files.map((file) => (
            <li key={file.path}>
              <button
                type="button"
                className="ms-desk-file"
                onClick={() => {
                  putOnDesk({ kind: "diff", path: file.path, since });
                }}
              >
                <span className="ms-mono">{file.path}</span>
                {file.binary === true ? (
                  <span className="ms-desk-count">binary</span>
                ) : (
                  <span className="ms-desk-count">
                    <span className="ms-desk-added">+{file.added}</span>{" "}
                    <span className="ms-desk-deleted">−{file.deleted}</span>
                  </span>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
      {changes.supplied < changes.total && (
        <p className="ms-desk-bound">
          {changes.supplied} of {changes.total} files are listed.
        </p>
      )}
    </section>
  );
}

/** One file's change as hunks, each line numbered on its sides and marked. */
function DiffView({ record, path, since, at }: { record: string; path: string; since: boolean; at: string }) {
  const { putOnDesk } = usePartner();
  const read = useRead<FileDiff>((signal) => loadDiff(record, path, since, signal), at);
  if (read.state === "loading") {
    return <p className="ms-desk-loading">Reading {path}'s change…</p>;
  }
  if (read.state === "refused") {
    return <Refused reason={read.reason} />;
  }
  return <DiffShown diff={read.value} since={since} put={putOnDesk} />;
}

/** One file's hunks as the desk shows them, once they have been read. */
export function DiffShown({ diff, since, put }: { diff: FileDiff; since: boolean; put: (item: DeskItem) => void }) {
  const putOnDesk = put;
  const path = diff.path;
  return (
    <section className="ms-desk-diff" aria-label={`The change to ${path}`}>
      <p className="ms-desk-caption">
        <span className="ms-mono">{diff.path}</span>
        <span>{since ? "since the tip you reviewed" : "as this work changed it"}</span>
      </p>
      {diff.binary === true && <p className="ms-desk-empty">A binary file; its change is not shown as lines.</p>}
      {diff.parts.map((part) =>
        part.hunks.map((hunk, at) => {
          const first = hunk.lines.find((line) => line.new !== undefined)?.new ?? 0;
          const last = [...hunk.lines].reverse().find((line) => line.new !== undefined)?.new ?? first;
          return (
            <div key={`${part.to}-${String(at)}`} className="ms-desk-hunk">
              <p className="ms-desk-hunk-head">
                <span className="ms-mono">{hunk.header}</span>
                {first > 0 && (
                  <button
                    type="button"
                    className="ms-desk-link"
                    onClick={() => {
                      putOnDesk({ kind: "source", path, from: first, to: last });
                    }}
                  >
                    Open these lines
                  </button>
                )}
              </p>
              <div
                className="ms-desk-lines"
                {...{ [ASK_SURFACE]: "desk", [ASK_SOURCE]: path, [ASK_REVISION]: part.to }}
              >
                {hunk.lines.map((line, index) => (
                  <div
                    key={index}
                    className={`ms-desk-line ms-desk-line--${line.kind}`}
                    data-line={line.new ?? line.old ?? 0}
                  >
                    <span className="ms-desk-number" aria-hidden="true">
                      {line.old ?? ""}
                    </span>
                    <span className="ms-desk-number" aria-hidden="true">
                      {line.new ?? ""}
                    </span>
                    <span className="ms-desk-sign" aria-hidden="true">
                      {line.kind === "added" ? "+" : line.kind === "deleted" ? "−" : " "}
                    </span>
                    <span className="ms-desk-code">{line.text === "" ? " " : line.text}</span>
                  </div>
                ))}
              </div>
            </div>
          );
        }),
      )}
      {diff.supplied < diff.total && (
        <p className="ms-desk-bound">
          {diff.supplied} of {diff.total} lines are shown.
        </p>
      )}
    </section>
  );
}

/**
 * A record's section, through the existing document reader: the heading the
 * Partner or an anchor named, and everything under it to the next heading of
 * its level.
 */
function SectionView({ record, section, at }: { record: string; section: string; at: string }) {
  const read = useRead<DocumentPayload>((signal) => loadDocument(record, signal), at);
  if (read.state === "loading") {
    return <p className="ms-desk-loading">Reading {record}…</p>;
  }
  if (read.state === "refused") {
    return <Refused reason={read.reason} />;
  }
  const document = read.value;
  const blocks = sectionOf(document.blocks, section);
  return (
    <section
      className="ms-desk-section"
      aria-label={`${record} at ${section}`}
      {...{ [ASK_SURFACE]: "desk", [ASK_SOURCE]: record, [ASK_REVISION]: document.revision }}
      data-ask-anchor={`${record} § ${section}`}
    >
      <p className="ms-desk-caption">
        <span className="ms-mono">{record}</span>
        <span>§ {section}</span>
      </p>
      {blocks.length === 0 ? (
        <p className="ms-desk-empty">This record has no section called {section}.</p>
      ) : (
        <Markdown blocks={blocks} from={record} />
      )}
    </section>
  );
}

/**
 * The blocks under one heading, found by its id or by its words: a Partner
 * names "D3" for the heading "D3. The owner holds the lock", and a reader names
 * the heading as written.
 */
export function sectionOf(blocks: readonly Block[], section: string): Block[] {
  const wanted = section.trim().toLowerCase();
  const at = blocks.findIndex((block) => {
    if (block.type !== "heading") {
      return false;
    }
    const said = wordsOf(block).trim().toLowerCase();
    return (
      (block.id ?? "").toLowerCase() === wanted ||
      said === wanted ||
      said.startsWith(`${wanted} `) ||
      said.startsWith(`${wanted}.`)
    );
  });
  if (at < 0) {
    return [];
  }
  const level = blocks[at].level ?? 1;
  let end = blocks.length;
  for (let index = at + 1; index < blocks.length; index += 1) {
    if (blocks[index].type === "heading" && (blocks[index].level ?? 1) <= level) {
      end = index;
      break;
    }
  }
  return blocks.slice(at, end);
}

/** A heading's words, joined from its inlines. */
function wordsOf(block: Block): string {
  return (block.inlines ?? []).map((inline) => inline.text ?? "").join("");
}
