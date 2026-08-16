<script setup lang="ts">
import { onMounted, onUnmounted, ref, shallowRef, watch } from 'vue'
import * as monaco from 'monaco-editor'
import { loadGrammars, loadTheme } from 'monaco-volar'
import * as onigasm from 'onigasm'
import onigasmWasm from 'onigasm/lib/onigasm.wasm?url'
import { useAICompletionStore } from '../stores/aiCompletion'
import { useWorkspaceStore } from '../stores/workspace'
import { changeLSPDocument, closeLSPDocument, openLSPDocument } from '../services/lsp'

let vueLanguageSupport: Promise<void> | undefined

function enableVueLanguageSupport(instance: monaco.editor.IStandaloneCodeEditor): Promise<void> {
  if (!vueLanguageSupport) {
    vueLanguageSupport = onigasm.loadWASM(onigasmWasm)
      .then(() => loadTheme(monaco.editor))
      .then(({ dark }) => {
        monaco.editor.setTheme(dark)
        return loadGrammars(monaco as any, instance as any)
      })
      .catch((error) => {
        // Vue editing remains available with plaintext fallback if the optional
        // TextMate grammar cannot load in the current WebView.
        console.warn('无法加载 Monaco Vue 语言支持:', error)
      })
  }
  return vueLanguageSupport
}

monaco.languages.typescript.typescriptDefaults.setCompilerOptions({
  jsx: monaco.languages.typescript.JsxEmit.ReactJSX,
  allowJs: true,
  allowNonTsExtensions: true,
  target: monaco.languages.typescript.ScriptTarget.ESNext,
})

const props = withDefaults(defineProps<{
  modelValue: string
  language: string
  readOnly?: boolean
  path?: string
}>(), {
  readOnly: false,
  path: '',
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'save'): void
}>()

const editorRef = ref<HTMLElement>()
const editor = shallowRef<monaco.editor.IStandaloneCodeEditor>()
const model = shallowRef<monaco.editor.ITextModel>()
const aiCompletion = useAICompletionStore()
const workspace = useWorkspaceStore()
let lspVersion = 1
let changeSubscription: monaco.IDisposable | undefined
let saveSubscription: monaco.IDisposable | undefined
let completionAction: monaco.IDisposable | undefined
let inlineCompletionProvider: monaco.IDisposable | undefined
let autoCompletionTimer: ReturnType<typeof setTimeout> | undefined
let completionController: AbortController | undefined
let completionRequestId = 0
let modelUri = ''

const languageAliases: Record<string, string> = {
  plaintext: 'plaintext',
  typescript: 'typescript',
  javascript: 'javascript',
  vue: 'vue',
  json: 'json',
  html: 'html',
  xml: 'xml',
  css: 'css',
  scss: 'scss',
  less: 'less',
  markdown: 'markdown',
  python: 'python',
  java: 'java',
  go: 'go',
  rust: 'rust',
  php: 'php',
  sql: 'sql',
  c: 'c',
  cpp: 'cpp',
  csharp: 'csharp',
  objectivec: 'objective-c',
  kotlin: 'kotlin',
  scala: 'scala',
  dart: 'dart',
  ruby: 'ruby',
  swift: 'swift',
  lua: 'lua',
  r: 'r',
  bash: 'shell',
  fish: 'shell',
  powershell: 'powershell',
  dockerfile: 'dockerfile',
  makefile: 'plaintext',
  yaml: 'yaml',
  ini: 'ini',
  toml: 'ini',
}

function getMonacoLanguage(language: string): string {
  return languageAliases[language] || 'plaintext'
}

function getModelUri(): monaco.Uri {
  const relativePath = props.path.replace(/\\/g, '/')
  const workspacePath = workspace.info && !workspace.info.isRemote ? workspace.info.path.replace(/\\/g, '/') : ''
  const safePath = workspacePath && relativePath ? `${workspacePath.replace(/\/+$/, '')}/${relativePath.replace(/^\/+/, '')}` : relativePath || `untitled-${Date.now()}.${props.language}`
  modelUri = safePath.startsWith('/') ? `file://${encodeURI(safePath)}` : safePath.match(/^[a-z]:\//i) ? `file:///${encodeURI(safePath)}` : `inmemory://aimuxterm/${encodeURI(safePath)}`
  return monaco.Uri.parse(modelUri)
}

function getCompletionEndpoint(baseUrl: string): string {
  return `${baseUrl.replace(/\/+$/, '').replace(/\/v1$/, '')}/v1/completions`
}

function buildCompletionContext(currentModel: monaco.editor.ITextModel, position: monaco.Position): { prefix: string; suffix: string } {
  const startLine = Math.max(1, position.lineNumber - 80)
  const endLine = Math.min(currentModel.getLineCount(), position.lineNumber + 80)
  return {
    prefix: currentModel.getValueInRange({
      startLineNumber: startLine,
      startColumn: 1,
      endLineNumber: position.lineNumber,
      endColumn: position.column,
    }),
    suffix: currentModel.getValueInRange({
      startLineNumber: position.lineNumber,
      startColumn: position.column,
      endLineNumber: endLine,
      endColumn: currentModel.getLineMaxColumn(endLine),
    }),
  }
}

function extractCompletionText(payload: unknown): string | undefined {
  const choice = (payload as { choices?: Array<{ text?: unknown; message?: { content?: unknown } }> })?.choices?.[0]
  if (typeof choice?.text === 'string') return choice.text
  if (typeof choice?.message?.content === 'string') return choice.message.content
  return undefined
}

async function requestInlineCompletion(currentModel: monaco.editor.ITextModel, position: monaco.Position, token: monaco.CancellationToken): Promise<string> {
  if (!aiCompletion.isReady || props.readOnly || token.isCancellationRequested) return ''

  completionController?.abort()
  const controller = new AbortController()
  completionController = controller
  const requestId = ++completionRequestId
  const cancel = token.onCancellationRequested(() => controller.abort())
  const config = aiCompletion.config
  const { prefix, suffix } = buildCompletionContext(currentModel, position)
  try {
    const response = await fetch(getCompletionEndpoint(config.baseUrl), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${config.apiKey}`,
      },
      body: JSON.stringify({
        model: config.model,
        prompt: prefix,
        suffix,
        temperature: 0,
        max_tokens: 128,
        stream: false,
      }),
      signal: controller.signal,
    })
    if (!response.ok) throw new Error(`AI 补全请求失败 (${response.status})`)
    const payload = await response.json()
    if (requestId !== completionRequestId || token.isCancellationRequested) return ''
    const text = extractCompletionText(payload)
    if (typeof text !== 'string') {
      console.warn('AI 补全响应缺少 choices[0].text:', payload)
      return ''
    }
    return text
      .replace(/^```[\w-]*\s*\n?/i, '')
      .replace(/\n?```\s*$/, '')
  } catch (error: any) {
    if (error?.name !== 'AbortError') console.warn('AI 自动补全失败:', error)
    return ''
  } finally {
    cancel.dispose()
    if (completionController === controller) completionController = undefined
  }
}

function configureInlineCompletion() {
  inlineCompletionProvider?.dispose()
  inlineCompletionProvider = undefined
  if (props.readOnly || !aiCompletion.isReady) return

  inlineCompletionProvider = monaco.languages.registerInlineCompletionsProvider('*', {
    provideInlineCompletions: async (currentModel, position, _context, token) => {
      const text = await requestInlineCompletion(currentModel, position, token)
      if (!text) return { items: [] }
      return {
        items: [{
          insertText: text,
        }],
      }
    },
    freeInlineCompletions: () => {},
  })
}

function triggerCopilot(requireFocus = true) {
  if (props.readOnly || !aiCompletion.isReady) return
  const instance = editor.value
  if (!instance || (requireFocus && !instance.hasTextFocus())) return
  instance.trigger('aimuxterm', 'editor.action.inlineSuggest.trigger', undefined)
}

function scheduleCopilot() {
  if (!aiCompletion.config.autoTrigger || props.readOnly || !aiCompletion.isReady) return
  clearTimeout(autoCompletionTimer)
  autoCompletionTimer = setTimeout(() => triggerCopilot(false), 700)
}

onMounted(() => {
  aiCompletion.load()
  const uri = getModelUri()
  model.value = monaco.editor.getModel(uri) || monaco.editor.createModel(
    props.modelValue,
    getMonacoLanguage(props.language),
    uri,
  )

  editor.value = monaco.editor.create(editorRef.value!, {
    model: model.value,
    theme: 'vs-dark',
    automaticLayout: true,
    readOnly: props.readOnly,
    minimap: { enabled: true },
    lineNumbers: 'on',
    renderWhitespace: 'selection',
    scrollBeyondLastLine: false,
    fontFamily: 'Consolas, "Courier New", monospace',
    fontSize: 13,
    lineHeight: 20,
    tabSize: 2,
    insertSpaces: true,
    wordWrap: 'off',
    bracketPairColorization: { enabled: true },
    guides: { bracketPairs: true, indentation: true },
    padding: { top: 8, bottom: 8 },
    inlineSuggest: {
      enabled: true,
      mode: 'subwordSmart',
      showToolbar: 'always',
    },
    acceptSuggestionOnCommitCharacter: false,
  })

  changeSubscription = model.value.onDidChangeContent(() => {
    emit('update:modelValue', model.value?.getValue() || '')
    if (model.value) void changeLSPDocument(getMonacoLanguage(props.language), model.value, ++lspVersion)
    scheduleCopilot()
  })
  saveSubscription = editor.value.addAction({
    id: 'aimuxterm.save-file',
    label: '保存文件',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS],
    run: () => emit('save'),
  })
  completionAction = editor.value.addAction({
    id: 'aimuxterm.trigger-ai-completion',
    label: '触发 AI 代码补全',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyB],
    precondition: 'editorTextFocus && !editorReadonly',
    run: () => triggerCopilot(),
  })

  configureInlineCompletion()
  void openLSPDocument(getMonacoLanguage(props.language), model.value)
  if (getMonacoLanguage(props.language) === 'vue') {
    enableVueLanguageSupport(editor.value)
  }
})

watch(() => props.modelValue, (value) => {
  const currentModel = model.value
  if (currentModel && value !== currentModel.getValue()) currentModel.setValue(value)
})

watch(() => props.language, (language) => {
  if (model.value) monaco.editor.setModelLanguage(model.value, getMonacoLanguage(language))
})

watch(() => props.readOnly, (readOnly) => {
  editor.value?.updateOptions({ readOnly })
  configureInlineCompletion()
})

watch(() => [
  aiCompletion.config.enabled,
  aiCompletion.config.apiKey,
  aiCompletion.config.baseUrl,
  aiCompletion.config.model,
  aiCompletion.config.autoTrigger,
], () => {
  configureInlineCompletion()
})

onUnmounted(() => {
  clearTimeout(autoCompletionTimer)
  completionController?.abort()
  inlineCompletionProvider?.dispose()
  changeSubscription?.dispose()
  const currentModel = model.value
  if (currentModel) void closeLSPDocument(getMonacoLanguage(props.language), currentModel)
  saveSubscription?.dispose()
  completionAction?.dispose()
  editor.value?.dispose()
  if (currentModel && currentModel.uri.toString() === modelUri) currentModel.dispose()
})

defineExpose({ triggerCopilot })
</script>

<template>
  <div ref="editorRef" class="monaco-editor-wrap"></div>
</template>

<style scoped>
.monaco-editor-wrap {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: #1e1e1e;
}
</style>
