<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'

const emit = defineEmits<{ (e: 'close'): void }>()

const LAST_URL_KEY = 'browser-last-url'
const addressInput = ref('')
const currentURL = ref('about:blank')
const history = ref<string[]>([])
const historyIndex = ref(-1)
const loading = ref(false)
const error = ref('')
const addressRef = ref<HTMLInputElement>()
const frameKey = ref(0)

const canGoBack = computed(() => historyIndex.value > 0)
const canGoForward = computed(() => historyIndex.value >= 0 && historyIndex.value < history.value.length - 1)

function normaliseURL(value: string): string | null {
  const trimmed = value.trim()
  if (!trimmed) return null
  const candidate = /^[a-z][a-z\d+.-]*:/i.test(trimmed) ? trimmed : `https://${trimmed}`
  try {
    const parsed = new URL(candidate)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.href : null
  } catch {
    return null
  }
}

function navigate(value = addressInput.value, pushHistory = true) {
  const url = normaliseURL(value)
  if (!url) {
    error.value = '请输入有效的 http 或 https 地址'
    return
  }
  error.value = ''
  addressInput.value = url
  currentURL.value = url
  loading.value = true
  frameKey.value++
  if (pushHistory) {
    history.value = [...history.value.slice(0, historyIndex.value + 1), url]
    historyIndex.value = history.value.length - 1
  }
  try { localStorage.setItem(LAST_URL_KEY, url) } catch { }
}

function goBack() {
  if (!canGoBack.value) return
  historyIndex.value--
  navigate(history.value[historyIndex.value], false)
}

function goForward() {
  if (!canGoForward.value) return
  historyIndex.value++
  navigate(history.value[historyIndex.value], false)
}

function reload() {
  if (currentURL.value === 'about:blank') return
  loading.value = true
  frameKey.value++
}

function openExternal() {
  if (currentURL.value !== 'about:blank') BrowserOpenURL(currentURL.value)
}

function onKeydown(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'l') {
    event.preventDefault()
    addressRef.value?.focus()
    addressRef.value?.select()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  try {
    const lastURL = localStorage.getItem(LAST_URL_KEY)
    if (lastURL) nextTick(() => navigate(lastURL))
  } catch { }
})
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="browser-panel">
    <form class="browser-toolbar" @submit.prevent="navigate()">
      <button type="button" :disabled="!canGoBack" title="后退" @click="goBack">←</button>
      <button type="button" :disabled="!canGoForward" title="前进" @click="goForward">→</button>
      <button type="button" :disabled="currentURL === 'about:blank'" title="刷新" @click="reload">↻</button>
      <input ref="addressRef" v-model="addressInput" autocomplete="url" spellcheck="false" placeholder="输入网址" aria-label="网址" />
      <button type="submit" class="go-button">访问</button>
      <button type="button" :disabled="currentURL === 'about:blank'" title="在系统浏览器中打开" @click="openExternal">↗</button>
      <button type="button" class="close-button" title="关闭浏览器标签" @click="emit('close')">×</button>
    </form>
    <div v-if="error" class="browser-error">{{ error }}</div>
    <div class="browser-page">
      <div v-if="currentURL === 'about:blank'" class="browser-empty">输入网址后按 Enter 访问。<br />部分网站禁止嵌入时，请使用 ↗ 在系统浏览器中打开。</div>
      <iframe v-else :key="frameKey" :src="currentURL" title="网页内容" @load="loading = false"></iframe>
      <div v-if="loading" class="loading-indicator">正在加载…</div>
    </div>
  </div>
</template>

<style scoped>
.browser-panel { display: flex; flex: 1; flex-direction: column; min-width: 0; min-height: 0; background: var(--surface-app); color: #c9d1d9; }
.browser-toolbar { display: flex; align-items: center; gap: 5px; padding: 7px; border-bottom: 1px solid #30363d; background: var(--surface-bar); flex-shrink: 0; }
.browser-toolbar button { min-width: 27px; height: 26px; border: 1px solid #3a4654; border-radius: 4px; background: var(--surface-tree); color: #c9d1d9; cursor: pointer; font-size: 14px; }
.browser-toolbar button:hover:not(:disabled) { border-color: #58a6ff; color: #58a6ff; }
.browser-toolbar button:disabled { opacity: .4; cursor: default; }
.browser-toolbar .go-button { padding: 0 9px; font-size: 11px; }
.browser-toolbar .close-button:hover { border-color: #f85149; color: #f85149; }
.browser-toolbar input { flex: 1; min-width: 0; height: 26px; box-sizing: border-box; border: 1px solid #3a4654; border-radius: 4px; outline: none; background: var(--surface-tree); color: #e6edf3; padding: 4px 8px; font-size: 12px; }
.browser-toolbar input:focus { border-color: #58a6ff; box-shadow: 0 0 0 1px #58a6ff; }
.browser-error { padding: 5px 8px; color: #f85149; background: rgba(248, 81, 73, .12); font-size: 11px; }
.browser-page { position: relative; flex: 1; min-height: 0; background: var(--surface-app); }
.browser-page iframe { display: block; width: 100%; height: 100%; border: 0; background: var(--surface-app); }
.browser-empty { display: grid; height: 100%; place-content: center; padding: 20px; color: #8b949e; background: var(--surface-app); text-align: center; font-size: 13px; line-height: 1.7; }
.loading-indicator { position: absolute; right: 10px; top: 8px; border-radius: 12px; background: var(--surface-bar); color: #c9d1d9; padding: 3px 8px; font-size: 11px; pointer-events: none; }
</style>
