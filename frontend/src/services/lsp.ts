import * as monaco from 'monaco-editor'
import { GetLSPStatus, SendLSPMessage, StartLSP, StopLSP } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { useWorkspaceStore } from '../stores/workspace'

type JsonRPCMessage = { id?: number; method?: string; params?: any; result?: any; error?: { message?: string } }

type Session = {
  language: string
  nextID: number
  ready?: Promise<void>
  pending: Map<number, { resolve: (value: any) => void; reject: (reason: Error) => void }>
  providers: monaco.IDisposable[]
  documents: Set<string>
  unsubscribe?: () => void
}

const sessions = new Map<string, Session>()
const LSP_LANGUAGES: Record<string, string> = {
  go: 'go',
  typescript: 'typescript', javascript: 'javascript', vue: 'vue',
  python: 'python', rust: 'rust',
  c: 'c', cpp: 'cpp', objectivec: 'objectivec',
  java: 'java', kotlin: 'kotlin', csharp: 'csharp',
  php: 'php', ruby: 'ruby', lua: 'lua', dart: 'dart',
  bash: 'bash', powershell: 'powershell', sql: 'sql',
  yaml: 'yaml', dockerfile: 'dockerfile',
  html: 'html', xml: 'xml', css: 'css', scss: 'scss', less: 'less', json: 'json',
}

function fileURI(path: string): string {
  const normalized = path.replace(/\\/g, '/')
  return normalized.startsWith('/') ? `file://${encodeURI(normalized)}` : `file:///${encodeURI(normalized)}`
}

function position(model: monaco.editor.ITextModel, position: monaco.Position) {
  return { line: position.lineNumber - 1, character: position.column - 1 }
}

function range(range: any): monaco.IRange {
  return {
    startLineNumber: range.start.line + 1,
    startColumn: range.start.character + 1,
    endLineNumber: range.end.line + 1,
    endColumn: range.end.character + 1,
  }
}

function send(session: Session, message: Record<string, any>) {
  return SendLSPMessage(session.language, JSON.stringify({ jsonrpc: '2.0', ...message }))
}

function request(session: Session, method: string, params: any): Promise<any> {
  const id = session.nextID++
  return new Promise((resolve, reject) => {
    session.pending.set(id, { resolve, reject })
    send(session, { id, method, params }).catch(error => {
      session.pending.delete(id)
      reject(error)
    })
  })
}

function applyDiagnostics(params: any) {
  const model = monaco.editor.getModel(monaco.Uri.parse(params.uri))
  if (!model) return
  const markers = (params.diagnostics || []).map((diagnostic: any) => ({
    severity: diagnostic.severity === 1 ? monaco.MarkerSeverity.Error : diagnostic.severity === 2 ? monaco.MarkerSeverity.Warning : monaco.MarkerSeverity.Info,
    message: diagnostic.message || '',
    source: diagnostic.source || 'LSP',
    ...range(diagnostic.range),
  }))
  monaco.editor.setModelMarkers(model, 'lsp', markers)
}

function handleMessage(session: Session, raw: string) {
  let message: JsonRPCMessage
  try { message = JSON.parse(raw) } catch { return }
  if (typeof message.id === 'number' && message.method) {
    const configuration = message.method === 'workspace/configuration'
      ? (message.params?.items || []).map(() => null)
      : null
    void send(session, { id: message.id, result: configuration })
    return
  }
  if (typeof message.id === 'number') {
    const pending = session.pending.get(message.id)
    if (pending) {
      session.pending.delete(message.id)
      if (message.error) pending.reject(new Error(message.error.message || 'LSP 请求失败'))
      else pending.resolve(message.result)
    }
    return
  }
  if (message.method === 'textDocument/publishDiagnostics') applyDiagnostics(message.params)
}

async function ensureSession(monacoLanguage: string): Promise<Session | null> {
  const language = LSP_LANGUAGES[monacoLanguage]
  const workspace = useWorkspaceStore().info
  if (!language || !workspace || workspace.isRemote) return null
  let session = sessions.get(language)
  if (session) return session.ready ? (await session.ready, session) : session

  const status = await GetLSPStatus(language)
  if (!status?.available) return null
  session = { language, nextID: 1, pending: new Map(), providers: [], documents: new Set() }
  sessions.set(language, session)
  session.unsubscribe = EventsOn(`lsp-message:${language}`, (raw: string) => handleMessage(session!, raw))
  session.ready = (async () => {
    await StartLSP(language)
    await request(session!, 'initialize', {
      processId: null,
      rootUri: fileURI(workspace.path),
      capabilities: {
        textDocument: {
          completion: { completionItem: { snippetSupport: true } },
          hover: {}, definition: {}, publishDiagnostics: {},
        },
      },
      workspaceFolders: [{ uri: fileURI(workspace.path), name: workspace.name }],
    })
    await send(session!, { method: 'initialized', params: {} })
    registerProviders(session!, monacoLanguage)
  })().catch(error => {
    sessions.delete(language)
    session?.unsubscribe?.()
    throw error
  })
  await session.ready
  return session
}

function registerProviders(session: Session, monacoLanguage: string) {
  const modelParams = (model: monaco.editor.ITextModel, at: monaco.Position) => ({ textDocument: { uri: model.uri.toString() }, position: position(model, at) })
  session.providers.push(monaco.languages.registerHoverProvider(monacoLanguage, {
    provideHover: async (model, at) => {
      try {
        const result = await request(session, 'textDocument/hover', modelParams(model, at))
        if (!result?.contents) return null
        const values = Array.isArray(result.contents) ? result.contents : [result.contents]
        return { range: result.range ? range(result.range) : undefined, contents: values.map((value: any) => ({ value: typeof value === 'string' ? value : value.value || '' })) }
      } catch { return null }
    },
  }))
  session.providers.push(monaco.languages.registerDefinitionProvider(monacoLanguage, {
    provideDefinition: async (model, at) => {
      try {
        const result = await request(session, 'textDocument/definition', modelParams(model, at))
        const items = Array.isArray(result) ? result : result ? [result] : []
        return items.map((item: any) => ({ uri: monaco.Uri.parse(item.uri || item.targetUri), range: range(item.range || item.targetSelectionRange) }))
      } catch { return [] }
    },
  }))
  session.providers.push(monaco.languages.registerCompletionItemProvider(monacoLanguage, {
    triggerCharacters: ['.', ':', '>', '/'],
    provideCompletionItems: async (model, at) => {
      try {
        const result = await request(session, 'textDocument/completion', { ...modelParams(model, at), context: { triggerKind: 1 } })
        const items = Array.isArray(result) ? result : result?.items || []
        return { suggestions: items.map((item: any) => ({
          label: item.label,
          detail: item.detail,
          documentation: typeof item.documentation === 'string' ? item.documentation : item.documentation?.value,
          kind: monaco.languages.CompletionItemKind.Text,
          insertText: typeof item.textEdit?.newText === 'string' ? item.textEdit.newText : item.insertText || item.label,
          range: item.textEdit?.range ? range(item.textEdit.range) : undefined,
        })) }
      } catch { return { suggestions: [] } }
    },
  }))
}

export async function openLSPDocument(monacoLanguage: string, model: monaco.editor.ITextModel) {
  const session = await ensureSession(monacoLanguage)
  if (!session || session.documents.has(model.uri.toString())) return
  session.documents.add(model.uri.toString())
  await send(session, { method: 'textDocument/didOpen', params: { textDocument: { uri: model.uri.toString(), languageId: monacoLanguage, version: 1, text: model.getValue() } } })
}

export async function changeLSPDocument(monacoLanguage: string, model: monaco.editor.ITextModel, version: number) {
  const session = sessions.get(LSP_LANGUAGES[monacoLanguage])
  if (!session?.documents.has(model.uri.toString())) return
  await send(session, { method: 'textDocument/didChange', params: { textDocument: { uri: model.uri.toString(), version }, contentChanges: [{ text: model.getValue() }] } })
}

export async function closeLSPDocument(monacoLanguage: string, model: monaco.editor.ITextModel) {
  const session = sessions.get(LSP_LANGUAGES[monacoLanguage])
  if (!session?.documents.delete(model.uri.toString())) return
  monaco.editor.setModelMarkers(model, 'lsp', [])
  await send(session, { method: 'textDocument/didClose', params: { textDocument: { uri: model.uri.toString() } } }).catch(() => {})
}

export async function stopAllLSP() {
  for (const session of sessions.values()) {
    session.unsubscribe?.()
    session.providers.forEach(provider => provider.dispose())
    await StopLSP(session.language).catch(() => {})
  }
  sessions.clear()
}
