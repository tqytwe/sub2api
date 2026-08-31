const PUBLIC_CONTENT_FALLBACK_SELECTOR = '[data-jisudeng-public-fallback="true"]'

/** Remove the no-JavaScript public document before Vue paints the SPA. */
export function removePublicContentFallback(root: ParentNode = document): void {
  root.querySelector(PUBLIC_CONTENT_FALLBACK_SELECTOR)?.remove()
}
