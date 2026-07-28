import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'

/**
 * Markdown renderer for the file preview.
 *
 * Security: `html: false` means raw HTML in the document is escaped rather than
 * emitted, so untrusted file content cannot inject markup or scripts into the
 * webview. Link targets still go through `isSafeHref` before being followed.
 */
function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

const md: MarkdownIt = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: false,
  highlight(code, lang) {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return hljs.highlight(code, { language: lang }).value
      } catch { /* fall through to escaped output */ }
    }
    return escapeHtml(code)
  },
})

export function renderMarkdown(source: string): string {
  return md.render(source)
}

const SAFE_SCHEME = /^(https?:|mailto:)/i

/** Whether a link from rendered markdown may be opened in the system browser. */
export function isSafeHref(href: string): boolean {
  return SAFE_SCHEME.test(href.trim())
}
