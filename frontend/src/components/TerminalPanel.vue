<script setup lang="ts">
import { computed, ref } from 'vue'
import { useTerminalStore } from '../stores/terminal'
import { useWorkspaceStore } from '../stores/workspace'
import TerminalView from './TerminalView.vue'
import SSHConnectDialog from './SSHConnectDialog.vue'

const store = useTerminalStore()
const ws = useWorkspaceStore()
const showSSHDialog = ref(false)
const showCmdInput = ref(false)

const gridCols = computed(() => {
  const n = store.tabs.length
  if (n <= 1) return 1
  if (n <= 4) return 2
  return 3
})
</script>

<template>
  <div class="main-panel">
    <div class="tab-bar">
      <div
        v-for="tab in store.tabs"
        :key="tab.id"
        class="tab"
        :class="{ active: tab.id === store.activeTabId && store.layoutMode === 'tabs' }"
        @click="store.setActive(tab.id)"
      >
        <span class="tab-type">></span>
        <span>{{ tab.title }}</span>
        <button class="tab-close" @click.stop="store.closeTab(tab.id)">×</button>
      </div>
      <button class="tab-new" @click="ws.showStartupPicker = true">+</button>
      <button class="tab-ssh" @click="showSSHDialog = true" title="SSH连接">&#x1F50C;</button>
      <div class="tab-spacer"></div>
      <button
        class="btn-layout"
        :title="store.layoutMode === 'tabs' ? '切换到网格布局' : '切换到标签布局'"
        @click="store.toggleLayout()"
      >
        {{ store.layoutMode === 'tabs' ? '⊞' : '⊟' }}
      </button>
      <button
        class="btn-cmd-input"
        :class="{ active: showCmdInput }"
        :title="showCmdInput ? '隐藏命令输入栏' : '显示命令输入栏'"
        @click="showCmdInput = !showCmdInput"
      >
        &#x2328;
      </button>
    </div>

    <div v-if="store.tabs.length === 0" class="no-tabs">
      <p v-if="store.error" class="error-msg">{{ store.error }}</p>
      <p v-else>点击 + 创建终端</p>
    </div>

    <!-- Grid layout -->
    <div v-else-if="store.layoutMode === 'grid'" class="grid-body" :style="{ gridTemplateColumns: `repeat(${gridCols}, 1fr)` }">
      <div v-for="tab in store.tabs" :key="tab.id" class="grid-cell">
        <div class="grid-cell-header">
          <span class="grid-cell-title">{{ tab.title }}</span>
          <button class="grid-cell-close" @click="store.closeTab(tab.id)">×</button>
        </div>
        <div class="grid-cell-body">
          <TerminalView :tab-id="tab.id" :show-cmd-input="showCmdInput" />
        </div>
      </div>
    </div>

    <!-- Tab layout -->
    <div v-else class="tab-body">
      <template v-for="tab in store.tabs" :key="tab.id">
        <div v-show="tab.id === store.activeTabId" class="tab-content">
          <TerminalView :tab-id="tab.id" :show-cmd-input="showCmdInput" />
        </div>
      </template>
    </div>
    <SSHConnectDialog
      v-if="showSSHDialog"
      @close="showSSHDialog = false"
      @connected="(id, title) => { store.addSSHTab(id, title); showSSHDialog = false }"
    />
  </div>
</template>

<style scoped>
.main-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.tab-bar {
  display: flex;
  align-items: center;
  background: var(--surface-bar);
  border-bottom: 1px solid #333;
  height: 32px;
  padding: 0 4px;
  gap: 2px;
  overflow-x: auto;
  flex-shrink: 0;
}
.tab {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  border-radius: 4px 4px 0 0;
  cursor: pointer;
  font-size: 12px;
  color: #999;
  white-space: nowrap;
  user-select: none;
}
.tab.active { background: #161618; color: #fff; }
.tab:hover { background: #2a2a2e; }
.tab-type { font-size: 10px; flex-shrink: 0; }
.tab-close {
  background: none;
  border: none;
  color: #666;
  cursor: pointer;
  font-size: 14px;
  padding: 0;
  line-height: 1;
}
.tab-close:hover { color: #fff; }
.tab-new {
  background: none;
  border: none;
  color: #888;
  cursor: pointer;
  font-size: 16px;
  padding: 0 10px;
}
.tab-new:hover { color: #fff; }
.tab-ssh {
  background: none; border: none; color: #888; cursor: pointer;
  font-size: 12px; padding: 0 8px;
}
.tab-ssh:hover { color: #58a6ff; }
.tab-spacer { flex: 1; }
.btn-layout {
  background: none;
  border: 1px solid #444;
  color: #888;
  cursor: pointer;
  font-size: 14px;
  padding: 0 8px;
  border-radius: 3px;
  line-height: 22px;
}
.btn-layout:hover { color: #fff; border-color: #666; }
.btn-cmd-input {
  background: none;
  border: 1px solid #444;
  color: #888;
  cursor: pointer;
  font-size: 14px;
  padding: 0 8px;
  border-radius: 3px;
  line-height: 22px;
}
.btn-cmd-input:hover { color: #fff; border-color: #666; }
.btn-cmd-input.active { color: #58a6ff; border-color: #58a6ff; }
.tab-body {
  flex: 1;
  display: flex;
  overflow: hidden;
  /* Transparent so xterm canvas opacity can show the app background.
     The dimmed fill lives on the canvas via TerminalView's allowTransparency. */
  background: transparent;
}
.no-tabs {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #666;
  font-size: 14px;
  gap: 8px;
}
.error-msg { color: #f44336; font-size: 13px; }
.tab-content {
  flex: 1;
  display: flex;
  overflow: hidden;
}
.grid-body {
  flex: 1;
  display: grid;
  gap: 2px;
  /* Transparent so multi-pane terminals can still show the app background. */
  background: transparent;
  overflow: hidden;
}
.grid-cell {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: transparent;
  border: 1px solid #2a2a2e;
}
.grid-cell-header {
  display: flex;
  align-items: center;
  padding: 2px 8px;
  background: #1e1e20;
  border-bottom: 1px solid #2a2a2e;
  height: 24px;
}
.grid-cell-title { flex: 1; font-size: 11px; color: #888; }
.grid-cell-close {
  background: none;
  border: none;
  color: #555;
  cursor: pointer;
  font-size: 14px;
  padding: 0 2px;
  line-height: 1;
}
.grid-cell-close:hover { color: #f44336; }
.grid-cell-body { flex: 1; overflow: hidden; }
</style>
