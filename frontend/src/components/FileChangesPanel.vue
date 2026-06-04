<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import DiffView from './DiffView.vue'
import { useFileChangesStore } from '../stores/fileChanges'
import { useWorkspaceStore } from '../stores/workspace'
import { snapshot } from '../../wailsjs/go/models'
import { detectLang } from '../utils/detectLang'
import { getFileIcon } from '../utils/fileIcon'
import type { FileDiff } from '../types'

const store = useFileChangesStore()
const ws = useWorkspaceStore()
const viewingDiff = ref<string | null>(null)
const refreshing = ref(false)

async function doRefresh() {
  refreshing.value = true
  try {
    if (ws.info?.isRemote) {
      await ws.refreshRemote()
    } else {
      await ws.refreshLocal()
    }
  } finally {
    refreshing.value = false
  }
}
const diffContent = ref<FileDiff | null>(null)

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && viewingDiff.value) {
    closeDiff()
  }
}
onMounted(() => document.addEventListener('keydown', onKeydown))
onUnmounted(() => document.removeEventListener('keydown', onKeydown))

const statusLabel: Record<string, string> = {
  added: '+新增',
  modified: '~修改',
  deleted: '-删除'
}

const statusClass: Record<string, string> = {
  added: 'status-added',
  modified: 'status-modified',
  deleted: 'status-deleted'
}

async function showDiff(file: snapshot.FileChange) {
  viewingDiff.value = file.path
  diffContent.value = await store.getDiff(file.path)
}

function onDragStart(event: DragEvent, relPath: string) {
  const absPath = ws.getAbsolutePath(relPath)
  event.dataTransfer?.setData('text/plain', absPath)
  event.dataTransfer!.effectAllowed = 'copy'
}

function closeDiff() {
  viewingDiff.value = null
  diffContent.value = null
}
</script>

<template>
  <div class="changes-panel">
    <div class="panel-header">
      文件变更
      <button class="btn-refresh" :disabled="refreshing" @click="doRefresh" title="手动刷新">
        {{ refreshing ? '...' : '↻' }}
      </button>
    </div>

    <div v-if="!store.hasChanges" class="panel-empty">无变更</div>

    <div v-else class="panel-body">
      <div
        v-for="f in store.changes"
        :key="f.path"
        class="file-item"
        :class="statusClass[f.status]"
        draggable="true"
        @click="showDiff(f)"
        @dragstart="onDragStart($event, f.path)"
      >
        <span class="file-icon">{{ getFileIcon(f.path) }}</span>
        <span class="status-badge">{{ statusLabel[f.status] }}</span>
        <span class="file-path">{{ f.path }}</span>
        <span class="diff-stats">
          <template v-if="f.additions"><span class="stat-add">+{{ f.additions }}</span></template>
          <template v-if="f.deletions"><span class="stat-del">-{{ f.deletions }}</span></template>
        </span>
        <div class="file-actions">
          <button class="btn-accept" @click.stop="store.acceptFile(f.path)" title="接受">&#x2713;</button>
          <button class="btn-revert" @click.stop="store.revertFile(f.path)" title="回退">&#x21A9;</button>
        </div>
      </div>
    </div>

    <div v-if="store.hasChanges" class="panel-footer">
      <button class="btn-all btn-accept-all" @click="store.acceptAll()">接受全部</button>
      <button class="btn-all btn-revert-all" @click="store.revertAll()">回退全部</button>
    </div>

    <div v-if="viewingDiff" class="diff-overlay" @click="closeDiff">
      <div class="diff-modal" @click.stop>
        <div class="diff-header">
          <span>{{ viewingDiff }}</span>
          <button class="btn-close" @click.stop="closeDiff">&times;</button>
        </div>
        <div class="diff-body">
          <DiffView
            v-if="diffContent"
            :old-string="diffContent.old"
            :new-string="diffContent.new"
            :language="detectLang(viewingDiff!)"
            :file-path="viewingDiff!"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.changes-panel {
  width: 280px;
  background: #1a1a1c;
  border-left: 1px solid #333;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.panel-header {
  padding: 8px 12px;
  font-size: 13px;
  font-weight: 600;
  color: #aaa;
  border-bottom: 1px solid #333;
  height: 36px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.btn-refresh {
  background: none; border: 1px solid #3a3a3e; color: #888;
  cursor: pointer; font-size: 14px; padding: 0 6px; border-radius: 3px;
  line-height: 20px;
}
.btn-refresh:hover { color: #fff; border-color: #58a6ff; }
.btn-refresh:disabled { opacity: 0.5; cursor: default; }
.panel-empty {
  padding: 20px;
  text-align: center;
  color: #555;
  font-size: 13px;
}
.panel-body {
  flex: 1;
  overflow-y: auto;
}
.file-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  cursor: pointer;
  border-bottom: 1px solid #222;
  font-size: 12px;
}
.file-item:hover { background: #2a2a2e; }
.status-badge {
  font-size: 11px;
  padding: 0 4px;
  border-radius: 3px;
  flex-shrink: 0;
}
.status-added .status-badge { color: #4caf50; }
.status-modified .status-badge { color: #ff9800; }
.status-deleted .status-badge { color: #f44336; }
.file-icon { flex-shrink: 0; font-size: 13px; }
.file-path {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #ccc;
}
.diff-stats {
  display: flex;
  gap: 4px;
  font-size: 11px;
  font-family: Consolas, monospace;
  flex-shrink: 0;
}
.stat-add { color: #3fb950; }
.stat-del { color: #f85149; }
.file-actions {
  display: flex;
  gap: 2px;
  opacity: 0;
}
.file-item:hover .file-actions { opacity: 1; }
.btn-accept, .btn-revert {
  background: none;
  border: 1px solid #444;
  color: #aaa;
  cursor: pointer;
  font-size: 11px;
  padding: 1px 5px;
  border-radius: 3px;
}
.btn-accept:hover { background: #2e7d32; color: #fff; border-color: #2e7d32; }
.btn-revert:hover { background: #c62828; color: #fff; border-color: #c62828; }

.panel-footer {
  padding: 8px;
  border-top: 1px solid #333;
  display: flex;
  gap: 6px;
}
.btn-all {
  flex: 1;
  padding: 6px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
}
.btn-accept-all { background: #2e7d32; color: #fff; }
.btn-accept-all:hover { background: #388e3c; }
.btn-revert-all { background: #c62828; color: #fff; }
.btn-revert-all:hover { background: #d32f2f; }

.diff-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.diff-modal {
  background: #0d1117;
  border: 1px solid #30363d;
  border-radius: 8px;
  width: 88vw;
  max-width: 1100px;
  height: 80vh;
  display: flex;
  flex-direction: column;
}
.diff-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid #30363d;
  font-size: 13px;
  color: #ccc;
  flex-shrink: 0;
}
.btn-close {
  background: none;
  border: none;
  color: #888;
  font-size: 18px;
  cursor: pointer;
}
.btn-close:hover { color: #fff; }
.diff-body { flex: 1; overflow: auto; min-height: 0; }
</style>
