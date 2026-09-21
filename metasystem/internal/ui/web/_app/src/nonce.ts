/**
 * Reads the per-response nonce the server substituted into the page.
 *
 * The `nonce` IDL property is used rather than `getAttribute`: a browser hides
 * the attribute once the document is parsed, so the attribute reads empty while
 * the property still carries the value.
 *
 * The root is a parameter so the rule is testable without a DOM.
 */
export function readNonce(root: ParentNode): string {
  const meta = root.querySelector<HTMLMetaElement>('meta[property="csp-nonce"]');
  return meta?.nonce ?? "";
}
