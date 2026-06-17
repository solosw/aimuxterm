<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { useTerminalStore } from '../stores/terminal'
import '@xterm/xterm/css/xterm.css'

const props = defineProps<{ tabId: string; showCmdInput?: boolean }>()

const store = useTerminalStore()
const termEl = ref<HTMLDivElement>()
const isDragOver = ref(false)
const cmdInput = ref('')
const tab = computed(() => store.tabs.find(t => t.id === props.tabId) || null)
let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let unsubscribe: (() => void) | null = null

onMounted(async () => {
  await nextTick()
  const el = termEl.value
  if (!el) return

  if (tab.value?.restored) {
    term = new Terminal({
      cursorBlink: false,
      disableStdin: true,
      fontSize: 14,
      fontFamily: 'Consolas, "Courier New", monospace',
      theme: {
        background: '#161618',
        foreground: '#cccccc',
        cursor: '#ffffff',
        selectionBackground: '#444'
      }
    })
    fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(el)
    term.write('[2J[H')
    term.write(tab.value.output || '[无输出]')
    requestAnimationFrame(() => {
      if (fitAddon && el.offsetParent !== null) {
        try { fitAddon.fit() } catch {}
      }
    })
    return
  }

  term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'Consolas, "Courier New", monospace',
    theme: {
      background: '#161618',
      foreground: '#cccccc',
      cursor: '#ffffff',
      selectionBackground: '#444'
    },
    allowProposedApi: true
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(el)

  requestAnimationFrame(() => {
    if (fitAddon && el.offsetParent !== null) {
      try { fitAddon.fit() } catch {}
    }
  })

  unsubscribe = store.subscribeTerminal(props.tabId, (data: string) => {
    term?.write(data)
  })

  term.onData((data: string) => {
    store.writeToTerminal(props.tabId, data)
  })

  term.onResize(({ cols, rows }) => {
    store.resizeTerminal(props.tabId, cols, rows)
  })

  const observer = new ResizeObserver(() => {
    if (fitAddon && el.offsetParent !== null) {
      try { fitAddon.fit() } catch {}
    }
  })
  observer.observe(el)
})

function sendCommand() {
  if (tab.value?.restored) return
  const text = cmdInput.value.trim()
  if (text) {
    store.writeToTerminal(props.tabId, text + '\n')
    cmdInput.value = ''
  }
}

function onDragOver(event: DragEvent) {
  event.preventDefault()
  event.dataTransfer!.dropEffect = 'copy'
  isDragOver.value = true
}
function onDragLeave() { isDragOver.value = false }
function onDrop(event: DragEvent) {
  event.preventDefault()
  isDragOver.value = false
  const path = event.dataTransfer?.getData('text/plain')
  if (path && !tab.value?.restored) store.writeToTerminal(props.tabId, path)
}

onUnmounted(() => {
  unsubscribe?.()
  term?.dispose()
})
</script>

<template>
  <div v-if="tab?.restored" class="restored-terminal" :class="{ 'has-cmd-input': props.showCmdInput }">
    <div class="restore-banner">
      <span>上次终端会话已恢复，原进程已结束。</span>
      <button class="reconnect-btn" @click="store.reconnectTab(props.tabId)">重新连接</button>
    </div>
    <div ref="termEl" class="terminal-container has-cmd-input"></div>
    <div v-if="tab.error" class="restore-error">{{ tab.error }}</div>
  </div>
  <div
    v-else
    ref="termEl"
    class="terminal-container"
    :class="{ 'drag-over': isDragOver, 'has-cmd-input': props.showCmdInput }"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
  ></div>
<div v-if="props.showCmdInput" class="cmd-input-bar">
  <textarea
    v-model="cmdInput"
    class="cmd-input"
    :placeholder="tab?.restored ? '恢复的终端需要先重新连接才能发送命令' : '输入命令，Ctrl+Enter 发送到终端...'"
    :disabled="tab?.restored"
    @keydown.ctrl.enter="sendCommand()"
    rows="8"
  ></textarea>
  <button class="cmd-send" :disabled="tab?.restored" @click="sendCommand()" title="Ctrl+Enter 发送">
    发送 &#x23CE;
  </button>
</div>
</template>

<style scoped>

.restored-terminal {
  width: 100%;
  height: 100%;
  background: #161618;
  color: #c9d1d9;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.restored-terminal.has-cmd-input {
  height: calc(100% - 80px);
}
.restore-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  background: #1f2937;
  border-bottom: 1px solid #374151;
  color: #d1d5db;
  font-size: 12px;
}
.reconnect-btn {
  background: #238636;
  border: 1px solid #2ea043;
  color: #fff;
  border-radius: 4px;
  padding: 4px 12px;
  cursor: pointer;
  font-size: 12px;
}
.reconnect-btn:hover { background: #2ea043; }
.restore-error {
  color: #f85149;
  padding: 6px 10px;
  border-top: 1px solid #333;
  font-size: 12px;
}

.terminal-container {
  width: 100%;
  height: 100%;
}
.terminal-container.drag-over {
  box-shadow: inset 0 0 0 2px #58a6ff;
}
.terminal-container.has-cmd-input {
  height: calc(100% - 80px);
}
.cmd-input-bar {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px 8px;
  background: #1a1a1c;
  border-top: 1px solid #333;
  flex-shrink: 0;
}
.cmd-input {
  width: 100%;
  background: #0d1117;
  border: 1px solid #30363d;
  color: #c9d1d9;
  padding: 6px 10px;
  border-radius: 4px;
  font-size: 13px;
  font-family: Consolas, 'Courier New', monospace;
  outline: none;
  resize: vertical;
  min-height: 48px;
}
.cmd-input:focus {
  border-color: #58a6ff;
}
.cmd-send {
  align-self: flex-end;
  background: #238636;
  border: 1px solid #2ea043;
  color: #fff;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  padding: 4px 16px;
  border-radius: 4px;
}
.cmd-send:hover {
  background: #30363d;
  color: #58a6ff;
}
.cmd-input:disabled,
.cmd-send:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
