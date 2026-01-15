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
  Grid3x3
} from 'lucide-react'
import ModelPreview from '@/components/ModelPreview'

interface FileInfo {
  name: string
  path: string
  size: number
  mod_time: string
  p_hash?: number
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

function PreviewImage({ path }: { path: string }) {
  const [error, setError] = useState(false)
  const [isHovering, setIsHovering] = useState(true)

  const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
  const previewUrl = `${apiHost}/api/preview?path=${encodeURIComponent(path)}`

  // Basic extension check for UI hints
  const isVideo = /\.(mp4|webm|mov|mkv|avi)$/i.test(path)
  const is3D = /\.(stl|obj|3mf)$/i.test(path)

  return (
    <div className="relative w-full aspect-video rounded-lg overflow-hidden border border-white/10 bg-black/40 flex items-center justify-center">
      {error ? (
        <div className="flex flex-col items-center justify-center opacity-40">
          <ImageIcon className="w-8 h-8 mb-1" />
          <span className="text-[10px] font-bold uppercase tracking-widest">Preview Error</span>
        </div>
      ) : is3D ? (
        <div className="flex flex-col items-center justify-center text-blue-400 opacity-60">
          <Box className="w-10 h-10 mb-1" />
          <span className="text-[10px] font-bold uppercase tracking-widest">3D Model</span>
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
        <span className="text-[8px] font-bold text-white/80 uppercase tracking-widest">AI Preview Stream</span>
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
      className="relative flex items-center gap-3 p-3 bg-white/5 rounded-xl group/file cursor-pointer hover:bg-white/[0.08] transition-all"
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      onClick={(e) => handleOpen(e, 'launch')}
    >
      <div className="w-10 h-10 rounded-lg bg-black/40 flex items-center justify-center text-blue-500/50 group-hover/file:text-blue-400 transition-colors">
        <Box className="w-5 h-5" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <p className="text-sm font-bold text-gray-200 truncate">{file.name}</p>
          <span className="text-[10px] font-black px-1.5 py-0.5 rounded bg-white/5 text-gray-500 uppercase tracking-tighter">
            {formatBytes(file.size)}
          </span>
        </div>
        <p className="text-[10px] text-gray-500 font-medium truncate opacity-60 uppercase tracking-tighter">{file.path}</p>
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
    return data.size_groups.filter(group => {
      return (group?.files || []).some(file => {
        const name = (file?.name || '').toLowerCase()
        const matchesSearch = name.includes(query)
        const matchesType = fileType === 'all' || name.endsWith(`.${fileType.toLowerCase()}`)
        return matchesSearch && matchesType
      })
    }) || []
  }, [data?.size_groups, searchQuery, fileType])

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
      // Trigger a status update manually or let the poller catch it
      setStatus('analyzing')
    } catch (err) {
      console.error("Failed to run Step 3:", err)
      alert("Error triggering Step 3")
    }
  }

  if (!mounted || loading) return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-[#0a0a0c] text-white">
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
    <div className="flex flex-col items-center justify-center min-h-screen bg-[#0a0a0c] text-white p-6">
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

  const stats = [
    { label: 'Total Files', value: data?.total_files || 0, icon: Box, color: 'text-blue-400' },
    { label: 'Size Groups', value: data?.size_groups?.length || 0, icon: Layers, color: 'text-purple-400' },
    { label: 'Similar Names', value: data?.similar_groups?.length || 0, icon: FileText, color: 'text-cyan-400' },
    { label: 'Visual Matches', value: data?.visual_groups?.length || 0, icon: ImageIcon, color: 'text-orange-400' },
    { label: 'Scan Time', value: `${data?.analysis_duration_seconds?.toFixed(2) || 0}s`, icon: Clock, color: 'text-green-400' },
  ]

  const fileTypes = ['all', 'zip', 'rar', '7z', 'stl']

  return (
    <div className="min-h-screen bg-[#0a0a0c] text-slate-200 p-8 md:p-12 flex flex-col items-center">
      <div className="w-full max-w-[1700px] transition-all duration-500 ease-in-out">
        {/* Header */}
        <header className="flex flex-col md:flex-row justify-between items-start md:items-center mb-16 gap-6">
          <div>
            <h1 className="text-5xl font-black tracking-tight flex items-center gap-4">
              <span className="bg-gradient-to-r from-blue-500 to-cyan-400 bg-clip-text text-transparent">ARCHIVE</span>
              <span className="text-white">FINDER</span>
              <div className="px-3 py-1 bg-blue-500/10 border border-blue-500/20 rounded-md text-xs text-blue-400 uppercase tracking-widest font-bold">Intelligence v1.8.0</div>
            </h1>
            <p className="text-gray-500 mt-2 font-medium tracking-wide text-lg">3D Asset Deduplication & Management Dashboard</p>
          </div>
          <div className="flex gap-4">
            <div className="flex items-center gap-4">
              <Link href="/gallery">
                <button className="px-6 py-3 bg-gradient-to-r from-blue-600 to-cyan-600 hover:from-blue-500 hover:to-cyan-500 rounded-2xl text-sm font-bold text-white transition-all border border-blue-500/20 shadow-lg shadow-blue-500/20 flex items-center gap-2">
                  <Grid3x3 className="w-5 h-5" />
                  Gallery View
                </button>
              </Link>
              <button
                onClick={requestNotificationPermission}
                className="px-6 py-3 bg-white/5 hover:bg-white/10 rounded-2xl text-sm font-medium text-gray-400 transition-all border border-white/10"
              >
                🔔 Enable Notifications
              </button>
              <div className="flex items-center gap-3 px-6 py-3 bg-white/5 rounded-2xl border border-white/10">
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
              className="w-full bg-white/5 border border-white/5 rounded-3xl py-5 pl-16 pr-6 text-base font-medium focus:outline-none focus:border-blue-500/50 focus:bg-white/[0.08] transition-all"
            />
          </div>

          <div className="flex flex-wrap gap-3 items-center flex-grow sm:flex-grow-0">
            <div className="flex gap-2 bg-white/5 p-1.5 rounded-3xl border border-white/5">
              <button
                onClick={() => setViewMode('size')}
                className={`px-6 py-4 rounded-2xl text-sm font-bold uppercase tracking-wide transition-all flex items-center gap-3 whitespace-nowrap ${viewMode === 'size'
                  ? 'bg-blue-600 text-white shadow-lg shadow-blue-500/20'
                  : 'text-gray-500 hover:text-gray-300'
                  }`}
              >
                <Layers className="w-5 h-5" />
                Size Matches
              </button>
              <button
                onClick={() => setViewMode('similar')}
                className={`px-6 py-4 rounded-2xl text-sm font-bold uppercase tracking-wide transition-all flex items-center gap-3 whitespace-nowrap ${viewMode === 'similar'
                  ? 'bg-cyan-600 text-white shadow-lg shadow-cyan-500/20'
                  : 'text-gray-500 hover:text-gray-300'
                  }`}
              >
                <FileText className="w-5 h-5" />
                Similar Names
              </button>
              <button
                onClick={() => setViewMode('visual')}
                className={`px-6 py-4 rounded-2xl text-sm font-bold uppercase tracking-wide transition-all flex items-center gap-3 whitespace-nowrap ${viewMode === 'visual'
                  ? 'bg-orange-600 text-white shadow-lg shadow-orange-500/20'
                  : 'text-gray-500 hover:text-gray-300'
                  }`}
              >
                <ImageIcon className="w-5 h-5" />
                Visual Hits
              </button>
            </div>

            <div className="relative flex-grow sm:flex-grow-0">
              <select
                value={itemsPerPage}
                onChange={(e) => setItemsPerPage(Number(e.target.value))}
                className="appearance-none w-full bg-white/5 border border-white/5 rounded-3xl px-8 py-4 text-sm font-bold uppercase tracking-wide text-gray-400 focus:outline-none focus:border-blue-500/50 transition-all cursor-pointer min-w-[160px]"
              >
                <option value={10}>10 Per Page</option>
                <option value={20}>20 Per Page</option>
                <option value={50}>50 Per Page</option>
                <option value={100}>100 Per Page</option>
              </select>
              <Filter className="absolute right-5 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-600 pointer-events-none" />
            </div>

            <div className="flex gap-2 bg-white/5 p-1.5 rounded-3xl border border-white/5 h-full overflow-x-auto max-w-full">
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
                className={`glass-card p-4 rounded-[1.2rem] relative overflow-hidden group hover:scale-[1.03] transition-all cursor-pointer h-full min-h-[110px] flex flex-col justify-between border border-white/5 ${(stat.label === 'Size Groups' && viewMode === 'size') || (stat.label === 'Similar Names' && viewMode === 'similar') || (stat.label === 'Visual Matches' && viewMode === 'visual')
                  ? 'border-blue-500/40 shadow-[0_0_20px_rgba(59,130,246,0.1)] bg-blue-500/5'
                  : 'hover:border-white/20'
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
                    <div className={`p-1.5 rounded-lg bg-white/5 border border-white/5 ${stat.color}`}>
                      <stat.icon className="w-4 h-4" />
                    </div>
                    <div className="text-[9px] font-black text-gray-500 uppercase tracking-[0.15em]">{stat.label}</div>
                  </div>

                  <div className="mt-1">
                    <div className={`text-2xl lg:text-3xl font-black text-white glow-text tracking-tighter truncate leading-none`}>
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
                          <span className={`text-[10px] font-black uppercase tracking-widest transition-colors ${isSelected ? 'text-blue-400' : 'text-blue-500/60'}`}>
                            Group {((currentPage - 1) * itemsPerPage) + i + 1}
                          </span>
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
                          <span className={`text-[10px] font-black uppercase tracking-widest truncate max-w-[70%] transition-colors ${isSelected ? 'text-cyan-400' : 'text-cyan-500/60'}`}>
                            Cluster: {group.base_name || "Unknown"}
                          </span>
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
                          <span className={`text-[10px] font-black uppercase tracking-widest truncate max-w-[70%] transition-colors ${isSelected ? 'text-orange-400' : 'text-orange-500/60'}`}>
                            Visual Perceptual Match: {group.base_name || "Unknown"}
                          </span>
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
                  <div className="flex flex-col items-center justify-center py-20 bg-white/5 rounded-3xl border border-dashed border-white/10">
                    <Box className="w-12 h-12 text-gray-700 mb-4" />
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
                    className="p-3 bg-white/5 rounded-xl text-gray-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-all"
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
                              : 'bg-white/5 text-gray-500 hover:text-gray-300'
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
                    className="p-3 bg-white/5 rounded-xl text-gray-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-all"
                  >
                    <Zap className="w-4 h-4" />
                  </button>
                </div>
              )}
            </section>
          </div>

          {/* Right Column: Actions and Intelligence */}
          <div className="lg:col-span-4 xl:col-span-3 space-y-8">
            <ModelPreview selectedFiles={selectedFiles} />

            <div className="glass-card p-6 rounded-3xl border border-blue-500/20 sticky top-8">
              <h3 className="text-lg font-black mb-6 text-white uppercase tracking-widest flex items-center gap-3">
                <Cpu className="w-5 h-5 text-blue-500" />
                Analysis Expert
              </h3>

              <div className="bg-blue-500/10 p-4 rounded-2xl border border-blue-500/20 mb-8">
                <p className="text-xs text-blue-200 leading-relaxed font-medium">
                  I found <span className="text-white font-black">{data?.size_groups?.length} identical size groups</span>. These are highly likely to be the same content with renamed files. Deleting one version is safe.
                </p>
              </div>

              <div className="space-y-4">
                <button className="w-full py-4 bg-gradient-to-r from-blue-600 to-blue-500 hover:from-blue-500 hover:to-blue-400 text-white font-black text-xs uppercase tracking-[0.2em] rounded-2xl transition-all shadow-xl shadow-blue-500/10 flex items-center justify-center gap-3 active:scale-95">
                  <Trash2 className="w-4 h-4" />
                  Auto-Cleanup Oldest
                </button>
                <button className="w-full py-4 glass-card border-white/10 hover:border-blue-500/40 text-gray-400 hover:text-white font-black text-xs uppercase tracking-[0.2em] rounded-2xl transition-all flex items-center justify-center gap-3">
                  <ExternalLink className="w-4 h-4" />
                  Browse Directory
                </button>

                <button
                  onClick={handleRunStep3}
                  disabled={data?.status === 'analyzing_step3'}
                  className="w-full py-4 glass-card border-white/10 hover:border-cyan-500/40 text-gray-400 hover:text-white font-black text-xs uppercase tracking-[0.2em] rounded-2xl transition-all flex items-center justify-center gap-3 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {data?.status === 'analyzing_step3' ? (
                    <div className="flex flex-col items-center w-full px-4">
                      <span className="mb-2">Scanning... {(data.progress || 0).toFixed(0)}%</span>
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
                      Run Similarity Analysis
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
      </div>
    </div>
  )
}
