<script setup lang="ts">
import { useAppearanceStore } from '../stores/appearance'

defineEmits<{ (e: 'close'): void }>()

const ap = useAppearanceStore()

function shortPath(p: string): string {
  if (!p) return ''
  const parts = p.replace(/\\/g, '/').split('/')
  return parts.length <= 3 ? p : '.../' + parts.slice(-2).join('/')
}

function onBgInput(e: Event) {
  ap.setBackgroundOpacity(Number((e.target as HTMLInputElement).value) / 100)
}

function onPanelInput(e: Event) {
  ap.setPanelOpacity(Number((e.target as HTMLInputElement).value) / 100)
}

function pct(v: number): number {
  return Math.round(v * 100)
}
</script>

<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <div class="dialog" role="dialog" aria-label="外观设置">
      <div class="dialog-header">
        <span>外观设置</span>
        <button class="btn-close" aria-label="关闭" @click="$emit('close')">×</button>
      </div>

      <div class="dialog-body">
        <div class="section">
          <div class="label">背景图片</div>
          <div class="bg-row">
            <div class="bg-preview" :class="{ empty: !ap.hasBackground }">
              <img v-if="ap.hasBackground" :src="ap.imageData" alt="背景预览" />
              <span v-else>无图片</span>
            </div>
            <div class="bg-actions">
              <button class="btn" @click="ap.pickImage()">选择图片…</button>
              <button class="btn btn-danger" :disabled="!ap.backgroundImage" @click="ap.clearImage()">
                移除
              </button>
              <div class="bg-path" :title="ap.backgroundImage">
                {{ ap.backgroundImage ? shortPath(ap.backgroundImage) : '未设置' }}
              </div>
            </div>
          </div>
          <div class="hint">支持 PNG / JPG / GIF / WebP / BMP，最大 12MB。</div>
        </div>

        <div class="section">
          <div class="slider-head">
            <span class="label">背景不透明度</span>
            <span class="value">{{ pct(ap.backgroundOpacity) }}%</span>
          </div>
          <input
            class="slider"
            type="range"
            min="0"
            max="100"
            :value="pct(ap.backgroundOpacity)"
            :disabled="!ap.hasBackground"
            aria-label="背景不透明度"
            @input="onBgInput"
            @change="ap.persist()"
          />
          <div class="hint">数值越低，背景图越淡。</div>
        </div>

        <div class="section">
          <div class="slider-head">
            <span class="label">面板不透明度</span>
            <span class="value">{{ pct(ap.panelOpacity) }}%</span>
          </div>
          <input
            class="slider"
            type="range"
            :min="pct(ap.MIN_PANEL_OPACITY)"
            max="100"
            :value="pct(ap.panelOpacity)"
            aria-label="面板不透明度"
            @input="onPanelInput"
            @change="ap.persist()"
          />
          <div class="hint">降低后可透出背景图，最低 {{ pct(ap.MIN_PANEL_OPACITY) }}% 以保证可读性。</div>
        </div>

        <div v-if="ap.error" class="error">{{ ap.error }}</div>
      </div>

      <div class="dialog-footer">
        <button class="btn" @click="ap.reset()">恢复默认</button>
        <button class="btn btn-primary" @click="$emit('close')">完成</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.dialog {
  background: #1e1e20;
  border: 1px solid #444;
  border-radius: 8px;
  width: 460px;
  display: flex;
  flex-direction: column;
}
.dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #333;
  font-size: 14px;
  color: #ddd;
  font-weight: 600;
}
.btn-close {
  background: none;
  border: none;
  color: #888;
  font-size: 18px;
  cursor: pointer;
}
.btn-close:hover { color: #fff; }
.dialog-body {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.section { display: flex; flex-direction: column; gap: 8px; }
.label { font-size: 12px; color: #aaa; }
.hint { font-size: 11px; color: #666; }
.bg-row { display: flex; gap: 12px; }
.bg-preview {
  width: 132px;
  height: 78px;
  border: 1px solid #333;
  border-radius: 4px;
  overflow: hidden;
  flex-shrink: 0;
  background: #0d1117;
  display: flex;
  align-items: center;
  justify-content: center;
}
.bg-preview img { width: 100%; height: 100%; object-fit: cover; }
.bg-preview.empty { color: #555; font-size: 12px; }
.bg-actions {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: flex-start;
  min-width: 0;
  flex: 1;
}
.bg-path {
  font-size: 11px;
  color: #777;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
.slider-head { display: flex; align-items: center; justify-content: space-between; }
.value { font-size: 12px; color: #58a6ff; font-variant-numeric: tabular-nums; }
.slider { width: 100%; accent-color: #58a6ff; cursor: pointer; }
.slider:disabled { cursor: default; opacity: 0.45; }
.btn {
  background: #21262d;
  border: 1px solid #30363d;
  color: #c9d1d9;
  padding: 5px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}
.btn:hover { background: #30363d; }
.btn:disabled { opacity: 0.45; cursor: default; }
.btn-danger:hover { color: #f85149; border-color: #f85149; background: #21262d; }
.btn-primary { background: #238636; border-color: #2ea043; color: #fff; font-weight: 600; }
.btn-primary:hover { background: #2ea043; }
.error { font-size: 12px; color: #f85149; }
.dialog-footer {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid #333;
}
</style>
