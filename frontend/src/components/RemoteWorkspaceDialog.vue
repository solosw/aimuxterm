<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { GetSSHConfigs, OpenRemoteWorkspace } from '../../wailsjs/go/main/App'
import { useWorkspaceStore } from '../stores/workspace'

const emit = defineEmits<{ (e: 'close'): void }>()
const ws = useWorkspaceStore()

const savedConfigs = ref<any[]>([])
const syncButtonText = computed(() => {
  if (!syncing.value) return '同步并打开'
  const p = ws.snapshotProgress
  if (p && p.total > 0) {
    return `正在初始化快照 (${p.current}/${p.total})...`
  }
  return '同步中...'
})
const selectedCfg = ref<any>(null)
const remotePath = ref('')
const syncing = ref(false)
const error = ref('')

async function loadConfigs() {
  try { savedConfigs.value = await GetSSHConfigs() || [] } catch {}
}

async function connect() {
  if (!selectedCfg.value) { error.value = '请选择SSH连接'; return }
  if (!remotePath.value.trim()) { error.value = '请输入远程目录路径'; return }

  syncing.value = true
  error.value = ''
  try {
    const info = await OpenRemoteWorkspace(selectedCfg.value, remotePath.value.trim())
    if (info) {
      ws.info = info
    }
    emit('close')
  } catch (e: any) {
    error.value = '连接失败: ' + (e?.message || String(e))
  }
  syncing.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}
onMounted(() => {
  document.addEventListener('keydown', onKeydown)
  loadConfigs()
})
onUnmounted(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="overlay" @click.self="emit('close')">
    <div class="dialog">
      <div class="dialog-header">
        <span>远程工作区</span>
        <button class="btn-close" @click="emit('close')">&times;</button>
      </div>

      <div class="dialog-body">
        <label class="label">SSH 连接</label>
        <div class="ssh-list">
          <div v-if="savedConfigs.length === 0" class="empty-hint">暂无保存的SSH连接，请先在终端栏创建SSH连接</div>
          <div v-for="cfg in savedConfigs" :key="cfg.name" class="ssh-item"
            :class="{ selected: selectedCfg?.name === cfg.name }"
            @click="selectedCfg = cfg">
            <span class="ssh-name">{{ cfg.name || cfg.host }}</span>
            <span class="ssh-addr">{{ cfg.user }}@{{ cfg.host }}:{{ cfg.port }}</span>
          </div>
        </div>

        <label class="label">远程目录</label>
        <input v-model="remotePath" placeholder="/home/user/project" class="input"
          @keydown.enter="connect" />

        <div v-if="error" class="error">{{ error }}
          <br/><small v-if="selectedCfg">目标: {{ selectedCfg.user }}@{{ selectedCfg.host }}:{{ selectedCfg.port }}{{ remotePath }}</small>
        </div>
      </div>

      <div class="dialog-footer">
        <button class="btn-cancel" @click="emit('close')">取消</button>
        <button class="btn-go" :disabled="syncing" @click="connect">
          {{ syncButtonText }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,0.6);
  display: flex; align-items: center; justify-content: center; z-index: 200;
}
.dialog {
  background: #1e1e20; border: 1px solid #444; border-radius: 8px;
  width: 440px; display: flex; flex-direction: column;
}
.dialog-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 16px; border-bottom: 1px solid #333;
  font-size: 14px; color: #ddd; font-weight: 600;
}
.btn-close { background: none; border: none; color: #888; font-size: 18px; cursor: pointer; }
.btn-close:hover { color: #fff; }

.dialog-body { padding: 16px; display: flex; flex-direction: column; gap: 10px; }
.label { font-size: 12px; color: #888; }
.ssh-list { max-height: 160px; overflow-y: auto; border: 1px solid #333; border-radius: 4px; }
.empty-hint { padding: 12px; color: #666; font-size: 12px; text-align: center; }
.ssh-item { display: flex; flex-direction: column; gap: 2px; padding: 8px 12px; cursor: pointer; border-bottom: 1px solid #222; }
.ssh-item:hover { background: #2a2a2e; }
.ssh-item.selected { background: #1a3a5c; }
.ssh-name { font-size: 13px; color: #ddd; font-weight: 600; }
.ssh-addr { font-size: 11px; color: #888; }
.input {
  background: #0d1117; border: 1px solid #30363d; color: #c9d1d9;
  padding: 8px 10px; border-radius: 4px; font-size: 13px; outline: none;
}
.input:focus { border-color: #58a6ff; }
.error { color: #f85149; font-size: 12px; }

.dialog-footer {
  display: flex; justify-content: flex-end; gap: 8px;
  padding: 12px 16px; border-top: 1px solid #333;
}
.btn-cancel {
  background: #21262d; border: 1px solid #30363d; color: #c9d1d9;
  padding: 6px 16px; border-radius: 4px; cursor: pointer; font-size: 13px;
}
.btn-cancel:hover { background: #30363d; }
.btn-go {
  background: #238636; border: 1px solid #2ea043; color: #fff;
  padding: 6px 20px; border-radius: 4px; cursor: pointer; font-size: 13px; font-weight: 600;
}
.btn-go:hover { background: #2ea043; }
.btn-go:disabled { opacity: 0.5; cursor: default; }
</style>
