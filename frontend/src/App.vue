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
import AppearanceSettings from './components/AppearanceSettings.vue'
import { useAppearanceStore } from './stores/appearance'

const ws = useWorkspaceStore()
const term = useTerminalStore()
const showSettings = ref(false)
const showAISettings = ref(false)
const showAppearance = ref(false)
const fc = useFileChangesStore()
const appearance = useAppearanceStore()

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
const changesWidth = ref(280)

function startResize(target: 'tree' | 'changes') {
  const onMove = (e: MouseEvent) => {
    if (target === 'tree') {
      treeWidth.value = Math.max(140, Math.min(400, e.clientX - 4))
    } else if (target === 'changes') {
      changesWidth.value = Math.max(180, Math.min(500, window.innerWidth - e.clientX - 4))
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

function onOpenAppearance() {
  showAppearance.value = true
}

onMounted(async () => {
  appearance.load()
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
    <!-- Background image layer. Fixed, z-index 0, below content (z-index 1+).
         Must NOT use z-index: -1 — that paints under the solid body colour
         and the image never becomes visible. Image + opacity come from the
         appearance store as an inline style (not a CSS variable). -->
    <div
      class="app-background"
      aria-hidden="true"
      :style="appearance.backgroundLayerStyle"
    ></div>
    <WorkspaceBar
      @open-appearance="onOpenAppearance"
      @open-ai-settings="onOpenAISettings"
    />
    <div class="main-area">
      <FileTreePanel v-if="ws.hasWorkspace" :style="{ width: treeWidth + 'px' }" />
      <div
        v-if="ws.hasWorkspace"
        class="resize-handle"
        @mousedown="startResize('tree')"
      ></div>
      <TerminalPanel />
      <div
        v-if="ws.hasWorkspace"
        class="resize-handle"
        @mousedown="startResize('changes')"
      ></div>
      <FileChangesPanel v-if="ws.hasWorkspace" :style="{ width: changesWidth + 'px' }" />
    </div>
    <FilePreviewPanel v-if="ws.hasWorkspace && ws.previewFiles.length > 0" />
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
    <AppearanceSettings
      v-if="showAppearance"
      @close="showAppearance = false"
    />
  </div>
</template>

<style scoped>
.app-layout {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
  /* Transparent so the background layer can show through. The solid base
     colour lives on html/body; panels paint their own --surface-* colours. */
  background: transparent;
  z-index: 0;
}
.main-area {
  position: relative;
  z-index: 1;
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
/* Background image layer. Fixed at z-index 0; content sits at z-index 1+.
   Image URL and opacity are set via :style from the appearance store —
   do not put large base64 data URLs into CSS custom properties. */
.app-background {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
}
</style>
