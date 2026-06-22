<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useWorkspaceStore } from './stores/workspace'
import { useFileChangesStore } from './stores/fileChanges'
import { useTerminalStore } from './stores/terminal'
import { GetStartupWorkspace, CreateTerminal, WriteToTerminal } from '../wailsjs/go/main/App'
import { config } from '../wailsjs/go/models'
import WorkspaceBar from './components/WorkspaceBar.vue'
import FileTreePanel from './components/FileTreePanel.vue'
import TerminalPanel from './components/TerminalPanel.vue'
import FilePreviewPanel from './components/FilePreviewPanel.vue'
import FileChangesPanel from './components/FileChangesPanel.vue'
import StartupCommandPicker from './components/StartupCommandPicker.vue'
import StartupCommandSettings from './components/StartupCommandSettings.vue'
import AIConfigSettings from './components/AIConfigSettings.vue'

const ws = useWorkspaceStore()
const term = useTerminalStore()
const showSettings = ref(false)
const showAISettings = ref(false)
const fc = useFileChangesStore()

function escapeCdPath(p: string) {
  return p.replace(/"/g, '\\"')
}

async function onPickerSelect(cmd: config.StartupCommand) {
  ws.showStartupPicker = false
  const id = await CreateTerminal()
  if (id) {
    term.addSSHTab(id, cmd.name)

    await WriteToTerminal(id, cmd.command + '\n')
    if (ws.info && !ws.info.isRemote) {
      await WriteToTerminal(id, 'cd "' + escapeCdPath(ws.info.path) + '"\n')
    }
  }
}

async function onPickerDismiss() {
  ws.showStartupPicker = false
  const tab = await term.createTerminal()
  if (tab && ws.info && !ws.info.isRemote) {
    await WriteToTerminal(tab.id, 'cd "' + escapeCdPath(ws.info.path) + '"\n')
  }
}

function onPickerSettings() {
  showSettings.value = true
  ws.showStartupPicker = false
}

function onOpenAISettings() {
  showAISettings.value = true
}

// Resizable panel widths
const treeWidth = ref(220)
const previewWidth = ref(320)
const changesWidth = ref(280)

function startResize(target: 'tree' | 'preview' | 'changes') {
  const onMove = (e: MouseEvent) => {
    if (target === 'tree') {
      treeWidth.value = Math.max(140, Math.min(400, e.clientX - 4))
    } else if (target === 'changes') {
      changesWidth.value = Math.max(180, Math.min(500, window.innerWidth - e.clientX - 4))
    } else if (target === 'preview') {
      previewWidth.value = Math.max(200, Math.min(600, e.clientX - treeWidth.value - 12))
    }
  }
  const onUp = () => {
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

onMounted(async () => {
  ws.loadHistory()
  fc.initListener()
  const startupWs = await GetStartupWorkspace()
  if (startupWs) {
    await ws.openWorkspace(startupWs)
  }
  await term.loadSnapshots()
})
</script>

<template>
  <div class="app-layout">
    <WorkspaceBar />
    <button class="ai-settings-fab" @click="onOpenAISettings">AI 配置</button>
    <div class="main-area">
      <FileTreePanel v-if="ws.hasWorkspace" :style="{ width: treeWidth + 'px' }" />
      <div
        v-if="ws.hasWorkspace"
        class="resize-handle"
        @mousedown="startResize('tree')"
      ></div>
      <TerminalPanel />
      <div
        v-if="ws.hasWorkspace && ws.previewFiles.length > 0"
        class="resize-handle"
        @mousedown="startResize('preview')"
      ></div>
      <FilePreviewPanel v-if="ws.hasWorkspace && ws.previewFiles.length > 0" :style="{ width: previewWidth + 'px' }" />
      <div
        v-if="ws.hasWorkspace"
        class="resize-handle"
        @mousedown="startResize('changes')"
      ></div>
      <FileChangesPanel v-if="ws.hasWorkspace" :style="{ width: changesWidth + 'px' }" />
    </div>
    <StartupCommandPicker
      v-if="ws.showStartupPicker"
      @select="onPickerSelect"
      @dismiss="onPickerDismiss"
      @settings="onPickerSettings"
    />
    <StartupCommandSettings
      v-if="showSettings"
      @close="showSettings = false"
    />
    <AIConfigSettings
      v-if="showAISettings"
      @close="showAISettings = false"
    />
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}
.main-area {
  flex: 1;
  display: flex;
  overflow: hidden;
}
.resize-handle {
  width: 4px;
  cursor: col-resize;
  background: transparent;
  transition: background 0.15s;
  flex-shrink: 0;
  z-index: 10;
}
.resize-handle:hover {
  background: #58a6ff;
}
.ai-settings-fab {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 120;
  background: #2563eb;
  border: 1px solid #1d4ed8;
  color: #fff;
  border-radius: 999px;
  padding: 10px 14px;
  font-weight: 700;
  cursor: pointer;
  box-shadow: 0 6px 20px rgba(0,0,0,0.35);
}
.ai-settings-fab:hover { background: #1d4ed8; }
</style>
