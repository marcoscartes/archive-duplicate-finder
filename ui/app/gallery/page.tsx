"use client"

import { useState, useEffect, useRef, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
    Search,
    Grid3x3,
    Loader2,
    Image as ImageIcon,
    Box,
    ArrowLeft,
    Folder,
    Trash2,
    AlertTriangle,
    ExternalLink,
    X,
    ChevronLeft,
    ChevronRight,
    Images,
    ArrowUpAZ,
    ArrowDownAZ,
    ArrowUp01,
    ArrowDown01,
    Clock,
    Filter,
    ArrowDownWideNarrow,
    Play
} from 'lucide-react'
import Link from 'next/link'
import dynamic from 'next/dynamic'

// Dynamically import the 3D viewer to avoid SSR issues
const STLViewer = dynamic(() => import('@/components/STLViewer'), { ssr: false })

interface FileInfo {
    name: string
    path: string
    size: number
    mod_time: string
    type?: string
}

interface GalleryResponse {
    files: FileInfo[]
    total: number
}

function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function GalleryItem({ file, index, onRefresh, onSelect }: { file: FileInfo, index: number, onRefresh?: () => void, onSelect: (index: number) => void }) {
    const [previewData, setPreviewData] = useState<{ url: string, type: 'image' | 'model' | 'video' } | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState(false)
    const [isDeleting, setIsDeleting] = useState(false)
    const [showConfirm, setShowConfirm] = useState(false)
    const [isVisible, setIsVisible] = useState(false)
    const itemRef = useRef<HTMLDivElement>(null)

    // Intersection Observer for lazy loading
    useEffect(() => {
        const observer = new IntersectionObserver(
            (entries) => {
                entries.forEach((entry) => {
                    if (entry.isIntersecting) {
                        setIsVisible(true)
                    }
                })
            },
            {
                rootMargin: '200px',
                threshold: 0.01
            }
        )

        if (itemRef.current) {
            observer.observe(itemRef.current)
        }

        return () => {
            if (itemRef.current) {
                observer.unobserve(itemRef.current)
            }
        }
    }, [])

    // Load preview when visible
    useEffect(() => {
        if (!isVisible) return

        const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
        const url = `${apiHost}/api/preview?path=${encodeURIComponent(file.path)}`

        if (file.type === 'video') {
            setPreviewData({ url, type: 'video' })
            setLoading(false)
            return
        }

        setLoading(true)
        fetch(url)
            .then(res => {
                if (!res.ok) throw new Error('No preview')
                const contentType = res.headers.get('content-type') || ''
                return res.blob().then(blob => ({ blob, contentType }))
            })
            .then(({ blob, contentType }) => {
                const objectUrl = URL.createObjectURL(blob)
                const isVideoExt = file.path.toLowerCase().match(/\.(mp4|webm|mkv|mov|avi)$/)
                let type: 'image' | 'model' | 'video' = isVideoExt ? 'video' : 'image'
                if (contentType.startsWith('model/')) type = 'model'
                else if (contentType.startsWith('video/')) type = 'video'

                setPreviewData({ url: type === 'video' ? url : objectUrl, type })
                if (type === 'video') URL.revokeObjectURL(objectUrl) // Don't need the blob for video
                setLoading(false)
            })
            .catch((err) => {
                console.error('Preview load error:', err)
                setError(true)
                setLoading(false)
            })
    }, [isVisible, file.path, file.type])

    // Cleanup blob URL only when component unmounts (only for images/models)
    useEffect(() => {
        return () => {
            if (previewData?.url && previewData.url.startsWith('blob:')) {
                URL.revokeObjectURL(previewData.url)
            }
        }
    }, [previewData?.url])

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

    const handleImageClick = (e: React.MouseEvent) => {
        e.stopPropagation()
        onSelect(index)
    }

    return (
        <div
            ref={itemRef}
            className="relative group bg-white/5 rounded-2xl overflow-hidden border border-white/10 hover:border-blue-500/30 transition-all w-full h-full cursor-pointer"
            onClick={handleImageClick}
        >
            {loading && (
                <div className="absolute inset-0 flex items-center justify-center bg-black/40">
                    <Loader2 className="w-8 h-8 text-blue-500 animate-spin" />
                </div>
            )}

            {error && !loading && (
                <div className="absolute inset-0 flex flex-col items-center justify-center bg-black/40 text-gray-600">
                    <ImageIcon className="w-12 h-12 mb-2 opacity-20" />
                    <span className="text-xs font-bold uppercase tracking-widest opacity-40">No Preview</span>
                </div>
            )}

            {previewData && !loading && (
                <>
                    {previewData.type === 'image' ? (
                        <img
                            src={previewData.url}
                            alt={file.name}
                            className="w-full h-full object-contain"
                        />
                    ) : previewData.type === 'video' ? (
                        <div className="relative w-full h-full overflow-hidden bg-black">
                            <video
                                src={previewData.url}
                                className="w-full h-full object-cover"
                                muted
                                playsInline
                                preload="metadata"
                                onLoadedMetadata={(e) => {
                                    const video = e.target as HTMLVideoElement;
                                    video.currentTime = Math.min(video.duration / 2, 60);
                                }}
                                onError={() => setError(true)}
                            />
                            <div className="absolute inset-0 flex flex-col items-center justify-center bg-black/20 group-hover:bg-black/0 transition-colors">
                                <Play className="w-12 h-12 text-white/90 drop-shadow-2xl opacity-80 group-hover:scale-110 transition-transform" />
                                <span className="text-[10px] font-black uppercase tracking-[0.2em] text-white/60 mt-2">Video</span>
                            </div>
                        </div>
                    ) : (
                        <div className="absolute inset-0 flex flex-col items-center justify-center bg-gradient-to-br from-blue-900/20 to-purple-900/20">
                            <Box className="w-16 h-16 mb-2 text-blue-400 opacity-60" />
                            <span className="text-xs font-bold uppercase tracking-widest text-blue-300">3D Model</span>
                            <span className="text-[10px] text-gray-400 mt-1">Click to view</span>
                        </div>
                    )}
                </>
            )}

            {/* File info footer - Always visible */}
            <div
                className="absolute bottom-0 left-0 right-0 p-3 bg-black/60 backdrop-blur-sm border-t border-white/5 pointer-events-auto hover:bg-black/80 transition-colors"
                onClick={(e) => { e.stopPropagation(); handleOpen(e, 'launch'); }}
                title="Click to open file"
            >
                <p className="text-sm font-bold text-white truncate">{file.name}</p>
                <p className="text-[10px] text-gray-400 truncate opacity-80">{formatBytes(file.size)}</p>
            </div>

            {/* Hover overlay for action buttons only */}
            <div className="absolute inset-0 bg-black/20 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none">
                <div className="absolute top-3 right-3 flex gap-2 pointer-events-auto">
                    <button
                        onClick={(e) => handleOpen(e, 'reveal')}
                        className="p-2 bg-blue-500/90 hover:bg-blue-500 rounded-lg text-white transition-all shadow-lg"
                        title="Reveal in Folder"
                    >
                        <Folder className="w-4 h-4" />
                    </button>
                    <button
                        onClick={(e) => { e.stopPropagation(); setShowConfirm(true); }}
                        disabled={isDeleting}
                        className={`p-2 bg-red-500/90 hover:bg-red-500 rounded-lg text-white transition-all shadow-lg ${isDeleting ? 'opacity-50 cursor-wait' : ''}`}
                        title="Delete/Trash File"
                    >
                        {isDeleting ? (
                            <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                            <Trash2 className="w-4 h-4" />
                        )}
                    </button>
                </div>
            </div>

            {/* Delete confirmation */}
            <AnimatePresence>
                {showConfirm && (
                    <motion.div
                        initial={{ opacity: 0, scale: 0.9 }}
                        animate={{ opacity: 1, scale: 1 }}
                        exit={{ opacity: 0, scale: 0.9 }}
                        className="absolute inset-0 bg-gray-900/95 z-50 flex flex-col items-center justify-center p-4"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <AlertTriangle className="w-12 h-12 text-red-500 mb-3" />
                        <span className="text-sm font-bold text-white text-center mb-1">Move to trash?</span>
                        <span className="text-xs text-gray-400 truncate max-w-full mb-4">{file.name}</span>
                        <div className="flex gap-2">
                            <button
                                onClick={() => setShowConfirm(false)}
                                className="px-4 py-2 bg-white/10 hover:bg-white/20 rounded-lg text-xs font-bold text-white transition-all uppercase tracking-wider"
                            >
                                No
                            </button>
                            <button
                                onClick={() => handleDelete()}
                                className="px-4 py-2 bg-red-500 hover:bg-red-600 rounded-lg text-xs font-bold text-white transition-all uppercase tracking-wider shadow-lg shadow-red-500/20"
                            >
                                Yes
                            </button>
                        </div>
                    </motion.div>
                )}
            </AnimatePresence>
        </div>
    )
}

function GlobalViewer({ files, selectedIndex, onClose, onPrev, onNext }: { files: FileInfo[], selectedIndex: number, onClose: () => void, onPrev: () => void, onNext: () => void }) {
    const file = files[selectedIndex]
    const [previewData, setPreviewData] = useState<{ url: string, type: 'image' | 'model' | 'video', internalPath: string } | null>(null)
    const [loading, setLoading] = useState(true)
    const [internalPreviews, setInternalPreviews] = useState<{ path: string, size: number }[]>([])
    const [internalIndex, setInternalIndex] = useState(0)

    const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''

    // Fetch list of previews in this archive
    useEffect(() => {
        fetch(`${apiHost}/api/list-previews?path=${encodeURIComponent(file.path)}`)
            .then(res => res.json())
            .then(data => {
                if (data.previews && data.previews.length > 0) {
                    setInternalPreviews(data.previews)
                    // The default preview might be any of these, let's start with index 0
                    // unless we want to find the one that FindPreviewInArchive returns.
                    // For simplicity, let's just use the first internal preview or the logic below.
                }
            })
            .catch(err => console.error("Failed to list previews:", err))
    }, [file.path])

    // Fetch the 0-th image when the archive changes
    useEffect(() => {
        setInternalIndex(0)
    }, [file.path])

    useEffect(() => {
        const path = internalPreviews.length > 0 ? internalPreviews[internalIndex].path : ''
        const urlParam = path ? `&internal_path=${encodeURIComponent(path)}` : ''
        const url = `${apiHost}/api/preview?path=${encodeURIComponent(file.path)}${urlParam}`

        // For internal videos, we also prefer direct URL to support Range requests
        const isVideo = (path || file.path).toLowerCase().match(/\.(mp4|webm|mkv|mov|avi)$/)
        if (isVideo) {
            setPreviewData({ url, type: 'video', internalPath: path })
            setLoading(false)
            return
        }

        setLoading(true)
        fetch(url)
            .then(res => {
                if (!res.ok) throw new Error('No preview')
                const contentType = res.headers.get('content-type') || ''
                const actualInternalPath = res.headers.get('X-Internal-Path') || path
                return res.blob().then(blob => ({ blob, contentType, actualInternalPath }))
            })
            .then(({ blob, contentType, actualInternalPath }) => {
                const objectUrl = URL.createObjectURL(blob)
                const isVideoExt = (actualInternalPath || file.path).toLowerCase().match(/\.(mp4|webm|mkv|mov|avi)$/)
                let type: 'image' | 'model' | 'video' = isVideoExt ? 'video' : 'image'
                if (contentType.startsWith('model/')) type = 'model'
                else if (contentType.startsWith('video/')) type = 'video'

                setPreviewData({ url: type === 'video' ? url : objectUrl, type, internalPath: actualInternalPath })
                if (type === 'video') URL.revokeObjectURL(objectUrl)

                // If we didn't have the list yet, we might find out which index we are at
                if (internalPreviews.length > 0 && actualInternalPath) {
                    const idx = internalPreviews.findIndex(p => p.path === actualInternalPath)
                    if (idx !== -1 && idx !== internalIndex) {
                        setInternalIndex(idx)
                    }
                }
                setLoading(false)
            })
            .catch(() => {
                setLoading(false)
            })

        return () => {
            if (previewData?.url) {
                URL.revokeObjectURL(previewData.url)
            }
        }
    }, [file.path, internalIndex, internalPreviews.length > 0])

    const handleOpenOriginal = (e: React.MouseEvent) => {
        e.stopPropagation()
        const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
        fetch(`${apiHost}/api/open?path=${encodeURIComponent(file.path)}&mode=launch`)
    }

    const [showInternalList, setShowInternalList] = useState(false)

    return (
        <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/95 z-[100] flex items-center justify-center p-4"
            onClick={() => { if (previewData?.type === 'image') onClose(); }}
        >
            {/* Internal File Selector Modal */}
            <AnimatePresence>
                {showInternalList && (
                    <motion.div
                        initial={{ opacity: 0, scale: 0.9, y: 20 }}
                        animate={{ opacity: 1, scale: 1, y: 0 }}
                        exit={{ opacity: 0, scale: 0.9, y: 20 }}
                        className="absolute bottom-32 left-1/2 -translate-x-1/2 w-full max-w-lg bg-[#111114]/90 backdrop-blur-2xl border border-white/10 rounded-3xl z-[150] shadow-2xl p-6 overflow-hidden"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <div className="flex items-center justify-between mb-4 px-2">
                            <h3 className="text-xs font-black uppercase tracking-[0.2em] text-blue-400">Select Internal File</h3>
                            <button
                                onClick={() => setShowInternalList(false)}
                                className="p-2 hover:bg-white/10 rounded-full text-gray-400 transition-all"
                            >
                                <X className="w-4 h-4" />
                            </button>
                        </div>
                        <div className="max-h-[40vh] overflow-y-auto space-y-1 pr-2 custom-scrollbar">
                            {internalPreviews.map((p, idx) => (
                                <button
                                    key={p.path}
                                    onClick={() => {
                                        setInternalIndex(idx)
                                        setShowInternalList(false)
                                    }}
                                    className={`w-full flex items-center gap-3 p-3 rounded-xl transition-all text-left ${internalIndex === idx ? 'bg-blue-600 text-white shadow-lg shadow-blue-600/20' : 'hover:bg-white/5 text-gray-400 hover:text-white'}`}
                                >
                                    <div className={`p-1.5 rounded-lg ${internalIndex === idx ? 'bg-white/20' : 'bg-white/5'}`}>
                                        {p.path.toLowerCase().endsWith('.stl') || p.path.toLowerCase().endsWith('.obj') ? <Box className="w-3.5 h-3.5" /> :
                                            p.path.toLowerCase().match(/\.(mp4|webm|mkv|mov|avi)$/) ? <Play className="w-3.5 h-3.5" /> :
                                                <ImageIcon className="w-3.5 h-3.5" />}
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <p className="text-[11px] font-bold truncate">{p.path}</p>
                                        <p className="text-[9px] opacity-60 font-mono">{formatBytes(p.size)}</p>
                                    </div>
                                    {internalIndex === idx && <div className="w-1.5 h-1.5 rounded-full bg-white animate-pulse" />}
                                </button>
                            ))}
                        </div>
                    </motion.div>
                )}
            </AnimatePresence>

            {/* Navigation Layer - Higher z-index to stay on top */}
            <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-[120]">
                <div className="w-full max-w-[1250px] flex justify-between px-4 md:px-8 pointer-events-auto">
                    <button
                        onClick={(e) => { e.stopPropagation(); onPrev(); }}
                        className="p-5 bg-black/40 hover:bg-blue-600/60 rounded-full text-white backdrop-blur-xl transition-all active:scale-90 border border-white/10 shadow-2xl group"
                        title="Previous (Arrow Left)"
                    >
                        <ChevronLeft className="w-10 h-10 group-hover:-translate-x-1 transition-transform" />
                    </button>
                    <button
                        onClick={(e) => { e.stopPropagation(); onNext(); }}
                        className="p-5 bg-black/40 hover:bg-blue-600/60 rounded-full text-white backdrop-blur-xl transition-all active:scale-90 border border-white/10 shadow-2xl group"
                        title="Next (Arrow Right)"
                    >
                        <ChevronRight className="w-10 h-10 group-hover:translate-x-1 transition-transform" />
                    </button>
                </div>
            </div>

            <button
                className="absolute top-6 right-6 p-4 bg-black/40 hover:bg-red-500/60 rounded-full text-white z-[130] backdrop-blur-xl border border-white/10 transition-all active:scale-90"
                onClick={onClose}
                title="Close (Esc)"
            >
                <X className="w-6 h-6" />
            </button>

            <motion.div
                initial={{ scale: 0.9, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                key={file.path}
                className="relative w-full max-w-6xl h-[85vh] flex flex-col items-center justify-center pointer-events-none z-[110]"
            >
                {loading ? (
                    <Loader2 className="w-12 h-12 text-blue-500 animate-spin" />
                ) : previewData?.type === 'model' ? (
                    <div
                        className="w-full h-full pointer-events-auto bg-gray-900 rounded-3xl overflow-hidden border border-white/10 shadow-2xl"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <STLViewer url={previewData.url} />
                    </div>
                ) : previewData?.type === 'video' ? (
                    <div
                        className="w-full h-full pointer-events-auto bg-black rounded-3xl overflow-hidden border border-white/10 shadow-2xl flex items-center justify-center"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <video
                            src={previewData.url}
                            controls
                            autoPlay
                            muted
                            playsInline
                            className="max-w-full max-h-full"
                        />
                    </div>
                ) : (
                    <img
                        src={previewData?.url}
                        alt={file.name}
                        className="max-w-full max-h-full object-contain shadow-2xl rounded-lg cursor-pointer pointer-events-auto"
                        onClick={onClose}
                    />
                )}

                {/* Floating Info Overlay */}
                <div className="absolute bottom-6 left-1/2 -translate-x-1/2 p-4 bg-black/60 backdrop-blur-md border border-white/10 rounded-2xl flex flex-col items-center gap-3 shadow-2xl pointer-events-auto min-w-[400px]">
                    <div className="flex items-center gap-6 w-full justify-between">
                        <div className="flex-1 min-w-0">
                            <p className="text-sm font-bold text-white truncate">{file.name}</p>
                            <p className="text-[10px] text-blue-400 font-mono truncate mt-0.5">{previewData?.internalPath}</p>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-[10px] font-bold text-gray-400 uppercase tracking-tighter mr-2">
                                {formatBytes(file.size)}
                            </span>
                            <button
                                onClick={handleOpenOriginal}
                                className="p-2 bg-blue-500 hover:bg-blue-400 rounded-xl text-white transition-all shadow-lg"
                                title="Open original file"
                            >
                                <ExternalLink className="w-4 h-4" />
                            </button>
                        </div>
                    </div>

                    {internalPreviews.length > 1 && (
                        <div className="flex items-center gap-4 pt-2 border-t border-white/5 w-full justify-center">
                            <button
                                onClick={(e) => { e.stopPropagation(); setInternalIndex(i => (i - 1 + internalPreviews.length) % internalPreviews.length) }}
                                className="p-1.5 hover:bg-white/10 rounded-lg text-gray-400 hover:text-white transition-all"
                            >
                                <ChevronLeft className="w-4 h-4" />
                            </button>
                            <button
                                onClick={(e) => { e.stopPropagation(); setShowInternalList(!showInternalList); }}
                                className={`flex items-center gap-2 px-3 py-1 rounded-xl transition-all ${showInternalList ? 'bg-blue-600 text-white shadow-lg shadow-blue-600/20' : 'hover:bg-white/10 text-gray-300'}`}
                            >
                                <Images className="w-3.5 h-3.5 text-blue-400" />
                                <span className="text-[10px] font-bold uppercase tracking-tighter">
                                    {internalIndex + 1} / {internalPreviews.length} INTERNAL FILES
                                </span>
                            </button>
                            <button
                                onClick={(e) => { e.stopPropagation(); setInternalIndex(i => (i + 1) % internalPreviews.length) }}
                                className="p-1.5 hover:bg-white/10 rounded-lg text-gray-400 hover:text-white transition-all"
                            >
                                <ChevronRight className="w-4 h-4" />
                            </button>
                        </div>
                    )}
                </div>
            </motion.div>
        </motion.div>
    )
}

export default function GalleryPage() {
    const [files, setFiles] = useState<FileInfo[]>([])
    const [filteredFiles, setFilteredFiles] = useState<FileInfo[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [searchQuery, setSearchQuery] = useState('')
    const [gridHeight, setGridHeight] = useState(0)
    const [selectedIndex, setSelectedIndex] = useState<number | null>(null)
    const [sortField, setSortField] = useState<'name' | 'size' | 'mod_time'>('name')
    const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc')
    const [filterExt, setFilterExt] = useState<'all' | 'zip' | 'rar' | '7z' | 'stl' | 'obj'>('all')
    const [page, setPage] = useState(1)
    const PAGE_SIZE = 60

    // Calculate grid height to fit exactly 3 rows
    useEffect(() => {
        const calculateGridHeight = () => {
            const headerHeight = 200 // Approximate header + search bar height
            const availableHeight = window.innerHeight - headerHeight - 64 // 64px for padding
            const rowHeight = availableHeight / 3
            setGridHeight(rowHeight)
        }

        calculateGridHeight()
        window.addEventListener('resize', calculateGridHeight)
        return () => window.removeEventListener('resize', calculateGridHeight)
    }, [])

    const fetchFiles = useCallback(async () => {
        try {
            const apiHost = window.location.port === '3000' ? 'http://localhost:8080' : ''
            const response = await fetch(`${apiHost}/api/all-files`)
            if (!response.ok) throw new Error(`HTTP ${response.status}: ${response.statusText}`)

            const data: GalleryResponse = await response.json()
            setFiles(data.files)
            setFilteredFiles(data.files)
            setLoading(false)
        } catch (err) {
            console.error("❌ Fetch error:", err)
            setError(err instanceof Error ? err.message : String(err))
            setLoading(false)
        }
    }, [])

    useEffect(() => {
        fetchFiles()
    }, [fetchFiles])

    // Filter and Sort files
    useEffect(() => {
        let result = [...files]

        // 1. Filter by Search Query
        if (searchQuery.trim() !== '') {
            const query = searchQuery.toLowerCase()
            result = result.filter(file =>
                file.name.toLowerCase().includes(query) ||
                file.path.toLowerCase().includes(query)
            )
        }

        // 2. Filter by Extension
        if (filterExt !== 'all') {
            result = result.filter(file =>
                file.name.toLowerCase().endsWith('.' + filterExt)
            )
        }

        // 3. Sort
        result.sort((a, b) => {
            let valA = a[sortField]
            let valB = b[sortField]

            if (sortField === 'name') {
                valA = (valA as string).toLowerCase()
                valB = (valB as string).toLowerCase()
            } else if (sortField === 'mod_time') {
                valA = new Date(valA as string).getTime()
                valB = new Date(valB as string).getTime()
            }

            if (valA < valB) return sortOrder === 'asc' ? -1 : 1
            if (valA > valB) return sortOrder === 'asc' ? 1 : -1
            return 0
        })

        setFilteredFiles(result)
        setPage(1) // Reset to first page when filters change
    }, [searchQuery, filterExt, sortField, sortOrder, files])

    const handlePrev = () => {
        if (selectedIndex === null) return
        setSelectedIndex(selectedIndex > 0 ? selectedIndex - 1 : filteredFiles.length - 1)
    }

    const handleNext = () => {
        if (selectedIndex === null) return
        setSelectedIndex(selectedIndex < filteredFiles.length - 1 ? selectedIndex + 1 : 0)
    }

    // Keyboard navigation
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if (selectedIndex === null) return
            if (e.key === 'ArrowLeft') handlePrev()
            if (e.key === 'ArrowRight') handleNext()
            if (e.key === 'Escape') setSelectedIndex(null)
        }
        window.addEventListener('keydown', handleKeyDown)
        return () => window.removeEventListener('keydown', handleKeyDown)
    }, [selectedIndex, filteredFiles.length])

    if (loading) return (
        <div className="flex flex-col items-center justify-center min-h-screen bg-[#0a0a0c] text-white">
            <motion.div
                animate={{ rotate: 360 }}
                transition={{ repeat: Infinity, duration: 2, ease: "linear" }}
            >
                <Grid3x3 className="w-12 h-12 text-blue-500" />
            </motion.div>
            <p className="mt-4 text-gray-400 animate-pulse font-light tracking-widest uppercase">Loading Gallery...</p>
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

    return (
        <div className="min-h-screen bg-[#0a0a0c] text-slate-200 p-4 md:p-8">
            <div className="max-w-[1000px] mx-auto">
                <header className="flex flex-col md:flex-row justify-between items-start md:items-center mb-8 gap-4">
                    <div className="flex items-center gap-4">
                        <Link href="/">
                            <button className="p-3 bg-white/5 hover:bg-white/10 rounded-xl transition-all border border-white/10">
                                <ArrowLeft className="w-5 h-5" />
                            </button>
                        </Link>
                        <div>
                            <h1 className="text-4xl font-black tracking-tight flex items-center gap-3">
                                <span className="bg-gradient-to-r from-blue-500 to-cyan-400 bg-clip-text text-transparent">GALLERY</span>
                                <span className="text-white">VIEW</span>
                            </h1>
                            <p className="text-gray-500 mt-1 font-medium tracking-wide">Browse all archive previews</p>
                        </div>
                    </div>
                </header>

                <div className="mb-8 flex flex-col md:flex-row gap-6 items-end md:items-center">
                    <div className="relative group flex-1 w-full">
                        <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-500 group-focus-within:text-blue-500 transition-colors" />
                        <input
                            type="text"
                            placeholder="Search files by name..."
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            className="w-full bg-white/5 border border-white/5 rounded-2xl py-4 pl-12 pr-4 text-sm font-medium focus:outline-none focus:border-blue-500/50 focus:bg-white/[0.08] transition-all"
                        />
                    </div>

                    <div className="flex items-center gap-2 bg-white/5 rounded-2xl p-1.5 border border-white/10 self-stretch md:self-auto overflow-x-auto">
                        {['all', 'zip', 'rar', '7z', 'stl', 'obj'].map(ext => (
                            <button
                                key={ext}
                                onClick={() => setFilterExt(ext as any)}
                                className={`px-4 py-2 rounded-xl text-[10px] font-black uppercase tracking-wider transition-all whitespace-nowrap ${filterExt === ext ? 'bg-blue-600 text-white shadow-lg shadow-blue-600/20' : 'text-gray-500 hover:text-white'}`}
                            >
                                {ext}
                            </button>
                        ))}
                    </div>
                </div>

                <div className="flex items-center justify-between mb-8 px-2">
                    <div className="flex items-center gap-2">
                        <ArrowDownWideNarrow className="w-4 h-4 text-gray-500" />
                        <span className="text-[10px] font-bold text-gray-500 uppercase tracking-widest">Sort by:</span>
                        <div className="flex gap-2 ml-2">
                            {[
                                { id: 'name', icon: sortOrder === 'asc' ? ArrowUpAZ : ArrowDownAZ, label: 'Name' },
                                { id: 'size', icon: sortOrder === 'asc' ? ArrowUp01 : ArrowDown01, label: 'Size' },
                                { id: 'mod_time', icon: Clock, label: 'Date' }
                            ].map(item => (
                                <button
                                    key={item.id}
                                    onClick={() => {
                                        if (sortField === item.id) {
                                            setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc')
                                        } else {
                                            setSortField(item.id as any)
                                            setSortOrder('asc')
                                        }
                                    }}
                                    className={`flex items-center gap-2 px-3 py-1.5 rounded-lg transition-all border ${sortField === item.id ? 'bg-blue-600/10 border-blue-500 text-blue-400' : 'bg-transparent border-white/5 text-gray-500 hover:bg-white/5'}`}
                                >
                                    <item.icon className="w-3 h-3" />
                                    <span className="text-[10px] font-bold uppercase tracking-tighter">{item.label}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    <div className="text-[10px] font-bold text-gray-500 uppercase tracking-widest">
                        Showing <span className="text-white">{filteredFiles.length}</span> / {files.length} archives
                    </div>
                </div>

                <div
                    className="grid grid-cols-3 gap-6"
                    style={{
                        gridAutoRows: gridHeight > 0 ? `${gridHeight - 32}px` : 'auto'
                    }}
                >
                    {filteredFiles.slice(0, page * PAGE_SIZE).map((file, idx) => (
                        <GalleryItem
                            key={file.path}
                            file={file}
                            index={idx}
                            onRefresh={fetchFiles}
                            onSelect={setSelectedIndex}
                        />
                    ))}
                </div>

                {filteredFiles.length > page * PAGE_SIZE && (
                    <div className="mt-12 mb-20 flex justify-center">
                        <button
                            onClick={() => setPage(p => p + 1)}
                            className="px-8 py-4 bg-white/5 hover:bg-blue-600 border border-white/10 rounded-2xl text-sm font-black uppercase tracking-widest transition-all shadow-xl hover:shadow-blue-600/20 group flex items-center gap-3"
                        >
                            <span>Load More Archives</span>
                            <span className="text-[10px] text-gray-500 group-hover:text-blue-200">
                                ({filteredFiles.length - page * PAGE_SIZE} remaining)
                            </span>
                        </button>
                    </div>
                )}

                <AnimatePresence>
                    {selectedIndex !== null && (
                        <GlobalViewer
                            files={filteredFiles}
                            selectedIndex={selectedIndex}
                            onClose={() => setSelectedIndex(null)}
                            onPrev={handlePrev}
                            onNext={handleNext}
                        />
                    )}
                </AnimatePresence>
            </div>
        </div>
    )
}
