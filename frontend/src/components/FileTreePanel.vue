<script setup lang="ts">
import { ref, watch, reactive } from 'vue'
import { DeleteWorkspaceFile, UploadWorkspaceFiles } from '../../wailsjs/go/main/App'
import { useWorkspaceStore } from '../stores/workspace'
import { getFileIcon } from '../utils/fileIcon'

const ws = useWorkspaceStore()

interface RemoteEntry {
  name: string
  path: string
  isDir: boolean
  size: number
  modTime: number
  isBinary?: boolean
}

interface TreeNode {
  name: string
  path: string
  isDir: boolean
  children: TreeNode[] | null  // null = not loaded, [] or [...] = loaded
  loading?: boolean
  isBinary?: boolean           // listed by name only, content never loaded
}

interface FlatNode {
  node: TreeNode
  depth: number
  padding: number
}

const tree = ref<TreeNode[]>([])
const expanded = ref<Set<string>>(new Set())
const actionError = ref('')
const contextMenu = ref<{ x: number; y: number; node: TreeNode } | null>(null)

function sortChildren(nodes: TreeNode[]) {
  nodes.sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
    if (a.name === '..') return -1
    if (b.name === '..') return 1
    return a.name.localeCompare(b.name)
  })
  for (const node of nodes) {
    if (node.isDir && node.children && node.children.length > 0) sortChildren(node.children)
  }
}

function entriesToTree(entries: RemoteEntry[]): TreeNode[] {
  return entries.map(e => ({
    name: e.name,
    path: e.path,
    isDir: e.isDir,
    children: e.isDir ? null : [],
    isBinary: !!e.isBinary,
  }))
}

// ── Local: static tree from flat file list ──
function buildTree(files: string[], binaryFiles: string[] = []): TreeNode[] {
  const root: TreeNode = { name: '', path: '', isDir: true, children: [] }
  const addPath = (file: string, isBinary: boolean) => {
    const parts = file.replace(/\\/g, '/').split('/')
    let current = root
    let currentPath = ''
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      currentPath = currentPath ? currentPath + '/' + part : part
      const isLast = i === parts.length - 1
      let child = current.children!.find(c => c.name === part)
      if (!child) {
        child = { name: part, path: currentPath, isDir: !isLast, children: [] }
        if (isLast) child.isBinary = isBinary
        current.children!.push(child)
      }
      if (!isLast) child.isDir = true
      current = child
    }
  }
  for (const file of files) addPath(file, false)
  for (const file of binaryFiles) addPath(file, true)
  sortChildren(root.children!)
  return root.children!
}

// ── Remote: lazy directory loading ──
async function initRemoteTree() {
  tree.value = []
  const entries = await ws.loadRemoteDir('')
  tree.value = entriesToTree(entries || [])
  sortChildren(tree.value)
}

async function loadRemoteChildren(node: TreeNode) {
  node.loading = true
  const entries = await ws.loadRemoteDir(node.path)
  node.children = entriesToTree(entries || [])
  node.loading = false
  sortChildren(node.children!)
  // Force reactivity for the tree
  tree.value = [...tree.value]
}

// ── Workspace change handler ──
watch(() => ws.info, async (newInfo) => {
  if (!newInfo) {
    tree.value = []
    return
  }
  if (newInfo.isRemote) {
    await initRemoteTree()
  } else if (newInfo.files || newInfo.otherFiles) {
    tree.value = buildTree(newInfo.files || [], newInfo.otherFiles || [])
  }
}, { immediate: true })

// ── Click handler ──
async function refreshTree() {
  actionError.value = ''
  if (ws.info?.isRemote) {
    await initRemoteTree()
  } else {
    await ws.refreshLocal()
  }
}

function getTargetDirectory(node?: TreeNode): string {
  if (!node || node.name === '..') return ''
  return node.isDir ? node.path : node.path.split('/').slice(0, -1).join('/')
}

function openContextMenu(event: MouseEvent, node: TreeNode) {
  event.preventDefault()
  contextMenu.value = { x: event.clientX, y: event.clientY, node }
}

function closeContextMenu() {
  contextMenu.value = null
}

async function uploadFiles(node?: TreeNode) {
  closeContextMenu()
  actionError.value = ''
  try {
    await UploadWorkspaceFiles(getTargetDirectory(node))
    await refreshTree()
  } catch (error: any) {
    actionError.value = error?.message || String(error)
  }
}

async function deleteFile(node: TreeNode) {
  closeContextMenu()
  if (node.isDir || node.name === '..') return
  if (!window.confirm(`确定删除“${node.name}”？该操作可通过文件变更列表还原。`)) return
  actionError.value = ''
  try {
    await DeleteWorkspaceFile(node.path)
    ws.closePreviewFile(node.path)
    await refreshTree()
  } catch (error: any) {
    actionError.value = error?.message || String(error)
  }
}

function handleClick(node: TreeNode) {
  if (node.isDir) {
    if (expanded.value.has(node.path)) {
      expanded.value.delete(node.path)
    } else {
      expanded.value.add(node.path)
      if (node.children === null) {
        loadRemoteChildren(node)
      }
    }
  } else if (!node.isBinary) {
    ws.openPreviewFile(node.path)
  }
}

function isExpanded(node: TreeNode): boolean {
  return expanded.value.has(node.path)
}

function onDragStart(event: DragEvent, node: TreeNode) {
	  if (node.isDir) return
	  const absPath = ws.getAbsolutePath(node.path)
	  event.dataTransfer?.setData('text/plain', absPath)
	  event.dataTransfer!.effectAllowed = 'copy'
	}

	function getIcon(node: TreeNode): string {
  if (node.name === '..') return '\u{1F519}'
  if (node.isDir) {
    if (node.loading) return '\u{23F3}'
    return isExpanded(node) ? '\u{1F4C2}' : '\u{1F4C1}'
  }
  return getFileIcon(node.name)
}

function renderTree(nodes: TreeNode[], depth: number = 0): FlatNode[] {
  const result: FlatNode[] = []
  for (const node of nodes) {
    result.push({ node, depth, padding: depth * 14 + 8 })
    if (node.isDir && isExpanded(node) && node.children) {
      result.push(...renderTree(node.children, depth + 1))
    }
  }
  return result
}
</script>

<template>
  <div class="file-tree-panel" @click="closeContextMenu">
    <div class="panel-header">
      <span>文件目录</span>
      <button class="tree-action" title="上传文件到工作区" @click.stop="uploadFiles()">上传</button>
      <button class="tree-action" title="刷新文件目录" @click.stop="refreshTree">↻</button>
    </div>
    <div class="tree-body">
      <div v-if="actionError" class="tree-error">{{ actionError }}</div>
      <div v-if="!ws.hasWorkspace" class="tree-empty">未选择工作区</div>
      <div v-else-if="tree.length === 0" class="tree-empty">加载中...</div>
      <div
        v-for="item in renderTree(tree)"
        :key="item.node.path"
        class="tree-node"
        :class="{ 'is-binary': item.node.isBinary }"
        :style="{ paddingLeft: item.padding + 'px' }"
        :draggable="!item.node.isDir"
        :title="item.node.isBinary ? item.node.name + ' — 二进制文件，不加载内容' : item.node.name"
        @click.stop="handleClick(item.node)"
        @contextmenu="openContextMenu($event, item.node)"
        @dragstart="onDragStart($event, item.node)"
      >
        <span class="node-icon">{{ getIcon(item.node) }}</span>
        <span class="node-name">{{ item.node.name }}</span>
        <span v-if="item.node.loading" class="node-loading">...</span>
      </div>
    </div>
    <div
      v-if="contextMenu"
      class="tree-context-menu"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
      @click.stop
    >
      <button @click="uploadFiles(contextMenu!.node)">上传到此处</button>
      <button v-if="!contextMenu.node.isDir && contextMenu.node.name !== '..'" class="danger" @click="deleteFile(contextMenu.node)">删除文件</button>
    </div>
  </div>
</template>

<style scoped>
.file-tree-panel {
  width: 220px;
  background: var(--surface-tree);
  border-right: 1px solid #2a2a2e;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex-shrink: 0;
}
.panel-header {
  padding: 8px 12px;
  font-size: 11px;
  font-weight: 600;
  color: #888;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 1px solid #2a2a2e;
  height: 32px;
  display: flex;
  align-items: center;
  gap: 4px;
}
.panel-header > span { margin-right: auto; }
.tree-action { border: 1px solid #3a3a3e; border-radius: 3px; background: #21262d; color: #8b949e; padding: 1px 5px; font-size: 10px; cursor: pointer; text-transform: none; }
.tree-action:hover { border-color: #58a6ff; color: #58a6ff; }
.tree-error { margin: 6px 8px; color: #f85149; font-size: 11px; line-height: 1.35; word-break: break-word; }
.tree-context-menu { position: fixed; z-index: 100; min-width: 132px; padding: 4px; border: 1px solid #3a3a3e; border-radius: 4px; background: #1e1e20; box-shadow: 0 8px 24px rgba(0, 0, 0, .45); }
.tree-context-menu button { display: block; width: 100%; border: 0; border-radius: 3px; background: transparent; color: #c9d1d9; padding: 5px 8px; text-align: left; font-size: 11px; cursor: pointer; }
.tree-context-menu button:hover { background: #30363d; }
.tree-context-menu button.danger { color: #f85149; }
.tree-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}
.tree-empty {
  padding: 20px 12px;
  text-align: center;
  color: #555;
  font-size: 12px;
}
.tree-node {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  cursor: pointer;
  font-size: 12px;
  color: #aaa;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  user-select: none;
}
.tree-node:hover {
  background: #1e1e22;
  color: #ddd;
}
/* Binary/oversized files: name only, not previewable */
.tree-node.is-binary {
  color: #6b6b73;
  cursor: default;
}
.tree-node.is-binary:hover {
  color: #8a8a93;
}
.node-icon {
  flex-shrink: 0;
  font-size: 11px;
}
.node-name {
  overflow: hidden;
  text-overflow: ellipsis;
}
.node-loading {
  color: #666;
  font-size: 10px;
}
</style>
