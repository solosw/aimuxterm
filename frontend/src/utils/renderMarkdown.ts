import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import mermaid from 'mermaid'

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

const defaultFence = md.renderer.rules.fence
md.renderer.rules.fence = (tokens, index, options, env, self) => {
  const token = tokens[index]
  const language = token.info.trim().split(/\s+/)[0].toLowerCase()
  if (language === 'mermaid' || language === 'mmd') {
    return `<div class="mermaid-block"><pre class="mermaid-source">${escapeHtml(token.content)}</pre></div>`
  }
  return defaultFence ? defaultFence(tokens, index, options, env, self) : self.renderToken(tokens, index, options)
}

export function renderMarkdown(source: string): string {
  return md.render(source)
}

let mermaidInitialized = false

export async function renderMermaidBlocks(root: HTMLElement): Promise<void> {
  const blocks = Array.from(root.querySelectorAll<HTMLElement>('.mermaid-source'))
  if (!blocks.length) return
  if (!mermaidInitialized) {
    mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'dark' })
    mermaidInitialized = true
  }
  for (const block of blocks) {
    if (block.dataset.rendered === 'true') continue
    const source = block.textContent || ''
    try {
      const id = `mermaid-${Date.now()}-${Math.random().toString(36).slice(2)}`
      const { svg } = await mermaid.render(id, source)
      block.outerHTML = `<div class="mermaid-diagram">${svg}</div>`
    } catch {
      block.classList.add('mermaid-error')
      block.dataset.rendered = 'true'
    }
  }
}

const SAFE_SCHEME = /^(https?:|mailto:)/i

export function isSafeHref(href: string): boolean {
  return SAFE_SCHEME.test(href.trim())
}
