"use client"

import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
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
  Folder
} from 'lucide-react'
import ModelPreview from '@/components/ModelPreview'

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

interface SimilarPair {
  file1: FileInfo
  file2: FileInfo
  similarity: number
}

interface Report {
  total_files: number
  size_groups: SizeGroup[]
  similar_pairs: SimilarPair[]
  analysis_duration_seconds: number
  status?: string
}

function PreviewImage({ path }: { path: string }) {
  const [imgUrl, setImgUrl] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    setLoading(true)
    fetch(`${apiHost}/api/preview?path=${encodeURIComponent(path)}`)
      .then(res => {
        if (!res.ok) throw new Error('No preview')
        return res.blob()
      })
      .then(blob => {
        setImgUrl(URL.createObjectURL(blob))
        setLoading(false)
      })
      .catch(() => {
        setError(true)
        setLoading(false)
      })
  }, [path])

  if (loading) return (
    <div className="w-full aspect-video flex items-center justify-center bg-black/40 rounded-lg">
      <Loader2 className="w-6 h-6 text-blue-500 animate-spin" />
    </div>
  )

  if (error || !imgUrl) return (
    <div className="w-full aspect-video flex flex-col items-center justify-center bg-black/40 rounded-lg text-gray-600">
      <ImageIcon className="w-8 h-8 mb-1 opacity-20" />
      <span className="text-[10px] font-bold uppercase tracking-widest opacity-40">No Preview Found</span>
    </div>
  )

  return (
    <div className="relative w-full aspect-video rounded-lg overflow-hidden border border-white/10">
      <img src={imgUrl} alt="Preview" className="w-full h-full object-cover" />
      <div className="absolute inset-0 bg-gradient-to-t from-black/60 to-transparent" />
      <div className="absolute bottom-2 left-3 flex items-center gap-2">
        <ImageIcon className="w-3 h-3 text-blue-400" />
        <span className="text-[8px] font-bold text-white uppercase tracking-tighter">Archive Intelligence Preview</span>
      </div>
    </div>
  )
}

function FileItem({ file }: { file: FileInfo }) {
  const [isHovered, setIsHovered] = useState(false)

  const handleOpen = (e: React.MouseEvent, mode: 'reveal' | 'launch') => {
    e.stopPropagation()
    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
    fetch(`${apiHost}/api/open?path=${encodeURIComponent(file.path)}&mode=${mode}`)
      .catch(err => console.error(`Failed to ${mode} file:`, err))
  }

  const handleDelete = async (e: React.MouseEvent) => {
    e.stopPropagation()
    if (!confirm(`Are you sure you want to move this file to trash?\n${file.name}`)) return

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
    } catch (err) {
      console.error("Failed to delete file:", err)
      alert("Error: " + err)
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
        <p className="text-sm font-bold text-gray-200 truncate">{file.name}</p>
        <p className="text-[10px] text-gray-500 font-medium truncate opacity-60 uppercase tracking-tighter">{file.path}</p>
      </div>
      <div className="flex gap-2 opacity-0 group-hover/file:opacity-100 transition-opacity">
        <button
          onClick={(e) => handleOpen(e, 'reveal')}
          className="p-2 bg-blue-500/10 hover:bg-blue-500/20 rounded-lg text-blue-400 transition-all"
          title="Reveal in Folder"
        >
          <Folder className="w-4 h-4" />
        </button>
        <button
          onClick={handleDelete}
          className="p-2 bg-red-500/10 hover:bg-red-500/20 rounded-lg text-red-400 transition-all"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </div>

      <AnimatePresence>
        {isHovered && (
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 10 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 10 }}
            className="absolute left-0 bottom-full mb-3 w-64 z-[100] pointer-events-none"
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
  const [data, setData] = useState<Report | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedItem, setSelectedItem] = useState(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [fileType, setFileType] = useState('all')
  const [status, setStatus] = useState<string | null>(null) // New state for status
  const [notified, setNotified] = useState(false) // New state for notification

  const requestNotificationPermission = () => {
    if ('Notification' in window) {
      Notification.requestPermission()
    }
  }

  useEffect(() => {
    const fetchData = async () => {
      try {
        const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
        const response = await fetch(`${apiHost}/api/report`)
        const report: Report = await response.json()
        setData(report)

        // Handle notification
        // Only notify if status transitions from 'analyzing' to 'finished' and not already notified
        if (report.status === 'finished' && status === 'analyzing' && !notified) {
          if (Notification.permission === 'granted') {
            new Notification('🔍 Analysis Complete!', {
              body: `Found ${report.similar_pairs.length} similar file pairs.`,
              icon: '/favicon.ico'
            })
          }
          setNotified(true)
        }

        setStatus(report.status || null)
        setLoading(false)
      } catch (err) {
        console.error(err)
        setError("Could not connect to the Archive Finder backend. Make sure it's running with the -web flag.")
        setLoading(false)
      }
    }

    fetchData() // Initial fetch
    const interval = setInterval(fetchData, 3000) // Poll every 3 seconds
    return () => clearInterval(interval) // Cleanup interval on component unmount
  }, [status, notified]) // Dependencies for useEffect

  if (loading) return (
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
    { label: 'Similar Names', value: data?.similar_pairs?.length || 0, icon: FileText, color: 'text-cyan-400' },
    { label: 'Scan Time', value: `${data?.analysis_duration_seconds?.toFixed(2)}s`, icon: Clock, color: 'text-green-400' },
  ]

  const filteredSizeGroups = data?.size_groups?.filter(group => {
    return group.files.some(file => {
      const matchesSearch = file.name.toLowerCase().includes(searchQuery.toLowerCase())
      const matchesType = fileType === 'all' || file.name.toLowerCase().endsWith(`.${fileType.toLowerCase()}`)
      return matchesSearch && matchesType
    })
  }) || []

  const filteredSimilarPairs = data?.similar_pairs?.filter(pair => {
    const matchesSearch =
      pair.file1.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      pair.file2.name.toLowerCase().includes(searchQuery.toLowerCase())
    const matchesType = fileType === 'all' ||
      pair.file1.name.toLowerCase().endsWith(`.${fileType.toLowerCase()}`) ||
      pair.file2.name.toLowerCase().endsWith(`.${fileType.toLowerCase()}`)
    return matchesSearch && matchesType
  }) || []

  const fileTypes = ['all', 'zip', 'rar', '7z', 'stl']

  return (
    <div className="min-h-screen bg-[#0a0a0c] text-slate-200 p-4 md:p-8">
      {/* Header */}
      <header className="flex flex-col md:flex-row justify-between items-start md:items-center mb-12 gap-4">
        <div>
          <h1 className="text-4xl font-black tracking-tight flex items-center gap-3">
            <span className="bg-gradient-to-r from-blue-500 to-cyan-400 bg-clip-text text-transparent">ARCHIVE</span>
            <span className="text-white">FINDER</span>
            <div className="px-2 py-0.5 bg-blue-500/10 border border-blue-500/20 rounded text-[10px] text-blue-400 uppercase tracking-widest font-bold">Intelligence v1.0</div>
          </h1>
          <p className="text-gray-500 mt-1 font-medium tracking-wide">3D Asset Deduplication & Management Dashboard</p>
        </div>
        <div className="flex gap-4">
          <div className="flex items-center gap-4">
            <button
              onClick={requestNotificationPermission}
              className="px-4 py-2 bg-white/5 hover:bg-white/10 rounded-xl text-xs font-medium text-gray-400 transition-all border border-white/10"
            >
              🔔 Enable Notifications
            </button>
            <div className="flex items-center gap-2 px-4 py-2 bg-white/5 rounded-xl border border-white/10">
              <div className={`w-2 h-2 rounded-full ${data?.status === 'finished' ? 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)]' : 'bg-yellow-500 animate-pulse'}`} />
              <span className="text-xs font-medium text-gray-300 uppercase tracking-widest">
                {data?.status || 'Analyzing'}
              </span>
            </div>
          </div>
        </div>
      </header>

      {/* Filter Bar */}
      <div className="flex flex-col md:flex-row gap-4 mb-8">
        <div className="relative flex-1 group">
          <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-500 group-focus-within:text-blue-500 transition-colors" />
          <input
            type="text"
            placeholder="Search by filename..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full bg-white/5 border border-white/5 rounded-2xl py-4 pl-12 pr-4 text-sm font-medium focus:outline-none focus:border-blue-500/50 focus:bg-white/[0.08] transition-all"
          />
        </div>
        <div className="flex gap-2">
          {fileTypes.map(type => (
            <button
              key={type}
              onClick={() => setFileType(type)}
              className={`px-6 py-2 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all ${fileType === type
                ? 'bg-blue-600 text-white shadow-lg shadow-blue-500/20'
                : 'bg-white/5 text-gray-500 hover:text-gray-300 hover:bg-white/10'
                }`}
            >
              {type}
            </button>
          ))}
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-6 mb-12">
        {stats.map((stat, i) => (
          <motion.div
            key={stat.label}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: i * 0.1 }}
            className="glass-card p-6 rounded-3xl relative overflow-hidden group hover:scale-[1.02] transition-all cursor-default"
          >
            <div className={`absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-blue-500/0 via-blue-500/50 to-blue-500/0 group-hover:via-blue-400 transition-all`} />
            <div className="flex justify-between items-center mb-4">
              <stat.icon className={`w-6 h-6 ${stat.color} opacity-80`} />
              <div className="w-8 h-8 rounded-full bg-white/5 flex items-center justify-center">
                <div className="w-1.5 h-1.5 rounded-full bg-white/20" />
              </div>
            </div>
            <div className="text-3xl font-black text-white glow-text">{stat.value}</div>
            <div className="text-[10px] font-bold text-gray-500 uppercase tracking-widest mt-1">{stat.label}</div>
          </motion.div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* Left Column: Listings */}
        <div className="lg:col-span-8 space-y-8">

          {/* Section: Identical Sizes */}
          <section>
            <div className="flex items-center gap-4 mb-6">
              <div className="p-2 bg-blue-500/20 rounded-xl">
                <Layers className="w-5 h-5 text-blue-400" />
              </div>
              <h2 className="text-xl font-bold text-white uppercase tracking-widest">Identical Size Groups</h2>
              <div className="flex-1 h-px bg-white/5" />
            </div>

            <div className="space-y-4">
              {filteredSizeGroups.map((group, i) => (
                <motion.div
                  key={i}
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  className="glass-card p-4 rounded-2xl border border-white/5 hover:border-blue-500/30 transition-all"
                >
                  <div className="flex justify-between items-center mb-4">
                    <span className="text-[10px] font-black text-blue-500/60 uppercase tracking-widest">Group {i + 1}</span>
                    <span className="text-xs font-bold bg-white/5 px-3 py-1 rounded-full text-gray-400 tracking-tighter">
                      Weight: {(group.size / (1024 * 1024)).toFixed(1)} MB
                    </span>
                  </div>
                  <div className="space-y-2">
                    {group.files.map((file, fi) => (
                      <FileItem key={fi} file={file} />
                    ))}
                  </div>
                </motion.div>
              ))}
            </div>
          </section>

          {/* Section: Similar Names */}
          <section>
            <div className="flex items-center gap-4 mb-6">
              <div className="p-2 bg-cyan-500/20 rounded-xl">
                <FileText className="w-5 h-5 text-cyan-400" />
              </div>
              <h2 className="text-xl font-bold text-white uppercase tracking-widest">Similarity Hits</h2>
              <div className="flex-1 h-px bg-white/5" />
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {filteredSimilarPairs.map((pair, i) => (
                <div key={i} className="glass-card p-5 rounded-2xl relative overflow-hidden">
                  <div className="flex flex-col gap-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <div className={`w-2 h-2 rounded-full ${pair.similarity > 90 ? 'bg-orange-500 shadow-[0_0_8px_rgba(249,115,22,0.5)]' : 'bg-yellow-500'}`} />
                        <span className="text-[10px] font-black uppercase tracking-widest text-gray-500">
                          Match: {pair.similarity.toFixed(1)}%
                        </span>
                      </div>
                      <AlertTriangle className={`w-4 h-4 ${pair.similarity > 90 ? 'text-orange-500' : 'text-yellow-500'} opacity-60`} />
                    </div>

                    <div className="space-y-3 mt-2">
                      <FileItem file={pair.file1} />
                      <div className="flex justify-center -my-2 relative z-10">
                        <div className="w-6 h-6 rounded-full bg-blue-500 flex items-center justify-center scale-90 shadow-lg shadow-blue-500/20">
                          <Search className="w-3 h-3 text-white" />
                        </div>
                      </div>
                      <FileItem file={pair.file2} />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </section>
        </div>

        {/* Right Column: Actions and Intelligence */}
        <div className="lg:col-span-4 space-y-8">
          <ModelPreview />

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
  )
}
