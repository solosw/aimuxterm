<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ApplyAIConfigGroup, DetectAIToolConfigPaths, GetAIConfigGroups, SaveAIConfigGroups } from '../../wailsjs/go/main/App'
import { config, main } from '../../wailsjs/go/models'
import { useAICompletionStore } from '../stores/aiCompletion'

const emit = defineEmits(['close'])
const aiCompletion = useAICompletionStore()
const groups = ref<any[]>([])
const selected = ref(0)
const paths = ref<any | null>(null)
const status = ref('')
const currentGroup = computed(() => groups.value[selected.value] || null)

function makeEmptyGroup() {
  return ({
    name: '新配置组',
    apiKey: '',
    baseURL: '',
    models: [''],
    claudeCode: ({ opusIndex: 0, sonnetIndex: 0, haikuIndex: 0 })
  })
}

onMounted(async () => {
  aiCompletion.load()
  groups.value = (await GetAIConfigGroups()) || []
  if (groups.value.length === 0) groups.value = [makeEmptyGroup()]
  paths.value = await DetectAIToolConfigPaths()
})

async function save() {
  await SaveAIConfigGroups(groups.value as any)
  aiCompletion.save()
  status.value = '已保存'
}

function addGroup() {
  groups.value.push(makeEmptyGroup())
  selected.value = groups.value.length - 1
}

function removeGroup(idx: number) {
  groups.value.splice(idx, 1)
  if (groups.value.length === 0) groups.value.push(makeEmptyGroup())
  selected.value = Math.max(0, Math.min(selected.value, groups.value.length - 1))
}

function addModel() {
  if (!currentGroup.value) return
  currentGroup.value.models.push('')
}

function removeModel(idx: number) {
  const g = currentGroup.value
  if (!g) return
  g.models.splice(idx, 1)
  if (g.models.length === 0) g.models.push('')
}

async function apply(target: string) {
  await save()
  if (!currentGroup.value) return
  await ApplyAIConfigGroup(currentGroup.value as any, target)
  status.value = target === 'all' ? '已应用到全部工具' : `已应用到 ${target}`
}
</script>

<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="modal-content ai-modal">
      <div class="modal-header">
        <span>AI 配置管理</span>
        <button class="btn-close" @click="emit('close')">&times;</button>
      </div>
      <div class="modal-body ai-body">
        <div class="group-list">
          <div v-for="(g, i) in groups" :key="i" class="group-item" :class="{ active: i === selected }" @click="selected = i">
            <span>{{ g.name || '未命名组' }}</span>
            <button class="btn-remove-saved" @click.stop="removeGroup(i)">&times;</button>
          </div>
          <button class="btn" @click="addGroup">+ 新增组</button>
        </div>

        <div v-if="currentGroup" class="group-editor">
          <div class="form-row">
            <label>组名</label>
            <input v-model="currentGroup.name" />
          </div>
          <div class="form-row">
            <label>API Key</label>
            <input v-model="currentGroup.apiKey" type="password" />
          </div>
          <div class="form-row">
            <label>Base URL</label>
            <input v-model="currentGroup.baseURL" />
          </div>

          <div class="section-title">模型池</div>
          <div v-for="(m, idx) in currentGroup.models" :key="idx" class="cmd-row">
            <input v-model="currentGroup.models[idx]" class="input-sm flex-1" placeholder="模型 ID" />
            <button class="btn-sm btn-danger" @click="removeModel(idx)">删除</button>
          </div>
          <button class="btn" @click="addModel">+ 添加模型</button>

          <div class="section-title">Claude Code 槽位</div>
          <div class="form-row">
            <label>Opus</label>
            <select v-model.number="currentGroup.claudeCode.opusIndex">
              <option v-for="(m, i) in currentGroup.models" :key="'o'+i" :value="i">{{ i }} - {{ m }}</option>
            </select>
          </div>
          <div class="form-row">
            <label>Sonnet</label>
            <select v-model.number="currentGroup.claudeCode.sonnetIndex">
              <option v-for="(m, i) in currentGroup.models" :key="'s'+i" :value="i">{{ i }} - {{ m }}</option>
            </select>
          </div>
          <div class="form-row">
            <label>Haiku</label>
            <select v-model.number="currentGroup.claudeCode.haikuIndex">
              <option v-for="(m, i) in currentGroup.models" :key="'h'+i" :value="i">{{ i }} - {{ m }}</option>
            </select>
          </div>

          <div class="section-title">编辑器 AI 自动补全</div>
          <div class="completion-note">使用 OpenAI 兼容的 FIM 补全接口（/v1/completions）。可按 Ctrl/Cmd + B 手动触发；开启自动触发后，停止输入约 700ms 会请求补全。</div>
          <div class="form-row">
            <label>启用补全</label>
            <input v-model="aiCompletion.config.enabled" type="checkbox" />
          </div>
          <div class="form-row">
            <label>自动触发</label>
            <input v-model="aiCompletion.config.autoTrigger" type="checkbox" :disabled="!aiCompletion.config.enabled" />
          </div>
          <div class="form-row">
            <label>API Key</label>
            <input v-model="aiCompletion.config.apiKey" type="password" autocomplete="off" :disabled="!aiCompletion.config.enabled" placeholder="Bearer Token" />
          </div>
          <div class="form-row">
            <label>Base URL</label>
            <input v-model="aiCompletion.config.baseUrl" :disabled="!aiCompletion.config.enabled" placeholder="https://api.example.com" />
          </div>
          <div class="form-row">
            <label>补全模型</label>
            <input v-model="aiCompletion.config.model" :disabled="!aiCompletion.config.enabled" placeholder="模型 ID" />
          </div>

          <div class="section-title">探测到的配置路径</div>
          <div class="path-item">Claude Code: {{ paths?.claudeCode || '未找到' }}</div>
          <div class="path-item">Codex: {{ paths?.codex || '未找到' }}</div>
          <div class="path-item">OpenCode: {{ paths?.openCode || '未找到' }}</div>
        </div>
      </div>
      <div class="modal-footer footer-actions">
        <span class="status">{{ status }}</span>
        <button class="btn" @click="save">保存</button>
        <button class="btn" @click="apply('claudeCode')">应用 Claude Code</button>
        <button class="btn" @click="apply('codex')">应用 Codex</button>
        <button class="btn" @click="apply('openCode')">应用 OpenCode</button>
        <button class="btn btn-primary" @click="apply('all')">应用全部</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal-content { background: #1a1a1e; border: 1px solid #3a3a3e; border-radius: 8px; display: flex; flex-direction: column; }
.ai-modal { width: 900px; max-height: 80vh; }
.modal-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid #2a2a2e; font-size: 14px; font-weight: 600; }
.modal-body { padding: 12px 16px; overflow-y: auto; flex: 1; }
.ai-body { display: flex; gap: 16px; }
.group-list { width: 220px; border-right: 1px solid #2a2a2e; padding-right: 12px; display: flex; flex-direction: column; gap: 8px; }
.group-item { display: flex; justify-content: space-between; align-items: center; padding: 8px 10px; background: #202024; border: 1px solid #333; border-radius: 4px; cursor: pointer; }
.group-item.active { border-color: #4a9eff; background: #1a2a3c; }
.group-editor { flex: 1; display: flex; flex-direction: column; gap: 10px; }
.form-row { display: flex; align-items: center; gap: 8px; }
.form-row label { width: 90px; font-size: 12px; color: #888; flex-shrink: 0; }
.form-row input, .form-row select { flex: 1; background: #111; border: 1px solid #3a3a3e; color: #ddd; padding: 6px 8px; border-radius: 4px; font-size: 12px; }
.section-title { margin-top: 8px; color: #8fb; font-size: 12px; font-weight: 600; }
.completion-note { color: #8b949e; font-size: 11px; line-height: 1.45; }
.cmd-row { display: flex; align-items: center; gap: 8px; }
.input-sm { background: #111; border: 1px solid #3a3a3e; color: #ddd; padding: 6px 8px; border-radius: 4px; font-size: 12px; }
.input-sm.flex-1 { flex: 1; }
.path-item { color: #999; font-size: 12px; word-break: break-all; background: #111; padding: 6px 8px; border-radius: 4px; }
.modal-footer { padding: 10px 16px; border-top: 1px solid #2a2a2e; }
.footer-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.status { color: #8f8; margin-right: auto; font-size: 12px; }
.btn-close { background: none; border: none; color: #888; font-size: 18px; cursor: pointer; }
.btn, .btn-sm, .btn-remove-saved { background: #2a2a2e; border: 1px solid #3a3a3e; color: #ccc; padding: 4px 10px; border-radius: 4px; cursor: pointer; font-size: 12px; }
.btn:hover, .btn-sm:hover, .btn-remove-saved:hover { background: #3a3a3e; }
.btn-primary { background: #2563eb; border-color: #2563eb; color: #fff; }
.btn-primary:hover { background: #1d4ed8; }
.btn-danger { color: #f66; }
.btn-danger:hover { background: #3a1a1a; }
</style>
