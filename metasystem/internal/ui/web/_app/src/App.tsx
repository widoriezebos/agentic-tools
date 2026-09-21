import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { useEffect, useState } from "react";

/**
 * The page the engine serves. It is deliberately near-empty: the shell, the
 * navigation and the visual language are a later slice. What it proves is the
 * pipeline, end to end — the bundle is built, embedded, and served; the page
 * reaches the server's own origin and nothing else; and the dialog's scroll
 * lock, the one inline style in the application, is allowed by the nonce.
 */

type Health = {
  status: string;
  checkout: string;
  startedAt: string;
  engineBuild: string;
  executableDigest: string;
  bundleDigest: string;
};

export function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [failure, setFailure] = useState("");

  useEffect(() => {
    const aborter = new AbortController();
    fetch("/-/health", { signal: aborter.signal })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`/-/health answered ${String(response.status)}`);
        }
        return (await response.json()) as Health;
      })
      .then(setHealth)
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setFailure(error instanceof Error ? error.message : String(error));
        }
      });
    return () => {
      aborter.abort();
    };
  }, []);

  return (
    <main className="p-8">
      <h1 className="text-xl font-semibold">MetaSystem interface</h1>

      {failure !== "" && <p role="alert">{failure}</p>}
      {health === null && failure === "" && <p>reading /-/health</p>}
      {health !== null && (
        <dl>
          {Object.entries(health).map(([name, value]) => (
            <div key={name}>
              <dt>{name}</dt>
              <dd>{value}</dd>
            </div>
          ))}
        </dl>
      )}

      <p>path: {window.location.pathname}</p>

      <Dialog.Root>
        <Dialog.Trigger className="underline">Open a dialog</Dialog.Trigger>
        <Dialog.Portal>
          {/* The overlay is what mounts the scroll lock, and the scroll lock is
              what appends the one inline style the nonce allows. A dialog
              without it would pass the nonce proof by never exercising it. */}
          <Dialog.Overlay />
          <Dialog.Content>
            <Dialog.Title>A dialog</Dialog.Title>
            <Dialog.Description>
              This dialog exists to prove one thing: its scroll lock appends a style element, and the page&rsquo;s
              content security policy allows that element only because it carries this response&rsquo;s nonce.
            </Dialog.Description>
            <Dialog.Close aria-label="Close">
              <X aria-hidden="true" />
            </Dialog.Close>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>

      <p>
        <a href="/THIRD-PARTY-NOTICES.txt">Open-source notices</a>
      </p>
    </main>
  );
}
