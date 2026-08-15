import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { SelectWorkspace, OpenWorkspace, GetWorkspaceInfo, GetWorkspaceHistory, RemoveWorkspaceFromHistory, OpenInNewWindow, RefreshLocalWorkspace, RefreshRemoteWorkspace, GetRemoteWorkspaces, RemoveRemoteWorkspace, OpenRemoteWorkspace, ListRemoteDir } from '../../wailsjs/go/main/App'
import { main, config } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { useFileChangesStore } from './fileChanges'

export const useWorkspaceStore = defineStore('workspace', () => {
  const info = ref<main.WorkspaceInfo | null>(null)
  const history = ref<config.WorkspaceEntry[]>([])
  const remoteList = ref<config.RemoteWorkspaceEntry[]>([])
  const hasWorkspace = computed(() => info.value !== null && info.value.path !== '')

  const previewFiles = ref<string[]>([])
  const activePreviewFile = ref<string | null>(null)
  const previewReloadVersion = ref(0)
  const previewReloadPaths = ref<string[]>([])
  const showStartupPicker = ref(false)
  const snapshotProgress = ref<{ total: number; current: number } | null>(null)

  EventsOn('snapshot-progress', (data: any) => {
    if (data?.phase === 'start' || data?.phase === 'progress') {
      snapshotProgress.value = { total: data.total, current: data.current }
    } else {
      snapshotProgress.value = null
    }
  })

  function openPreviewFile(path: string) {
    if (!previewFiles.value.includes(path)) previewFiles.value.push(path)
    activePreviewFile.value = path
  }
  function closePreviewFile(path: string) {
    const idx = previewFiles.value.indexOf(path)
    if (idx !== -1) previewFiles.value.splice(idx, 1)
    if (activePreviewFile.value === path) {
      activePreviewFile.value = previewFiles.value.length > 0 ? previewFiles.value[previewFiles.value.length - 1] : null
    }
  }

  function replacePreviewPath(oldPath: string, newPath: string, isDirectory = false) {
    const prefix = oldPath.replace(/\\/g, '/').replace(/\/+$/, '')
    const target = newPath.replace(/\\/g, '/').replace(/\/+$/, '')
    const mapPath = (current: string) => {
      const normalized = current.replace(/\\/g, '/')
      if (normalized === prefix) return target
      if (isDirectory && normalized.startsWith(prefix + '/')) return target + normalized.slice(prefix.length)
      return current
    }
    const seen = new Set<string>()
    previewFiles.value = previewFiles.value.map(mapPath).filter(path => {
      if (seen.has(path)) return false
      seen.add(path)
      return true
    })
    if (activePreviewFile.value) activePreviewFile.value = mapPath(activePreviewFile.value)
  }

  function closePreviewPaths(paths: string[]) {
    const normalized = paths.map(path => path.replace(/\\/g, '/').replace(/\/+$/, ''))
    for (const path of [...previewFiles.value]) {
      const current = path.replace(/\\/g, '/')
      if (normalized.some(target => current === target || current.startsWith(target + '/'))) closePreviewFile(path)
    }
  }

  function reloadPreviewFiles(paths: string[]) {
    previewReloadPaths.value = paths.filter(path => previewFiles.value.includes(path))
    if (previewReloadPaths.value.length) previewReloadVersion.value++
  }

  async function syncChanges() { const fc = useFileChangesStore(); await fc.refresh() }

  async function loadHistory() {
    history.value = (await GetWorkspaceHistory()) || []
    remoteList.value = (await GetRemoteWorkspaces()) || []
  }

  async function selectWorkspace() {
    const r = await SelectWorkspace()
    if (r) { info.value = r; syncChanges(); await loadHistory(); showStartupPicker.value = true }
    return r
  }

  async function openWorkspace(path: string) {
    const r = await OpenWorkspace(path)
    if (r) { info.value = r; syncChanges(); await loadHistory(); showStartupPicker.value = true }
    return r
  }

  async function openRemoteWorkspace(entry: config.RemoteWorkspaceEntry) {
    const r = await OpenRemoteWorkspace(entry as any, entry.remotePath)
    if (r) { info.value = r; syncChanges(); await loadHistory(); showStartupPicker.value = true }
    return r
  }

  async function removeRemote(name: string) {
    await RemoveRemoteWorkspace(name)
    await loadHistory()
  }

  async function openInNewWindow(path: string) { await OpenInNewWindow(path) }
  async function removeFromHistory(path: string) { await RemoveWorkspaceFromHistory(path); await loadHistory() }
  async function refresh() { info.value = await GetWorkspaceInfo() }
  async function refreshLocal() { const r = await RefreshLocalWorkspace(); if (r) { info.value = r; syncChanges() } }
  async function refreshRemote() { const r = await RefreshRemoteWorkspace(); if (r) { info.value = r; syncChanges() } }
  async function loadRemoteDir(dir: string) { return await ListRemoteDir(dir) }

  function getAbsolutePath(relPath: string): string {
    if (!info.value) return relPath
    let base = info.value.path
    if (info.value.isRemote) {
      const idx = base.indexOf(':')
      if (idx >= 0) base = base.substring(idx + 1)
    }
    base = base.replace(/\\/g, '/')
    return base + '/' + relPath
  }

  return {
    info, history, remoteList, hasWorkspace,
    previewFiles, activePreviewFile, previewReloadVersion, previewReloadPaths, openPreviewFile, closePreviewFile, replacePreviewPath, closePreviewPaths, reloadPreviewFiles,
    syncChanges, loadHistory, selectWorkspace, openWorkspace, openRemoteWorkspace, removeRemote,
    openInNewWindow, removeFromHistory, refresh, refreshLocal, refreshRemote, loadRemoteDir,
    snapshotProgress, showStartupPicker, getAbsolutePath
  }
})
