"use client"

import { useState, useEffect, useMemo, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import Link from 'next/link'
import {
  Box,
  Search,
  Trash2,
  FileText,
  AlertTriangle,
  CheckCircle2,
  Layers,
  Cpu,
  ShieldCheck,
  Zap,
  Clock,
  ExternalLink,
  Filter,
  Image as ImageIcon,
  Loader2,
  Folder,
  Grid3x3,
  Moon,
  Sun,
  ArrowUp01,
  ArrowDown01,
  Settings,
  HardDrive
} from 'lucide-react'
import ModelPreview from '@/components/ModelPreview'

interface FileInfo {
  name: string
  path: string
  size: number
  mod_time: string
  p_hash?: number
  visual_score?: number
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
  progress?: number
}

interface AppConfig {
  directory: string
  trash_path: string
  threshold: number
  recursive: boolean
  leave_ref: boolean
  delete_mode: string
  scan_full_system: boolean
}

function SetupView({ onStart, isLoading }: { onStart: (config: AppConfig) => void, isLoading: boolean }) {
  const [config, setConfig] = useState<AppConfig>({
    directory: '',
    trash_path: '',
    threshold: 70,
    recursive: true,
    leave_ref: false,
    delete_mode: 'oldest',
    scan_full_system: false
  })

  useEffect(() => {
    // Load existing config if any
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    fetch(`${apiHost}/api/config`)
      .then(res => res.json())
      .then(data => {
        if (data && data.directory) setConfig(data)
      })
      .catch(err => console.error("Failed to load config:", err))
  }, [])

  return (
    <div className="min-h-screen bg-background text-foreground flex items-center justify-center p-6">
      <motion.div
        initial={{ opacity: 0, y: 30 }}
        animate={{ opacity: 1, y: 0 }}
        className="w-full max-w-2xl glass-card p-10 rounded-[2.5rem] border border-blue-500/20 shadow-2xl shadow-blue-500/10"
      >
        <div className="flex items-center gap-4 mb-10">
          <div className="w-16 h-16 rounded-2xl bg-blue-600/20 flex items-center justify-center border border-blue-500/30">
            <Box className="w-8 h-8 text-blue-400" />
          </div>
          <div>
            <h1 className="text-3xl font-black tracking-tight">ANALYSIS SETUP</h1>
            <p className="text-gray-600 dark:text-gray-500 font-medium">Configure your workspace intelligence</p>
          </div>
        </div>

        <div className="space-y-8">
          <div className="space-y-3">
            <div className="flex items-center justify-between ml-2">
              <label className="text-[10px] font-black text-gray-400 uppercase tracking-[0.2em]">Scan Mode</label>
              <label className="flex items-center gap-2 cursor-pointer group">
                <div className={`w-5 h-5 rounded-md border-2 flex items-center justify-center transition-all ${config.scan_full_system ? 'bg-purple-600 border-purple-600 shadow-lg shadow-purple-500/20' : 'border-glass-border group-hover:border-purple-500/40'}`} onClick={() => {
                  const newFullSystem = !config.scan_full_system
                  setConfig({ 
                    ...config, 
                    scan_full_system: newFullSystem,
                    directory: newFullSystem ? '' : config.directory
                  })
                }}>
                  {config.scan_full_system && <CheckCircle2 className="w-3 h-3 text-white" />}
                </div>
                <span className="text-[10px] font-bold uppercase tracking-widest text-gray-500 dark:text-gray-400">Full PC Scan</span>
              </label>
            </div>
            <div className="relative group">
              <Folder className="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-500 group-focus-within:text-blue-500 transition-colors" />
              <input
                type="text"
                placeholder="C:\Users\...\MyAssets"
                value={config.directory}
                onChange={(e) => setConfig({ ...config, directory: e.target.value })}
                disabled={config.scan_full_system}
                className="w-full bg-glass-layer border border-glass-border rounded-2xl py-5 pl-14 pr-6 text-sm font-medium focus:outline-none focus:border-blue-500/50 focus:bg-glass-layer transition-all disabled:opacity-50 disabled:cursor-not-allowed"
              />
            </div>
            {config.scan_full_system && (
              <p className="text-xs text-purple-400/70 ml-2">🔍 Will scan all available drives (C:, D:, etc. on Windows; /Users, /Volumes on macOS; /home, /mnt on Linux)</p>
            )}
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-3">
              <label className="text-[10px] font-black text-gray-400 uppercase tracking-[0.2em] ml-2">Trash Folder (Optional)</label>
              <div className="relative group">
                <Trash2 className="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-500 group-focus-within:text-red-500 transition-colors" />
                <input
                  type="text"
                  placeholder="C:\...\Trash"
                  value={config.trash_path}
                  onChange={(e) => setConfig({ ...config, trash_path: e.target.value })}
                  className="w-full bg-glass-layer border border-glass-border rounded-2xl py-5 pl-14 pr-6 text-sm font-medium focus:outline-none focus:border-red-500/50 focus:bg-glass-layer transition-all"
                />
              </div>
            </div>

            <div className="space-y-3">
              <label className="text-[10px] font-black text-gray-400 uppercase tracking-[0.2em] ml-2">Similarity threshold (%)</label>
              <div className="relative group">
                <Filter className="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-500 group-focus-within:text-cyan-500 transition-colors" />
                <input
                  type="number"
                  min="0"
                  max="100"
                  value={config.threshold}
                  onChange={(e) => setConfig({ ...config, threshold: parseInt(e.target.value) || 0 })}
                  className="w-full bg-glass-layer border border-glass-border rounded-2xl py-5 pl-14 pr-6 text-sm font-medium focus:outline-none focus:border-cyan-500/50 focus:bg-glass-layer transition-all"
                />
              </div>
            </div>
          </div>

          <div className="flex flex-wrap gap-8 items-center bg-glass-layer p-6 rounded-3xl border border-glass-border">
            <label className="flex items-center gap-3 cursor-pointer group">
              <div className={`w-6 h-6 rounded-md border-2 flex items-center justify-center transition-all ${config.recursive ? 'bg-blue-600 border-blue-600 shadow-lg shadow-blue-500/20' : 'border-glass-border group-hover:border-blue-500/40'}`} onClick={() => setConfig({ ...config, recursive: !config.recursive })}>
                {config.recursive && <CheckCircle2 className="w-4 h-4 text-white" />}
              </div>
              <span className="text-xs font-bold uppercase tracking-widest text-gray-500 dark:text-gray-400">Recursive Scan</span>
            </label>

            <label className="flex items-center gap-3 cursor-pointer group">
              <div className={`w-6 h-6 rounded-md border-2 flex items-center justify-center transition-all ${config.leave_ref ? 'bg-purple-600 border-purple-600 shadow-lg shadow-purple-500/20' : 'border-glass-border group-hover:border-purple-500/40'}`} onClick={() => setConfig({ ...config, leave_ref: !config.leave_ref })}>
                {config.leave_ref && <CheckCircle2 className="w-4 h-4 text-white" />}
              </div>
              <span className="text-xs font-bold uppercase tracking-widest text-gray-500 dark:text-gray-400">Leave Reference TXT</span>
            </label>
          </div>

          <button
            onClick={() => onStart(config)}
            disabled={(!config.directory && !config.scan_full_system) || isLoading}
            className="w-full py-6 bg-gradient-to-r from-blue-600 to-cyan-600 hover:from-blue-500 hover:to-cyan-500 rounded-3xl text-sm font-black uppercase tracking-[0.3em] text-white shadow-2xl shadow-blue-500/20 transition-all active:scale-95 flex items-center justify-center gap-4 disabled:opacity-50 disabled:grayscale transition-all"
          >
            {isLoading ? (
              <Loader2 className="w-6 h-6 animate-spin" />
            ) : (
              <>
                <Zap className="w-6 h-6" />
                Initialize Scanner Intelligence
              </>
            )}
          </button>
        </div>
      </motion.div>
    </div>
  )
}

function PreviewImage({ path }: { path: string }) {
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const [error, setError] = useState(false)
  const [isStlFallback, setIsStlFallback] = useState(false)

  const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
  const isVideo = /\.(mp4|webm|mov|mkv|avi)$/i.test(path)
  const is3D = /\.(stl|obj|3mf)$/i.test(path)

  useEffect(() => {
    setPreviewUrl(null)
    setError(false)
    setIsStlFallback(false)

    if (isVideo) {
      setPreviewUrl(`${apiHost}/api/preview?path=${encodeURIComponent(path)}`)
      return
    }

    // For direct 3D files, request PNG render immediately
    if (is3D) {
      setPreviewUrl(`${apiHost}/api/preview?path=${encodeURIComponent(path)}&format=png`)
      return
    }

    // For everything else: attempt normal preview first, then fall back to STL render
    const primaryUrl = `${apiHost}/api/preview?path=${encodeURIComponent(path)}`
    fetch(primaryUrl)
      .then(res => {
        if (res.ok) {
          return res.blob().then(blob => {
            setPreviewUrl(URL.createObjectURL(blob))
          })
        }
        // Primary failed — try STL render fallback
        const stlUrl = `${apiHost}/api/preview?path=${encodeURIComponent(path)}&type=model&format=png`
        return fetch(stlUrl).then(res2 => {
          if (res2.ok) {
            return res2.blob().then(blob => {
              setIsStlFallback(true)
              setPreviewUrl(URL.createObjectURL(blob))
            })
          }
          throw new Error('No preview available')
        })
      })
      .catch(() => setError(true))
  }, [path])

  // Cleanup object URLs on unmount
  useEffect(() => {
    return () => {
      if (previewUrl?.startsWith('blob:')) URL.revokeObjectURL(previewUrl)
    }
  }, [previewUrl])

  return (
    <div className="relative w-full aspect-video rounded-lg overflow-hidden border border-glass-border bg-black/40 flex items-center justify-center">
      {error ? (
        <div className="flex flex-col items-center justify-center opacity-40">
          <ImageIcon className="w-8 h-8 mb-1" />
          <span className="text-[10px] font-bold uppercase tracking-widest">Preview Error</span>
        </div>
      ) : !previewUrl ? (
        <div className="flex flex-col items-center justify-center opacity-40">
          <Loader2 className="w-6 h-6 animate-spin mb-1" />
        </div>
      ) : isVideo ? (
        <video
          src={previewUrl}
          className="w-full h-full object-contain"
          onLoadedMetadata={(e) => {
            const video = e.target as HTMLVideoElement;
            video.currentTime = Math.min(video.duration / 2, 60);
          }}
          onError={() => setError(true)}
          muted
          playsInline
        />
      ) : (
        <img
          src={previewUrl}
          alt="Preview"
          className="w-full h-full object-contain"
          onError={() => setError(true)}
        />
      )}

      <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent pointer-events-none" />
      <div className="absolute bottom-2 left-3 flex items-center gap-2">
        <Zap className="w-3 h-3 text-blue-400 animate-pulse" />
        <span className="text-[8px] font-bold text-white/80 uppercase tracking-widest">
          {is3D || isStlFallback ? 'A.I. 3D RENDER' : isVideo ? 'AI VIDEO STREAM' : 'AI PREVIEW STREAM'}
        </span>
      </div>
    </div>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function FileItem({ file, onRefresh }: { file: FileInfo, onRefresh?: () => void }) {
  const [isHovered, setIsHovered] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)

  // Reset states when the file changes, preventing "stuck" buttons when React reuses the component
  useEffect(() => {
    setIsDeleting(false)
    setShowConfirm(false)
  }, [file.path])

  const handleOpen = (e: React.MouseEvent, mode: 'reveal' | 'launch') => {
    e.stopPropagation()
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    fetch(`${apiHost}/api/open?path=${encodeURIComponent(file.path)}&mode=${mode}`)
      .catch(err => console.error(`Failed to ${mode} file:`, err))
  }

  const handleDelete = async (e?: React.MouseEvent) => {
    if (e) e.stopPropagation()
    setShowConfirm(false)
    setIsDeleting(true)
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    try {
      const response = await fetch(`${apiHost}/api/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: file.path })
      })
      if (!response.ok) {
        throw new Error(await response.text())
      }
      if (onRefresh) onRefresh()
    } catch (err) {
      console.error("Failed to delete file:", err)
      alert("Error: " + err)
      setIsDeleting(false)
    }
  }

  return (
    <div
      className="relative flex items-center gap-3 p-3 bg-glass-layer rounded-xl group/file cursor-pointer hover:bg-glass-border transition-all"
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      onClick={(e) => handleOpen(e, 'launch')}
    >
      <div className="w-10 h-10 rounded-lg bg-black/40 flex items-center justify-center text-blue-500/50 group-hover/file:text-blue-400 transition-colors">
        <Box className="w-5 h-5" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <p className="text-sm font-bold text-foreground truncate">{file.name}</p>
          <span className="text-[10px] font-black px-1.5 py-0.5 rounded bg-glass-layer text-gray-500 dark:text-gray-400 uppercase tracking-tighter">
            {formatBytes(file.size)}
          </span>
          {file.visual_score !== undefined && (
            <span className={`text-[10px] font-black px-1.5 py-0.5 rounded uppercase tracking-tighter border ${file.visual_score >= 95 ? 'bg-green-500/10 text-green-400 border-green-500/20' : 'bg-orange-500/10 text-orange-400 border-orange-500/20'}`}>
              {file.visual_score.toFixed(1)}% Match
            </span>
          )}
        </div>
        <p className="text-[10px] font-medium truncate uppercase tracking-tighter font-mono">
          {(() => {
            const parts = file.path.split(/[\\/]/)
            const fileName = parts[parts.length - 1]
            const dirPath = parts.slice(0, -1).join('\\')
            return (
              <>
                {dirPath && <span className="text-cyan-500/70">{dirPath}\</span>}
                <span className="text-gray-400">{fileName}</span>
              </>
            )
          })()}
        </p>
      </div>
      <div className="flex gap-2">
        <button
          onClick={(e) => handleOpen(e, 'reveal')}
          className="p-2 bg-blue-500/10 hover:bg-blue-500/20 rounded-lg text-blue-400 transition-all"
          title="Reveal in Folder"
        >
          <Folder className="w-4 h-4" />
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); setShowConfirm(true); }}
          disabled={isDeleting}
          className={`p-2 bg-red-500/10 hover:bg-red-500/20 rounded-lg text-red-400 transition-all ${isDeleting ? 'opacity-50 cursor-wait' : ''}`}
          title="Delete/Trash File"
        >
          {isDeleting ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <Trash2 className="w-4 h-4" />
          )}
        </button>
      </div>

      <AnimatePresence>
        {showConfirm && (
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
            className="absolute inset-0 bg-gray-900/90 rounded-xl z-50 flex items-center justify-between px-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex flex-col">
              <span className="text-xs font-bold text-white">Move to trash?</span>
              <span className="text-[10px] text-gray-400 truncate max-w-[200px]">{file.name}</span>
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => setShowConfirm(false)}
                className="px-3 py-1 bg-white/10 hover:bg-white/20 rounded-lg text-[10px] font-bold text-white transition-all uppercase tracking-wider"
              >
                No
              </button>
              <button
                onClick={() => handleDelete()}
                className="px-3 py-1 bg-red-500 hover:bg-red-600 rounded-lg text-[10px] font-bold text-white transition-all uppercase tracking-wider shadow-lg shadow-red-500/20"
              >
                Yes
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      <AnimatePresence>
        {isHovered && (
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 10 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 10 }}
            className="absolute left-0 bottom-full mb-3 w-96 z-[100] pointer-events-none"
          >
            <div className="glass-card p-2 rounded-2xl shadow-2xl border border-blue-500/30">
              <PreviewImage path={file.path} />
            </div>
            <div className="w-3 h-3 bg-[#111114] rotate-45 border-r border-b border-blue-500/30 absolute -bottom-1.5 left-8 z-[-1]" />
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

export default function Dashboard() {
  const [mounted, setMounted] = useState(false)
  const [data, setData] = useState<Report | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [fileType, setFileType] = useState('all')
  const [status, setStatus] = useState<string | null>(null)
  const [notified, setNotified] = useState(false)
  const [viewMode, setViewMode] = useState<'size' | 'similar' | 'visual'>('size')
  const [currentPage, setCurrentPage] = useState(1)
  const [itemsPerPage, setItemsPerPage] = useState(50)
  const [selectedFiles, setSelectedFiles] = useState<string[]>([])
  const [isEditingPage, setIsEditingPage] = useState(false)
  const [tempPage, setTempPage] = useState('')
  const [theme, setTheme] = useState<'light' | 'dark' | 'system'>('system')
  const [sizeSortOrder, setSizeSortOrder] = useState<'asc' | 'desc'>('desc')

  // Theme Logic
  useEffect(() => {
    const saved = localStorage.getItem('theme') as 'light' | 'dark' | 'system' | null
    if (saved) setTheme(saved)
  }, [])

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'system') {
      root.classList.remove('light', 'dark')
      localStorage.removeItem('theme')
    } else {
      root.classList.remove('light', 'dark')
      root.classList.add(theme)
      localStorage.setItem('theme', theme)
    }
  }, [theme])

  const toggleTheme = () => {
    if (theme === 'system') {
      const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches
      setTheme(isDark ? 'light' : 'dark')
    } else {
      setTheme(theme === 'dark' ? 'light' : 'dark')
    }
  }

  // Global error listener for debugging
  useEffect(() => {
    setMounted(true)
    const handleError = (e: ErrorEvent) => {
      console.error("Global captured error:", e.error)
      if (typeof window !== 'undefined') {
        localStorage.setItem('last_error', JSON.stringify({
          message: e.message,
          stack: e.error?.stack,
          timestamp: new Date().toISOString()
        }))
      }
    }
    window.addEventListener('error', handleError)
    return () => window.removeEventListener('error', handleError)
  }, [])


  const requestNotificationPermission = () => {
    if ('Notification' in window) {
      Notification.requestPermission()
    }
  }

  const fetchData = useCallback(async () => {
    try {
      const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
      const response = await fetch(`${apiHost}/api/report`)
      if (!response.ok) throw new Error(`HTTP ${response.status}: ${response.statusText}`)

      const report: Report = await response.json()
      console.log("📊 Data received:", {
        files: report.total_files,
        sizeGroups: report.size_groups?.length || 0,
        similarGroups: report.similar_groups?.length || 0
      })

      setData(report)

      if (report.status === 'finished' && status === 'analyzing' && !notified) {
        if (typeof window !== 'undefined' && 'Notification' in window && Notification.permission === 'granted') {
          new Notification('🔍 Analysis Complete!', {
            body: `Found ${report.similar_groups?.length || 0} similar file clusters.`,
            icon: '/favicon.ico'
          })
        }
        setNotified(true)
      }

      setStatus(report.status || 'finished')
      setLoading(false)
    } catch (err) {
      console.error("❌ Fetch error:", err)
      setError(err instanceof Error ? err.message : String(err))
      setLoading(false)
    }
  }, [status, notified])

  useEffect(() => {
    if (!mounted) return
    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [mounted, fetchData])

  const filteredSizeGroups = useMemo(() => {
    if (!data?.size_groups) return []
    const query = searchQuery.toLowerCase()
    const filtered = data.size_groups.filter(group => {
      return (group?.files || []).some(file => {
        const name = (file?.name || '').toLowerCase()
        const matchesSearch = name.includes(query)
        const matchesType = fileType === 'all' || name.endsWith(`.${fileType.toLowerCase()}`)
        return matchesSearch && matchesType
      })
    }) || []

    return filtered.sort((a, b) => {
      return sizeSortOrder === 'desc' ? b.size - a.size : a.size - b.size
    })
  }, [data?.size_groups, searchQuery, fileType, sizeSortOrder])

  const filteredSimilarGroups = useMemo(() => {
    if (!data?.similar_groups) return []
    const query = searchQuery.toLowerCase()

    // Performance optimization: limit rendering if list is huge
    const list = searchQuery === '' && fileType === 'all'
      ? data.similar_groups.slice(0, 5000)
      : data.similar_groups

    return list.filter(group => {
      // Check if ANY file in the group matches
      return group.files.some(f => {
        const name = (f?.name || '').toLowerCase()
        const matchesSearch = name.includes(query)
        const matchesType = fileType === 'all' || name.endsWith(`.${fileType.toLowerCase()}`)
        return matchesSearch && matchesType
      })
    }) || []
  }, [data?.similar_groups, searchQuery, fileType])

  const currentItems = useMemo(() => {
    if (viewMode === 'size') return filteredSizeGroups || []
    if (viewMode === 'similar') return filteredSimilarGroups || []

    // Visual Matching Filtering (similar to similarity groups)
    if (!data?.visual_groups) return []
    const query = searchQuery.toLowerCase()
    return data.visual_groups.filter(group => {
      return group.files.some(f => {
        const name = (f?.name || '').toLowerCase()
        const matchesSearch = name.includes(query)
        const matchesType = fileType === 'all' || name.endsWith(`.${fileType.toLowerCase()}`)
        return matchesSearch && matchesType
      })
    }) || []
  }, [viewMode, filteredSizeGroups, filteredSimilarGroups, data?.visual_groups, searchQuery, fileType])

  const paginatedItems = useMemo(() =>
    currentItems.slice((currentPage - 1) * itemsPerPage, currentPage * itemsPerPage)
    , [currentItems, currentPage, itemsPerPage])

  const totalPages = useMemo(() =>
    Math.ceil(currentItems.length / itemsPerPage)
    , [currentItems.length, itemsPerPage])

  const handlePageChange = (page: number) => {
    setCurrentPage(page)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  // Reset to page 1 when filters or view mode change
  useEffect(() => {
    setCurrentPage(1)
  }, [searchQuery, fileType, viewMode, itemsPerPage])

  const handleRunStep3 = async () => {
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    try {
      await fetch(`${apiHost}/api/run-step-3`, { method: 'POST' })
      setStatus('analyzing_step3')
    } catch (err) {
      console.error("Failed to run Step 3:", err)
      alert("Error triggering Step 3")
    }
  }

  const handleRunVisual = async () => {
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    try {
      await fetch(`${apiHost}/api/run-visual`, { method: 'POST' })
      setStatus('analyzing_visual')
    } catch (err) {
      console.error("Failed to run Visual analysis:", err)
      alert("Error triggering Visual analysis")
    }
  }

  const handleOpenDirectory = async () => {
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    try {
      await fetch(`${apiHost}/api/open-directory`, { method: 'POST' })
    } catch (err) {
      console.error("Failed to open directory:", err)
      alert("Error opening directory")
    }
  }

  const handleMarkAsGood = async (e: React.MouseEvent, files: FileInfo[]) => {
    e.stopPropagation()
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    try {
      const response = await fetch(`${apiHost}/api/mark-as-good`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files })
      })
      if (!response.ok) throw new Error(await response.text())
      fetchData() // Refresh data to hide the group
    } catch (err) {
      console.error("Failed to mark group as good:", err)
      alert("Error: " + err)
    }
  }

  const [savingConfig, setSavingConfig] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [cacheStats, setCacheStats] = useState<{ size_gb: number, limit_gb: number } | null>(null)

  const fetchCacheStats = useCallback(async () => {
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    try {
      const res = await fetch(`${apiHost}/api/cache-stats`)
      const data = await res.json()
      setCacheStats(data)
    } catch (err) {
      console.error("Failed to fetch cache stats:", err)
    }
  }, [])

  useEffect(() => {
    if (showSettings) {
      fetchCacheStats()
    }
  }, [showSettings, fetchCacheStats])

  const handleClearCache = async () => {
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    if (!confirm("Are you sure you want to clear all cached previews? (They will be re-extracted on next view)")) return
    try {
      await fetch(`${apiHost}/api/cache-clear`, { method: 'POST' })
      fetchCacheStats()
    } catch (err) {
      alert("Error clearing cache")
    }
  }

  const handleUpdateCacheLimit = async (limit: number) => {
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    // Load current config, update limit, then save
    try {
      const res = await fetch(`${apiHost}/api/config`)
      const cfg = await res.json()
      cfg.cache_limit_gb = limit
      await fetch(`${apiHost}/api/config`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(cfg)
      })
      fetchCacheStats()
    } catch (err) {
      console.error("Failed to update cache limit:", err)
    }
  }
  const handleStartScan = async (config: AppConfig) => {
    setSavingConfig(true)
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    try {
      // 1. Save Config
      await fetch(`${apiHost}/api/config`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      })
      // 2. Start Scan
      await fetch(`${apiHost}/api/start-scan`, { method: 'POST' })
      fetchData()
    } catch (err) {
      console.error("Failed to start scan:", err)
      alert("Error starting scan: " + err)
    } finally {
      setSavingConfig(false)
    }
  }

  if (!mounted || loading) return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-background text-foreground">
      <motion.div
        animate={{ rotate: 360 }}
        transition={{ repeat: Infinity, duration: 2, ease: "linear" }}
      >
        <Zap className="w-12 h-12 text-blue-500" />
      </motion.div>
      <p className="mt-4 text-gray-400 animate-pulse font-light tracking-widest uppercase">Initializing Scanner Intelligence...</p>
    </div>
  )

  if (error) return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-background text-foreground p-6">
      <AlertTriangle className="w-16 h-16 text-red-500 mb-4" />
      <h1 className="text-2xl font-bold mb-2">Connection Error</h1>
      <p className="text-gray-400 text-center max-w-md">{error}</p>
      <button
        onClick={() => window.location.reload()}
        className="mt-6 px-6 py-2 bg-blue-600 hover:bg-blue-500 rounded-full transition-all"
      >
        Retry Connection
      </button>
    </div>
  )

  if (data?.status === 'idle') return (
    <SetupView onStart={handleStartScan} isLoading={savingConfig} />
  )

  const stats = [
    { label: 'Total Files', value: data?.total_files || 0, icon: Box, color: 'text-blue-400' },
    { label: 'Size Groups', value: data?.size_groups?.length || 0, icon: Layers, color: 'text-purple-400' },
    { label: 'Similar Names', value: data?.similar_groups?.length || 0, icon: FileText, color: 'text-cyan-400' },
    { label: 'Visual Matches', value: data?.visual_groups?.length || 0, icon: ImageIcon, color: 'text-orange-400' },
    { label: 'Scan Time', value: `${data?.analysis_duration_seconds?.toFixed(2) || 0}s`, icon: Clock, color: 'text-green-400' },
  ]

  const fileTypes = ['all', 'zip', 'rar', '7z', 'stl']

  return (
    <div className="min-h-screen bg-background text-foreground p-8 md:p-12 flex flex-col items-center">
      <div className="w-full max-w-[1700px] transition-all duration-500 ease-in-out">
        {/* Header */}
        <header className="flex flex-col md:flex-row justify-between items-start md:items-center mb-16 gap-6">
          <div>
            <h1 className="text-5xl font-black tracking-tight flex items-center gap-4">
              <span className="bg-gradient-to-r from-blue-500 to-cyan-400 bg-clip-text text-transparent">ARCHIVE</span>
              <span className="text-foreground">FINDER</span>
              <div className="px-3 py-1 bg-blue-500/10 border border-blue-500/20 rounded-md text-xs text-blue-400 uppercase tracking-widest font-bold">Intelligence v2.0.0</div>
            </h1>
            <p className="text-gray-500 mt-2 font-medium tracking-wide text-lg">3D Asset Deduplication & Management Dashboard</p>
          </div>
          <div className="flex gap-4">
            <div className="flex items-center gap-4">
              <button
                onClick={async () => {
                  const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
                  await fetch(`${apiHost}/api/reset`, { method: 'POST' })
                }}
                className="px-6 py-3 bg-red-500/10 hover:bg-red-500/20 rounded-2xl text-sm font-medium text-red-400 transition-all border border-red-500/20"
              >
                New Scan
              </button>
              <button
                onClick={toggleTheme}
                className="p-3 bg-glass-layer hover:bg-glass-border rounded-2xl text-gray-500 dark:text-gray-400 transition-all border border-glass-border"
                title={`Theme: ${theme}`}
              >
                {theme === 'dark' ? <Moon className="w-5 h-5" /> : theme === 'light' ? <Sun className="w-5 h-5" /> : <div className="relative"><Sun className="w-5 h-5" /><Moon className="w-3 h-3 absolute -bottom-1 -right-1" /></div>}
              </button>
              <button
                onClick={() => setShowSettings(true)}
                className="p-3 bg-glass-layer hover:bg-glass-border rounded-2xl text-gray-500 dark:text-gray-400 transition-all border border-glass-border"
                title="Settings"
              >
                <Settings className="w-5 h-5" />
              </button>
              <Link href="/gallery">
                <button className="px-6 py-3 bg-gradient-to-r from-blue-600 to-cyan-600 hover:from-blue-500 hover:to-cyan-500 rounded-2xl text-sm font-bold text-white transition-all border border-blue-500/20 shadow-lg shadow-blue-500/20 flex items-center gap-2">
                  <Grid3x3 className="w-5 h-5" />
                  Gallery View
                </button>
              </Link>
              <button
                onClick={requestNotificationPermission}
                className="px-6 py-3 bg-glass-layer hover:bg-glass-border rounded-2xl text-sm font-medium text-gray-500 dark:text-gray-400 transition-all border border-glass-border"
              >
                🔔 Enable Notifications
              </button>
              <div className="flex items-center gap-3 px-6 py-3 bg-glass-layer rounded-2xl border border-glass-border">
                <div className={`w-2.5 h-2.5 rounded-full ${data?.status === 'finished' ? 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)]' : 'bg-yellow-500 animate-pulse'}`} />
                <span className="text-sm font-medium text-gray-300 uppercase tracking-widest">
                  {data?.status || 'Analyzing'}
                </span>
              </div>
            </div>
          </div>
        </header>

        {/* Filter Bar */}
        <div className="flex flex-wrap items-center gap-6 mb-12 w-full">
          <div className="relative flex-grow min-w-[320px] group">
            <Search className="absolute left-6 top-1/2 -translate-y-1/2 w-6 h-6 text-gray-500 group-focus-within:text-blue-500 transition-colors" />
            <input
              type="text"
              placeholder="Search by filename..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full bg-glass-layer border border-glass-border rounded-3xl py-5 pl-16 pr-6 text-base font-medium focus:outline-none focus:border-blue-500/50 focus:bg-glass-layer transition-all"
            />
          </div>

          {viewMode === 'size' && (
            <button
              onClick={() => setSizeSortOrder(current => current === 'desc' ? 'asc' : 'desc')}
              className="p-5 bg-glass-layer hover:bg-glass-border border border-glass-border rounded-3xl transition-all text-gray-400 hover:text-foreground flex items-center gap-2"
              title={`Sort by Size: ${sizeSortOrder === 'desc' ? 'Descending' : 'Ascending'}`}
            >
              {sizeSortOrder === 'desc' ? <ArrowDown01 className="w-6 h-6" /> : <ArrowUp01 className="w-6 h-6" />}
            </button>
          )}

          <div className="flex flex-wrap gap-3 items-center w-full justify-between">
            <div className="flex gap-2 bg-glass-layer p-1.5 rounded-3xl flex-grow sm:flex-grow-0">
              <button
                onClick={() => setViewMode('size')}
                className={`flex-1 sm:flex-none px-6 py-4 rounded-2xl text-sm font-bold uppercase tracking-wide transition-all flex items-center justify-center gap-3 whitespace-nowrap ${viewMode === 'size'
                  ? 'bg-blue-600 text-white shadow-lg shadow-blue-500/20'
                  : 'text-gray-500 hover:text-gray-300'
                  }`}
              >
                <Layers className="w-5 h-5" />
                Size Matches
              </button>
              <button
                onClick={() => setViewMode('similar')}
                className={`flex-1 sm:flex-none px-6 py-4 rounded-2xl text-sm font-bold uppercase tracking-wide transition-all flex items-center justify-center gap-3 whitespace-nowrap ${viewMode === 'similar'
                  ? 'bg-cyan-600 text-white shadow-lg shadow-cyan-500/20'
                  : 'text-gray-500 hover:text-gray-300'
                  }`}
              >
                <FileText className="w-5 h-5" />
                Similar Names
              </button>
              <button
                onClick={() => setViewMode('visual')}
                className={`flex-1 sm:flex-none px-6 py-4 rounded-2xl text-sm font-bold uppercase tracking-wide transition-all flex items-center justify-center gap-3 whitespace-nowrap ${viewMode === 'visual'
                  ? 'bg-orange-600 text-white shadow-lg shadow-orange-500/20'
                  : 'text-gray-500 hover:text-gray-300'
                  }`}
              >
                <ImageIcon className="w-5 h-5" />
                Visual Hits
              </button>
            </div>

            <div className="flex gap-2 bg-glass-layer p-1.5 rounded-3xl overflow-x-auto max-w-full">
              {fileTypes.map(type => (
                <button
                  key={type}
                  onClick={() => setFileType(type)}
                  className={`px-5 py-3 rounded-2xl text-sm font-bold uppercase tracking-wide transition-all whitespace-nowrap ${fileType === type
                    ? 'bg-blue-600 text-white shadow-lg shadow-blue-500/20'
                    : 'text-gray-500 hover:text-gray-300 hover:bg-white/10'
                    }`}
                >
                  {type}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4 mb-10 relative z-10 w-full">
          {stats.map((stat, i) => (
            <div key={stat.label} className="relative w-full">
              <motion.div
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.1 }}
                className={`bg-glass-layer backdrop-blur-xl p-5 rounded-3xl relative overflow-hidden group hover:scale-[1.03] transition-all cursor-pointer h-full min-h-[110px] flex flex-col justify-between ${(stat.label === 'Size Groups' && viewMode === 'size') || (stat.label === 'Similar Names' && viewMode === 'similar') || (stat.label === 'Visual Matches' && viewMode === 'visual')
                  ? 'border border-blue-500/50 shadow-lg shadow-blue-500/10 bg-blue-500/10'
                  : 'border border-transparent hover:border-glass-border hover:bg-glass-layer/80'
                  }`}
                onClick={() => {
                  if (stat.label === 'Size Groups') setViewMode('size')
                  if (stat.label === 'Similar Names') setViewMode('similar')
                  if (stat.label === 'Visual Matches') setViewMode('visual')
                }}
              >
                {/* Dynamic Accent Bar */}
                <div className={`absolute top-0 left-0 w-1 h-full bg-current opacity-30 group-hover:opacity-100 transition-all ${stat.color.replace('text-', 'bg-')}`} />

                <div className="flex flex-col gap-2">
                  <div className="flex justify-between items-center">
                    <div className={`p-1.5 rounded-lg bg-glass-layer ${stat.color}`}>
                      <stat.icon className="w-4 h-4" />
                    </div>
                    <div className="text-[9px] font-black text-gray-500 uppercase tracking-[0.15em]">{stat.label}</div>
                  </div>

                  <div className="mt-1">
                    <div className={`text-2xl lg:text-3xl font-black text-foreground glow-text tracking-tighter truncate leading-none`}>
                      {stat.value}
                    </div>
                  </div>
                </div>

                {/* Subtle Background Glow */}
                <div className={`absolute -right-4 -bottom-4 w-16 h-16 rounded-full blur-[40px] opacity-10 transition-opacity group-hover:opacity-30 ${stat.color.replace('text-', 'bg-')}`} />
              </motion.div>
            </div>
          ))}
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-12 gap-10 w-full">
          {/* Left Column: Listings */}
          <div className="lg:col-span-8 xl:col-span-9 space-y-6">

            {/* Section: Results */}
            <section className="w-full">
              <div className="flex flex-col sm:flex-row items-start sm:items-center gap-4 mb-6 pb-4 border-b border-white/5">
                <div className={`p-3 rounded-xl ${viewMode === 'size' ? 'bg-blue-500/20' : 'bg-cyan-500/20'}`}>
                  {viewMode === 'size' ? (
                    <Layers className="w-6 h-6 text-blue-400" />
                  ) : (
                    <FileText className="w-6 h-6 text-cyan-400" />
                  )}
                </div>
                <div>
                  <h2 className="text-2xl font-black text-white uppercase tracking-wide">
                    {viewMode === 'size' ? 'Identical Size Groups' :
                      viewMode === 'similar' ? 'Similarity Hits' : 'Visual Match Hits'}
                  </h2>
                  <p className="text-xs text-gray-500 font-medium mt-1">
                    Review and manage detected duplicate sets
                  </p>
                </div>
                <div className="flex-1" />
                {currentItems.length > 0 && (
                  <div
                    className="text-xs font-bold text-gray-400 uppercase tracking-wide bg-white/5 px-4 py-2 rounded-xl border border-white/5 whitespace-nowrap cursor-pointer hover:bg-white/10 transition-all flex items-center group"
                    title="Click to jump to page"
                    onClick={() => {
                      setIsEditingPage(true);
                      setTempPage(currentPage.toString());
                    }}
                  >
                    {isEditingPage ? (
                      <div className="flex items-center gap-1 px-1">
                        <span className="opacity-50">PAGE</span>
                        <input
                          autoFocus
                          type="text"
                          value={tempPage}
                          onChange={(e) => setTempPage(e.target.value.replace(/\D/g, ''))}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                              const p = parseInt(tempPage);
                              if (!isNaN(p) && p > 0 && p <= totalPages) {
                                handlePageChange(p);
                              }
                              setIsEditingPage(false);
                            } else if (e.key === 'Escape') {
                              setIsEditingPage(false);
                            }
                          }}
                          onBlur={() => {
                            const p = parseInt(tempPage);
                            if (!isNaN(p) && p > 0 && p <= totalPages) {
                              handlePageChange(p);
                            }
                            setIsEditingPage(false);
                          }}
                          className="w-10 bg-blue-500/20 border-none outline-none text-white text-center rounded py-0 px-1 font-black"
                          onClick={(e) => e.stopPropagation()}
                        />
                        <span className="opacity-50">OF {totalPages}</span>
                      </div>
                    ) : (
                      <div className="px-4 py-2">
                        Page <span className="text-white group-hover:text-blue-400 transition-colors">{currentPage}</span> of {totalPages} <span className="opacity-50 mx-2">|</span> {currentItems.length} Groups
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div className="space-y-4">
                {viewMode === 'size' ? (
                  (paginatedItems as SizeGroup[]).map((group, i) => {
                    const isSelected = selectedFiles.length > 0 && group.files.length > 0 && selectedFiles[0] === group.files[0].path
                    return (
                      <motion.div
                        key={i}
                        layoutId={`group-${viewMode}-${i}`} // Smooth layout transitions
                        onClick={() => setSelectedFiles(group.files.map(f => f.path))}
                        initial={{ opacity: 0, x: -20 }}
                        animate={{ opacity: 1, x: 0 }}
                        className={`glass-card p-4 rounded-2xl border transition-all cursor-pointer ${isSelected
                          ? 'border-blue-500 shadow-lg shadow-blue-500/20 bg-blue-500/5'
                          : 'border-white/5 hover:border-blue-500/30'
                          }`}
                      >
                        <div className="flex justify-between items-center mb-4">
                          <div className="flex items-center gap-2">
                            <span className={`text-[10px] font-black uppercase tracking-widest transition-colors ${isSelected ? 'text-blue-400' : 'text-blue-500/60'}`}>
                              Group {((currentPage - 1) * itemsPerPage) + i + 1}
                            </span>
                            <button
                              onClick={(e) => handleMarkAsGood(e, group.files)}
                              className="p-1.5 hover:bg-green-500/20 rounded-lg text-green-500/40 hover:text-green-400 transition-all group/btn"
                              title="Mark as GOOD (Files are same size but NOT duplicates)"
                            >
                              <CheckCircle2 className="w-3.5 h-3.5" />
                            </button>
                          </div>
                          <span className="text-xs font-bold bg-white/5 px-3 py-1 rounded-full text-gray-400 tracking-tighter">
                            Weight: {(group.size / (1024 * 1024)).toFixed(1)} MB
                          </span>
                        </div>
                        <div className="space-y-2">
                          {group.files.map((file) => (
                            <FileItem key={file.path} file={file} onRefresh={fetchData} />
                          ))}
                        </div>
                      </motion.div>
                    )
                  })
                ) : viewMode === 'similar' ? (
                  (paginatedItems as SimilarityGroup[]).map((group, i) => {
                    const isSelected = selectedFiles.length > 0 && group.files.length > 0 && selectedFiles[0] === group.files[0].path
                    return (
                      <motion.div
                        key={i}
                        layoutId={`group-${viewMode}-${i}`}
                        onClick={() => setSelectedFiles(group.files.map(f => f.path))}
                        initial={{ opacity: 0, scale: 0.95 }}
                        animate={{ opacity: 1, scale: 1 }}
                        className={`glass-card p-4 rounded-2xl border transition-all cursor-pointer ${isSelected
                          ? 'border-cyan-500 shadow-lg shadow-cyan-500/20 bg-cyan-500/5'
                          : 'border-white/5 hover:border-cyan-500/30 bg-gradient-to-r from-cyan-900/10 to-transparent'
                          }`}
                      >
                        <div className="flex justify-between items-center mb-4">
                          <div className="flex items-center gap-2 max-w-[70%]">
                            <span className={`text-[10px] font-black uppercase tracking-widest truncate transition-colors ${isSelected ? 'text-cyan-400' : 'text-cyan-500/60'}`}>
                              Cluster: {group.base_name || "Unknown"}
                            </span>
                            <button
                              onClick={(e) => handleMarkAsGood(e, group.files)}
                              className="p-1.5 hover:bg-green-500/20 rounded-lg text-green-500/40 hover:text-green-400 transition-all"
                              title="Mark as GOOD (Files are NOT duplicates)"
                            >
                              <CheckCircle2 className="w-3.5 h-3.5" />
                            </button>
                          </div>
                          <span className="text-xs font-bold bg-white/5 px-3 py-1 rounded-full text-gray-400 tracking-tighter">
                            {group.files.length} Files
                          </span>
                        </div>
                        <div className="space-y-2">
                          {/* Sort by size descending within group for better visibility */}
                          {[...group.files].sort((a, b) => b.size - a.size).map((file) => (
                            <FileItem key={file.path} file={file} onRefresh={fetchData} />
                          ))}
                        </div>
                      </motion.div>
                    )
                  })
                ) : (
                  (paginatedItems as SimilarityGroup[]).map((group, i) => {
                    const isSelected = selectedFiles.length > 0 && group.files.length > 0 && selectedFiles[0] === group.files[0].path
                    return (
                      <motion.div
                        key={i}
                        layoutId={`group-${viewMode}-${i}`}
                        onClick={() => setSelectedFiles(group.files.map(f => f.path))}
                        initial={{ opacity: 0, scale: 0.95 }}
                        animate={{ opacity: 1, scale: 1 }}
                        className={`glass-card p-4 rounded-2xl border transition-all cursor-pointer ${isSelected
                          ? 'border-orange-500 shadow-lg shadow-orange-500/20 bg-orange-500/5'
                          : 'border-white/5 hover:border-orange-500/30 bg-gradient-to-r from-orange-900/10 to-transparent'
                          }`}
                      >
                        <div className="flex justify-between items-center mb-4">
                          <div className="flex items-center gap-2 max-w-[70%]">
                            <span className={`text-[10px] font-black uppercase tracking-widest truncate transition-colors ${isSelected ? 'text-orange-400' : 'text-orange-500/60'}`}>
                              Visual Perceptual Match: {group.base_name || "Unknown"}
                            </span>
                            <button
                              onClick={(e) => handleMarkAsGood(e, group.files)}
                              className="p-1.5 hover:bg-green-500/20 rounded-lg text-green-500/40 hover:text-green-400 transition-all"
                              title="Mark as GOOD (Files are NOT duplicates)"
                            >
                              <CheckCircle2 className="w-3.5 h-3.5" />
                            </button>
                          </div>
                          <div className="flex items-center gap-2">
                            <div className="px-2 py-0.5 rounded bg-orange-500/20 text-[10px] font-bold text-orange-400 uppercase tracking-widest border border-orange-500/30">
                              A.I. Confirmed
                            </div>
                            <span className="text-xs font-bold bg-white/5 px-3 py-1 rounded-full text-gray-400 tracking-tighter">
                              {group.files.length} Files
                            </span>
                          </div>
                        </div>
                        <div className="space-y-4">
                          <div className="grid grid-cols-2 gap-4">
                            {group.files.slice(0, 2).map(f => (
                              <div key={f.path} className="rounded-lg overflow-hidden border border-white/5">
                                <PreviewImage path={f.path} />
                              </div>
                            ))}
                          </div>
                          <div className="space-y-2">
                            {group.files.map((file) => (
                              <FileItem key={file.path} file={file} onRefresh={fetchData} />
                            ))}
                          </div>
                        </div>
                      </motion.div>
                    )
                  })
                )}

                {currentItems.length === 0 && (
                  <div className="flex flex-col items-center justify-center py-20 bg-glass-layer rounded-3xl border border-dashed border-glass-border">
                    <Box className="w-12 h-12 text-gray-400 dark:text-gray-600 mb-4" />
                    <p className="text-gray-500 font-bold uppercase tracking-widest">No duplicates found</p>
                  </div>
                )}
              </div>

              {/* Pagination Controls */}
              {totalPages > 1 && (
                <div className="flex justify-center items-center gap-2 mt-12">
                  <button
                    onClick={() => handlePageChange(currentPage - 1)}
                    disabled={currentPage === 1}
                    className="p-3 bg-glass-layer rounded-xl text-gray-400 hover:text-foreground disabled:opacity-30 disabled:cursor-not-allowed transition-all"
                  >
                    <Zap className="w-4 h-4 rotate-180" />
                  </button>

                  <div className="flex gap-2">
                    {[...Array(totalPages)].map((_, i) => {
                      const page = i + 1
                      if (
                        page === 1 ||
                        page === totalPages ||
                        (page >= currentPage - 1 && page <= currentPage + 1)
                      ) {
                        return (
                          <button
                            key={page}
                            onClick={() => handlePageChange(page)}
                            className={`w-10 h-10 rounded-xl text-[10px] font-black transition-all ${currentPage === page
                              ? 'bg-blue-600 text-white shadow-lg shadow-blue-500/20'
                              : 'bg-glass-layer text-gray-500 hover:text-gray-300'
                              }`}
                          >
                            {page}
                          </button>
                        )
                      } else if (
                        page === currentPage - 2 ||
                        page === currentPage + 2
                      ) {
                        return <span key={page} className="w-10 h-10 flex items-center justify-center text-gray-700">...</span>
                      }
                      return null
                    })}
                  </div>

                  <button
                    onClick={() => handlePageChange(currentPage + 1)}
                    disabled={currentPage === totalPages}
                    className="p-3 bg-glass-layer rounded-xl text-gray-400 hover:text-foreground disabled:opacity-30 disabled:cursor-not-allowed transition-all"
                  >
                    <Zap className="w-4 h-4" />
                  </button>

                  <div className="relative ml-4">
                    <select
                      value={itemsPerPage}
                      onChange={(e) => setItemsPerPage(Number(e.target.value))}
                      className="appearance-none bg-glass-layer border border-glass-border rounded-xl px-4 py-3 text-[10px] font-black uppercase tracking-widest text-gray-400 focus:outline-none focus:border-blue-500/50 transition-all cursor-pointer pr-8 hover:bg-glass-border hover:text-gray-200"
                    >
                      <option value={10}>10</option>
                      <option value={20}>20</option>
                      <option value={50}>50</option>
                      <option value={100}>100</option>
                    </select>
                    <Filter className="absolute right-2 top-1/2 -translate-y-1/2 w-3 h-3 text-gray-500 pointer-events-none" />
                  </div>
                </div>
              )}
            </section>
          </div>

          {/* Right Column: Actions and Intelligence */}
          <div className="lg:col-span-4 xl:col-span-3 space-y-8">
            <ModelPreview selectedFiles={selectedFiles} />

            <div className="glass-card p-6 rounded-3xl border border-blue-500/20 sticky top-8">
              <h3 className="text-lg font-black mb-6 text-foreground uppercase tracking-widest flex items-center gap-3">
                <Cpu className="w-5 h-5 text-blue-500" />
                Analysis Expert
              </h3>

              <div className="bg-blue-500/10 p-4 rounded-2xl border border-blue-500/20 mb-8">
                <p className="text-xs text-blue-600 dark:text-blue-200 leading-relaxed font-medium">
                  I found <span className="text-foreground font-black">{data?.size_groups?.length} identical size groups</span>. These are highly likely to be the same content with renamed files. Deleting one version is safe.
                </p>
              </div>

              <div className="space-y-4">
                <button className="w-full py-4 bg-gradient-to-r from-blue-600 to-blue-500 hover:from-blue-500 hover:to-blue-400 text-white font-black text-xs uppercase tracking-[0.2em] rounded-2xl transition-all shadow-xl shadow-blue-500/10 flex items-center justify-center gap-3 active:scale-95">
                  <Trash2 className="w-4 h-4" />
                  Auto-Cleanup Oldest
                </button>
                <button
                  onClick={handleOpenDirectory}
                  className="w-full py-4 glass-card border-glass-border hover:border-blue-500/40 text-gray-400 hover:text-foreground font-black text-xs uppercase tracking-[0.2em] rounded-2xl transition-all flex items-center justify-center gap-3 active:scale-95"
                >
                  <ExternalLink className="w-4 h-4" />
                  Browse Directory
                </button>

                <button
                  onClick={handleRunStep3}
                  disabled={data?.status === 'analyzing_step3' || data?.status === 'analyzing_visual'}
                  className="w-full py-4 glass-card border-white/10 hover:border-cyan-500/40 text-gray-400 hover:text-white font-black text-xs uppercase tracking-[0.2em] rounded-2xl transition-all flex items-center justify-center gap-3 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {data?.status === 'analyzing_step3' ? (
                    <div className="flex flex-col items-center w-full px-4">
                      <span className="mb-2">Scanning Names... {(data.progress || 0).toFixed(0)}%</span>
                      <div className="w-full h-1 bg-white/10 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-cyan-500 transition-all duration-300 ease-out"
                          style={{ width: `${data.progress || 0}%` }}
                        />
                      </div>
                    </div>
                  ) : (
                    <>
                      <FileText className="w-4 h-4 text-cyan-500" />
                      Similar Name Scan
                    </>
                  )}
                </button>

                <button
                  onClick={handleRunVisual}
                  disabled={data?.status === 'analyzing_step3' || data?.status === 'analyzing_visual'}
                  className="w-full py-4 glass-card border-white/10 hover:border-orange-500/40 text-gray-400 hover:text-white font-black text-xs uppercase tracking-[0.2em] rounded-2xl transition-all flex items-center justify-center gap-3 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {data?.status === 'analyzing_visual' ? (
                    <div className="flex flex-col items-center w-full px-4">
                      <span className="mb-2">A.I. Visual Scan... {(data.progress || 0).toFixed(0)}%</span>
                      <div className="w-full h-1 bg-white/10 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-orange-500 transition-all duration-300 ease-out"
                          style={{ width: `${data.progress || 0}%` }}
                        />
                      </div>
                    </div>
                  ) : (
                    <>
                      <ImageIcon className="w-4 h-4 text-orange-500" />
                      A.I. Visual Match Scan
                    </>
                  )}
                </button>
              </div>

              <div className="mt-12 pt-8 border-t border-white/5">
                <div className="flex items-center gap-3 text-[10px] font-black uppercase tracking-widest text-gray-600">
                  <div className="w-1.5 h-1.5 rounded-full bg-green-500" />
                  Scanner Core Online
                </div>
                <div className="mt-4 text-[9px] text-gray-700 font-bold leading-tight">
                  ANTIGRAVITY INTELLIGENCE<br />
                  DEPLOYED: 2026-01-05
                </div>
              </div>
            </div>
          </div>
        </div>
      </div >

      {/* Settings Modal */}
      <AnimatePresence>
        {showSettings && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-[100] flex items-center justify-center p-6 bg-black/60 backdrop-blur-md"
            onClick={() => setShowSettings(false)}
          >
            <motion.div
              initial={{ scale: 0.9, opacity: 0, y: 20 }}
              animate={{ scale: 1, opacity: 1, y: 0 }}
              exit={{ scale: 0.9, opacity: 0, y: 20 }}
              className="w-full max-w-lg glass-card p-10 rounded-[2.5rem] border border-blue-500/30 shadow-2xl relative"
              onClick={e => e.stopPropagation()}
            >
              <div className="flex items-center gap-4 mb-8">
                <div className="w-12 h-12 rounded-2xl bg-blue-600/20 flex items-center justify-center border border-blue-500/30">
                  <Settings className="w-6 h-6 text-blue-400" />
                </div>
                <h2 className="text-3xl font-black tracking-tight text-white">SETTINGS</h2>
              </div>

              <div className="space-y-8">
                {/* Cache Management Section */}
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <HardDrive className="w-5 h-5 text-gray-400" />
                      <h3 className="text-lg font-bold text-gray-200">Cache Management</h3>
                    </div>
                    <button
                      onClick={handleClearCache}
                      className="px-4 py-2 bg-red-500/10 hover:bg-red-500/20 text-red-400 text-sm font-bold rounded-xl border border-red-500/20 transition-all"
                    >
                      CLEAR CACHE
                    </button>
                  </div>

                  <div className="bg-white/5 p-4 rounded-2xl border border-white/5">
                    <div className="flex justify-between mb-2">
                      <span className="text-sm text-gray-400">Current Cache Size</span>
                      <span className="text-sm font-bold text-white">{cacheStats?.size_gb.toFixed(2)} GB</span>
                    </div>
                    <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-blue-500 transition-all"
                        style={{ width: `${Math.min(100, (cacheStats?.size_gb || 0) / (cacheStats?.limit_gb || 1) * 100)}%` }}
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <div className="flex justify-between items-center">
                      <label className="text-sm font-bold text-gray-400 uppercase tracking-widest">
                        Auto-Clear Threshold {cacheStats?.limit_gb && cacheStats.limit_gb > 0 ? `(${cacheStats.limit_gb} GB)` : '(Disabled)'}
                      </label>
                    </div>
                    <input
                      type="range"
                      min="0"
                      max="50"
                      step="1"
                      value={cacheStats?.limit_gb || 0}
                      onChange={(e) => handleUpdateCacheLimit(parseFloat(e.target.value))}
                      className="w-full accent-blue-500 h-2 bg-white/10 rounded-lg appearance-none cursor-pointer"
                    />
                    <div className="flex justify-between text-[10px] text-gray-500 font-bold uppercase tracking-tighter">
                      <span>Off</span>
                      <span>10GB</span>
                      <span>20GB</span>
                      <span>30GB</span>
                      <span>40GB</span>
                      <span>50GB</span>
                    </div>
                    <p className="text-xs text-gray-500 italic mt-2">
                      Automatically clears cached previews and icons when the limit is reached.
                    </p>
                  </div>
                </div>
              </div>

              <button
                onClick={() => setShowSettings(false)}
                className="mt-10 w-full py-4 bg-gradient-to-r from-blue-600 to-cyan-600 hover:from-blue-500 hover:to-cyan-500 rounded-2xl text-sm font-black text-white transition-all shadow-lg shadow-blue-500/20 uppercase tracking-widest"
              >
                Close Settings
              </button>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div >
  )
}
