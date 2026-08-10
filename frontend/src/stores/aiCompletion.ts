import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

const STORAGE_KEY = 'editor-ai-completion-config'

export interface AICompletionConfig {
  enabled: boolean
  autoTrigger: boolean
  apiKey: string
  baseUrl: string
  model: string
}

const defaults: AICompletionConfig = {
  enabled: false,
  autoTrigger: false,
  apiKey: '',
  baseUrl: '',
  model: '',
}

function normalizeBaseUrl(value: string): string {
  return value.trim().replace(/\/+$/, '').replace(/\/v1$/, '')
}

function normalize(config: Partial<AICompletionConfig>): AICompletionConfig {
  return {
    enabled: !!config.enabled,
    autoTrigger: !!config.autoTrigger,
    apiKey: (config.apiKey || '').trim(),
    baseUrl: normalizeBaseUrl(config.baseUrl || ''),
    model: config.model || '',
  }
}

export const useAICompletionStore = defineStore('aiCompletion', () => {
  const config = ref<AICompletionConfig>({ ...defaults })
  const loaded = ref(false)

  const isReady = computed(() =>
    config.value.enabled && !!config.value.apiKey && !!config.value.baseUrl && !!config.value.model,
  )

  function load() {
    if (loaded.value) return
    loaded.value = true
    try {
      const raw = localStorage.getItem(STORAGE_KEY)
      if (raw) config.value = normalize(JSON.parse(raw))
    } catch {
      config.value = { ...defaults }
    }
  }

  function save(next: Partial<AICompletionConfig> = {}) {
    config.value = normalize({ ...config.value, ...next })
    localStorage.setItem(STORAGE_KEY, JSON.stringify(config.value))
  }

  return { config, loaded, isReady, load, save }
})
