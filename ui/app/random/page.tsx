"use client"

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import {
  AlertTriangle,
  Box,
  FolderOpen,
  Loader2,
  RefreshCw,
  Shuffle,
  Sparkles,
  Trash2,
  Image as ImageIcon,
} from 'lucide-react'

interface FileInfo {
  name: string
  path: string
  size: number
  mod_time: string
}

interface SizeGroup {
  size: number
  files: FileInfo[]
}

interface SimilarityGroup {
  base_name: string
  files: FileInfo[]
}

interface Report {
  total_files: number
  size_groups: SizeGroup[]
  similar_groups: SimilarityGroup[]
  visual_groups: SimilarityGroup[]
  visual_count: number
  analysis_duration_seconds: number
  status?: string
}

type FileType = 'all' | 'zip' | 'rar' | '7z'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function getApiHost() {
  return window.location.port === '3000' ? 'http://localhost:8080' : ''
}

function getGroupSize(group: SizeGroup | SimilarityGroup) {
  if ('size' in group) return group.size
  return group.files.reduce((sum, file) => sum + file.size, 0)
}

function groupMatchesType(group: SizeGroup | SimilarityGroup, type: FileType) {
  if (type === 'all') return true
  return group.files.some((file) => file.path.toLowerCase().endsWith(`.${type}`))
}

function isImageMime(type: string) {
  return type.startsWith('image/')
}

function isModelMime(type: string) {
  return type.startsWith('model/') || type === 'application/octet-stream' || type === 'text/plain'
}

function PreviewCard({
  file,
  onDelete,
  disabling,
}: {
  file: FileInfo
  onDelete: (path: string) => void
  disabling: boolean
}) {
  const [previewUrl, setPreviewUrl] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [previewType, setPreviewType] = useState<'image' | 'model' | 'none'>('none')

  useEffect(() => {
    let active = true
    const controller = new AbortController()
    const apiHost = getApiHost()
    const url = `${apiHost}/api/preview?path=${encodeURIComponent(file.path)}&format=png`

    setLoading(true)
    setError(false)
    setPreviewType('none')
    setPreviewUrl('')

    fetch(url, { signal: controller.signal })
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(`Preview request failed: ${res.status}`)
        }
        const contentType = res.headers.get('content-type') || ''
        if (isImageMime(contentType)) {
          const blob = await res.blob()
          if (!active) return
          setPreviewType('image')
          setPreviewUrl(URL.createObjectURL(blob))
        } else if (isModelMime(contentType)) {
          if (!active) return
          setPreviewType('model')
        } else {
          if (!active) return
          setPreviewType('none')
        }
      })
      .catch((err) => {
        if (err.name !== 'AbortError') {
          console.error('Preview error:', err)
          setError(true)
        }
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => {
      active = false
      controller.abort()
      if (previewUrl.startsWith('blob:')) {
        URL.revokeObjectURL(previewUrl)
      }
    }
  }, [file.path])

  const handleOpenFile = async () => {
    const apiHost = getApiHost()
    await fetch(`${apiHost}/api/open?path=${encodeURIComponent(file.path)}&mode=launch`)
  }

  const handleOpenFolder = async () => {
    const apiHost = getApiHost()
    await fetch(`${apiHost}/api/open?path=${encodeURIComponent(file.path)}&mode=reveal`)
  }

  const handleDelete = () => {
    if (!window.confirm(`Delete this duplicate file?\n\n${file.path}`)) {
      return
    }
    onDelete(file.path)
  }

  return (
    <div className="rounded-[2rem] overflow-hidden border border-white/10 bg-[#0d1116] shadow-[0_24px_80px_rgba(0,0,0,0.25)]">
      <div className="relative w-full h-[420px] bg-slate-950/80">
        {loading && (
          <div className="absolute inset-0 flex items-center justify-center text-slate-400">
            <Loader2 className="w-10 h-10 animate-spin" />
          </div>
        )}
        {error && !loading && (
          <div className="absolute inset-0 flex flex-col items-center justify-center text-slate-500 gap-3 px-6 text-center">
            <AlertTriangle className="w-12 h-12" />
            <p className="text-sm font-semibold">Preview unavailable</p>
            <p className="text-[11px] text-slate-400">This file can still be opened or deleted.</p>
          </div>
        )}
        {!loading && previewType === 'image' && previewUrl && (
          <img src={previewUrl} alt={file.name} className="w-full h-full object-cover" />
        )}
        {!loading && previewType === 'model' && !previewUrl && (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 text-slate-200 bg-slate-950/70 p-6 text-center">
            <ImageIcon className="w-14 h-14 text-fuchsia-400" />
            <p className="text-lg font-semibold">Model preview</p>
            <p className="text-sm text-slate-400">Click the file name to open the default viewer.</p>
          </div>
        )}
        {!loading && previewType === 'none' && !previewUrl && (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 text-slate-400 bg-slate-950/80 p-6 text-center">
            <AlertTriangle className="w-12 h-12" />
            <p className="text-sm font-semibold">No preview available</p>
            <p className="text-xs text-slate-500">Open the file to inspect it in the associated app.</p>
          </div>
        )}
      </div>

      <div className="p-6 space-y-4">
        <div className="flex flex-col gap-2">
          <p className="text-[10px] uppercase tracking-[0.35em] text-slate-500">Duplicate file</p>
          <p className="text-lg font-bold text-white break-all cursor-pointer hover:text-blue-300" onClick={handleOpenFile}>
            {file.name}
          </p>
        </div>

        <div className="grid gap-2 text-sm text-slate-400">
          <div className="flex items-center justify-between gap-3">
            <span className="font-semibold">Size</span>
            <span>{formatBytes(file.size)}</span>
          </div>
          <div className="flex items-start gap-3">
            <span className="font-semibold">Path</span>
            <span className="break-all text-xs text-slate-400">{file.path}</span>
          </div>
        </div>

        <div className="flex flex-col sm:flex-row gap-3 pt-2">
          <button
            onClick={handleOpenFolder}
            className="flex-1 inline-flex items-center justify-center gap-2 rounded-2xl bg-blue-500/15 border border-blue-500/20 px-4 py-3 text-sm font-semibold text-blue-300 hover:bg-blue-500/20 transition"
          >
            <FolderOpen className="w-4 h-4" />
            Open Folder
          </button>
          <button
            onClick={handleDelete}
            disabled={disabling}
            className="flex-1 inline-flex items-center justify-center gap-2 rounded-2xl bg-red-500/15 border border-red-500/20 px-4 py-3 text-sm font-semibold text-red-300 hover:bg-red-500/20 transition disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <Trash2 className="w-4 h-4" />
            {disabling ? 'Deleting…' : 'Delete File'}
          </button>
        </div>
      </div>
    </div>
  )
}

export default function RandomDuplicateGroupPage() {
  const [groupIndex, setGroupIndex] = useState<number>(-1)
  const [fileType, setFileType] = useState<FileType>('all')
  const [groups, setGroups] = useState<(SizeGroup | SimilarityGroup)[]>([])
  const [groupTitle, setGroupTitle] = useState<string>('')
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [message, setMessage] = useState<string>('')
  const [refreshCount, setRefreshCount] = useState(0)
  const [deleting, setDeleting] = useState<string>('')

  const filteredGroups = useMemo(
    () => groups.filter((group) => groupMatchesType(group, fileType)),
    [groups, fileType]
  )

  const selectedGroup = useMemo(() => {
    if (filteredGroups.length === 0 || groupIndex < 0 || groupIndex >= filteredGroups.length) return null
    return filteredGroups[groupIndex]
  }, [filteredGroups, groupIndex])

  const apiHost = typeof window !== 'undefined' ? getApiHost() : ''

  useEffect(() => {
    const fetchReport = async () => {
      setStatus('loading')
      setMessage('Loading duplicate groups...')

      try {
        const res = await fetch(`${apiHost}/api/report?exclude_similar=true`)
        if (!res.ok) {
          throw new Error(`HTTP ${res.status}`)
        }
        const data: Report = await res.json()
        const availableGroups: (SizeGroup | SimilarityGroup)[] = []

        if (Array.isArray(data.size_groups) && data.size_groups.length > 0) {
          availableGroups.push(...data.size_groups)
        }
        if (Array.isArray(data.similar_groups) && data.similar_groups.length > 0) {
          availableGroups.push(...data.similar_groups)
        }

        if (availableGroups.length === 0) {
          setGroups([])
          setGroupIndex(-1)
          setGroupTitle('No duplicate groups found')
          setStatus('ready')
          setMessage('Run a scan first or add more files to your scan directory.')
          return
        }

        setGroups(availableGroups)
        setStatus('ready')
      } catch (err) {
        console.error(err)
        setStatus('error')
        setMessage('Unable to fetch duplicate groups. Make sure the dashboard backend is running.')
      }
    }

    fetchReport()
  }, [apiHost, refreshCount])

  useEffect(() => {
    if (filteredGroups.length === 0) {
      setGroupIndex(-1)
      setGroupTitle('No duplicate groups found')
      setMessage(fileType === 'all' ? 'No duplicate groups available.' : `No duplicate groups found for ${fileType.toUpperCase()}.`)
      return
    }

    const randomIndex = Math.floor(Math.random() * filteredGroups.length)
    setGroupIndex(randomIndex)
    setGroupTitle(`Random duplicate group — ${formatBytes(getGroupSize(filteredGroups[randomIndex]))}`)
    setMessage(`Showing ${filteredGroups[randomIndex].files.length} duplicates (${fileType.toUpperCase()}).`)
  }, [filteredGroups])

  const chooseAnother = () => {
    if (filteredGroups.length === 0) return
    const nextIndex = Math.floor(Math.random() * filteredGroups.length)
    setGroupIndex(nextIndex)
    setGroupTitle(`Random duplicate group — ${formatBytes(getGroupSize(filteredGroups[nextIndex]))}`)
    setMessage(`Showing ${filteredGroups[nextIndex].files.length} duplicates (${fileType.toUpperCase()}).`)
  }

  const fileTypeOptions: { value: FileType; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'zip', label: 'ZIP' },
    { value: 'rar', label: 'RAR' },
    { value: '7z', label: '7Z' },
  ]

  const handleDelete = async (path: string) => {
    setDeleting(path)
    try {
      const res = await fetch(`${apiHost}/api/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path }),
      })
      if (!res.ok) {
        throw new Error(await res.text())
      }
      setMessage('File deleted successfully. Refreshing group...')
      setRefreshCount((value) => value + 1)
    } catch (err) {
      console.error(err)
      setMessage('Delete failed. Check console for details.')
    } finally {
      setDeleting('')
    }
  }

  const changeType = (type: FileType) => setFileType(type)

  return (
    <div className="min-h-screen bg-background text-foreground p-8 lg:p-12">
      <div className="max-w-[1500px] mx-auto">
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-6 mb-10">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-3 rounded-3xl bg-fuchsia-500/10 px-4 py-2 text-sm font-semibold text-fuchsia-200 border border-fuchsia-500/20">
              <Sparkles className="w-4 h-4" />
              Random Duplicate Group
            </div>
            <div className="flex items-center gap-3">
              <h1 className="text-5xl font-black tracking-tight text-white">One random duplicate set</h1>
              <Sparkles className="w-10 h-10 text-fuchsia-400" />
            </div>
            <p className="max-w-3xl text-slate-400 text-lg leading-8">
              Filter by archive type, see large cards for each duplicate, and open the file with the default app.
            </p>
          </div>

          <div className="flex flex-wrap gap-3">
            <Link href="/">
              <button className="px-6 py-3 rounded-2xl border border-white/10 bg-white/5 text-sm font-semibold text-white hover:bg-white/10 transition">
                Dashboard
              </button>
            </Link>
            <Link href="/gallery">
              <button className="px-6 py-3 rounded-2xl border border-white/10 bg-white/5 text-sm font-semibold text-white hover:bg-white/10 transition">
                Gallery View
              </button>
            </Link>
            <button
              onClick={chooseAnother}
              className="px-6 py-3 rounded-2xl bg-blue-500 text-sm font-semibold text-white hover:bg-blue-400 transition flex items-center gap-2"
            >
              <RefreshCw className="w-4 h-4" />
              Another group
            </button>
          </div>
        </div>

        <div className="mb-8 rounded-[2rem] border border-white/10 bg-[#111827]/80 p-6 flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
          <div>
            <p className="text-sm uppercase tracking-[0.28em] text-slate-500">Filter archive type</p>
            <div className="mt-3 flex flex-wrap gap-3">
              {fileTypeOptions.map((option) => (
                <button
                  key={option.value}
                  onClick={() => changeType(option.value)}
                  className={`px-4 py-3 rounded-2xl text-sm font-semibold transition ${
                    fileType === option.value
                      ? 'bg-fuchsia-500 text-white border border-fuchsia-500/40'
                      : 'bg-white/5 text-slate-300 border border-white/10 hover:bg-white/10'
                  }`}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>
          <div className="rounded-3xl bg-white/5 px-5 py-4 border border-white/10 text-sm text-slate-300">
            <div className="font-semibold text-white">Status</div>
            <div className="mt-2 text-slate-400">{status === 'loading' ? 'Loading groups...' : selectedGroup ? `Showing ${selectedGroup.files.length} files` : 'No group available'}</div>
          </div>
        </div>

        {status === 'loading' && (
          <div className="rounded-[2rem] border border-white/10 bg-[#0f172a]/80 p-16 text-center text-slate-400">
            <Loader2 className="mx-auto mb-6 h-10 w-10 animate-spin text-slate-400" />
            <p>Loading a random duplicate group...</p>
          </div>
        )}

        {status === 'error' && (
          <div className="rounded-[2rem] border border-red-500/20 bg-red-500/10 p-10 text-center text-red-200">
            <AlertTriangle className="mx-auto mb-4 h-12 w-12" />
            <p className="text-lg font-semibold">{message}</p>
          </div>
        )}

        {status === 'ready' && selectedGroup && (
          <>
            <div className="mb-8 rounded-[2rem] border border-white/10 bg-[#111827]/80 p-8">
              <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                <div>
                  <p className="text-sm uppercase tracking-[0.28em] text-slate-500">Selected group</p>
                  <h2 className="mt-3 text-3xl font-black text-white">{groupTitle}</h2>
                  <p className="mt-2 text-slate-400 max-w-2xl">{message}</p>
                </div>
                <div className="rounded-3xl bg-white/5 px-5 py-4 border border-white/10 text-sm text-slate-300">
                  <div className="space-y-2">
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-slate-400">Files</span>
                      <span className="font-semibold text-white">{selectedGroup.files.length}</span>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-slate-400">Group size</span>
                      <span className="font-semibold text-white">{formatBytes(getGroupSize(selectedGroup))}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div className="grid gap-8 xl:grid-cols-[1.3fr_0.7fr]">
              <div className="space-y-8">
                <div className="grid gap-6 lg:grid-cols-2">
                  {selectedGroup.files.map((file) => (
                    <PreviewCard key={file.path} file={file} onDelete={handleDelete} disabling={deleting === file.path} />
                  ))}
                </div>
              </div>

              <aside className="rounded-[2rem] border border-white/10 bg-[#0f172a]/80 p-8 space-y-6">
                <div className="rounded-3xl bg-white/5 p-6 border border-white/10">
                  <div className="flex items-center gap-3 text-slate-300">
                    <Box className="w-5 h-5 text-blue-400" />
                    <div>
                      <p className="text-xs uppercase tracking-[0.28em] text-slate-500">Duplicate insights</p>
                      <p className="mt-2 text-white font-semibold">Large cards make duplicate review fast.</p>
                    </div>
                  </div>
                </div>
                <div className="space-y-4">
                  <div className="rounded-3xl bg-white/5 p-5 border border-white/10">
                    <p className="text-sm uppercase tracking-[0.28em] text-slate-500">How it works</p>
                    <ul className="mt-3 space-y-3 text-sm text-slate-400">
                      <li>• Filter by file type and get one random duplicate group.</li>
                      <li>• Click the file name to open the file with the default app.</li>
                      <li>• Use the folder button to open the file location.</li>
                    </ul>
                  </div>
                  <div className="rounded-3xl bg-white/5 p-5 border border-white/10">
                    <p className="text-sm uppercase tracking-[0.28em] text-slate-500">Tip</p>
                    <p className="text-sm text-slate-400">If preview is unavailable, open the file directly using the filename or default app.</p>
                  </div>
                </div>
              </aside>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
