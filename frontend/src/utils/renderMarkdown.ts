import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import mermaid from 'mermaid'
import { GetImagePreviewData } from '../../wailsjs/go/main/App'

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

const IMAGE_EXT = /\.(png|jpe?g|gif|webp|bmp|ico|svg)(?:[?#].*)?$/i
const SAFE_DIRECT_IMG = /^(https?:|data:image\/)/i

export function isDirectImageSrc(src: string): boolean {
  return SAFE_DIRECT_IMG.test(src.trim())
}

/** True when markdown src needs backend resolution (relative / absolute / file://). */
export function needsImageHydration(src: string): boolean {
  const s = src.trim()
  if (!s || SAFE_DIRECT_IMG.test(s) || /^mailto:/i.test(s)) return false
  if (/^file:/i.test(s)) return true
  if (/^[a-zA-Z]:[\\/]/.test(s) || s.startsWith('\\\\')) return true
  // Relative / bare path that looks like an image
  if (IMAGE_EXT.test(s) || s.startsWith('./') || s.startsWith('../')) return true
  // Relative path without scheme
  return !/^[a-z][a-z0-9+.-]*:/i.test(s)
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

const defaultImage = md.renderer.rules.image
md.renderer.rules.image = (tokens, index, options, env, self) => {
  const token = tokens[index]
  const src = (token.attrGet('src') || '').trim()
  if (src && needsImageHydration(src)) {
    token.attrSet('src', '')
    token.attrSet('data-aimux-src', src)
    token.attrSet('class', [token.attrGet('class') || '', 'aimux-md-img'].filter(Boolean).join(' '))
    token.attrSet('loading', 'lazy')
  }
  return defaultImage ? defaultImage(tokens, index, options, env, self) : self.renderToken(tokens, index, options)
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

const imagePreviewCache = new Map<string, Promise<string>>()

async function resolveImageDataUrl(src: string): Promise<string> {
  const key = src
  let pending = imagePreviewCache.get(key)
  if (!pending) {
    pending = GetImagePreviewData(src).catch((err) => {
      imagePreviewCache.delete(key)
      throw err
    })
    imagePreviewCache.set(key, pending)
  }
  return pending
}

/** Resolve local / file:// / absolute markdown images to data URLs in-place. */
export async function hydrateMarkdownImages(root: HTMLElement): Promise<void> {
  const imgs = Array.from(root.querySelectorAll<HTMLImageElement>('img[data-aimux-src]'))
  if (!imgs.length) return
  await Promise.all(imgs.map(async (img) => {
    const src = img.getAttribute('data-aimux-src') || ''
    if (!src) return
    if (img.dataset.aimuxHydrated === '1') return
    img.dataset.aimuxHydrated = 'pending'
    try {
      const dataUrl = await resolveImageDataUrl(src)
      if (!dataUrl || !isDirectImageSrc(dataUrl)) {
        img.classList.add('aimux-md-img-error')
        img.dataset.aimuxHydrated = 'error'
        return
      }
      img.src = dataUrl
      img.removeAttribute('data-aimux-src')
      img.dataset.aimuxHydrated = '1'
      img.classList.remove('aimux-md-img-error')
    } catch {
      img.classList.add('aimux-md-img-error')
      img.dataset.aimuxHydrated = 'error'
      img.alt = img.alt || src
      img.title = `无法加载图片: ${src}`
    }
  }))
}

const SAFE_SCHEME = /^(https?:|mailto:)/i

export function isSafeHref(href: string): boolean {
  return SAFE_SCHEME.test(href.trim())
}
