"use client"

import { Canvas, useLoader, useFrame } from '@react-three/fiber'
import { OrbitControls, Stage, Float, MeshDistortMaterial, Sphere, Center, Text, Html } from '@react-three/drei'
import { useRef, Suspense, useState, useEffect, useMemo } from 'react'
import * as THREE from 'three'
import { STLLoader, OBJLoader } from 'three-stdlib'
import { Maximize2, Minimize2 } from 'lucide-react'

function DemoScene() {
    const mesh1 = useRef<THREE.Mesh>(null!)
    const mesh2 = useRef<THREE.Mesh>(null!)

    useFrame((state) => {
        const t = state.clock.getElapsedTime()
        mesh1.current.rotation.y = t * 0.2
        mesh1.current.rotation.x = Math.sin(t * 0.5) * 0.2

        mesh2.current.rotation.y = t * 0.2
        mesh2.current.rotation.x = Math.sin(t * 0.5) * 0.2
    })

    return (
        <group>
            <mesh ref={mesh1} position={[-2, 0, 0]} castShadow receiveShadow>
                <octahedronGeometry args={[1.5, 2]} />
                <meshStandardMaterial color="#3b82f6" roughness={0.1} metalness={0.6} />
            </mesh>
            <mesh ref={mesh2} position={[2, 0, 0]} castShadow receiveShadow>
                <octahedronGeometry args={[1.5, 2]} />
                <meshStandardMaterial color="#06b6d4" roughness={0.1} metalness={0.6} />
            </mesh>
            <Float speed={2} rotationIntensity={1} floatIntensity={1}>
                <Sphere args={[0.2, 16, 16]} position={[0, -2, 0]}>
                    <MeshDistortMaterial color="#3b82f6" speed={2} distort={0.4} radius={1} />
                </Sphere>
            </Float>
        </group>
    )
}

function ModelViewer({ url, path, color, position }: { url: string, path: string, color: string, position: [number, number, number] }) {
    const isObj = path.toLowerCase().endsWith('.obj');
    const loaded = useLoader(isObj ? OBJLoader : STLLoader, url);
    const [scale, setScale] = useState(1);
    const [geometry, setGeometry] = useState<THREE.BufferGeometry | null>(null);

    useEffect(() => {
        if (!loaded) return;

        let geom: THREE.BufferGeometry | null = null;
        if (isObj) {
            const group = loaded as THREE.Group;
            group.traverse((child) => {
                if (child instanceof THREE.Mesh && !geom) {
                    geom = child.geometry;
                }
            });
        } else {
            geom = loaded as THREE.BufferGeometry;
        }

        if (geom) {
            geom.center();
            geom.computeBoundingBox();
            const box = geom.boundingBox;
            if (box) {
                const size = new THREE.Vector3();
                box.getSize(size);
                const maxDim = Math.max(size.x, size.y, size.z);
                if (maxDim > 0) {
                    setScale(3 / maxDim);
                }
            }
            geom.computeVertexNormals();
            setGeometry(geom);
        }
    }, [loaded, isObj]);

    if (!geometry) return null;

    return (
        <group position={position}>
            <mesh
                geometry={geometry}
                scale={[scale, scale, scale]}
                rotation={[-Math.PI / 2, 0, 0]}
                castShadow
                receiveShadow
            >
                <meshStandardMaterial
                    color={color}
                    roughness={0.2}
                    metalness={0.8}
                    envMapIntensity={1}
                />
            </mesh>
            {scale !== 1 && (
                <Html position={[0, -2, 0]} center>
                    <div className="bg-black/50 px-2 py-1 rounded text-[10px] text-white whitespace-nowrap backdrop-blur-sm">
                        Raw Scale: {(1 / scale).toFixed(2)}x
                    </div>
                </Html>
            )}
        </group>
    );
}

function ComparisonScene({ files }: { files: string[] }) {
    const colors = ["#3b82f6", "#06b6d4", "#a855f7", "#ef4444", "#22c55e"]
    const apiHost = typeof window !== 'undefined' && window.location.port === '3000' ? 'http://localhost:8080' : ''

    if (!files || files.length === 0) return (
        <Stage environment="city" intensity={0.5}>
            <DemoScene />
        </Stage>
    )

    // Layout calculation: Vertical stack (Top to Bottom)
    const spacing = 3.5
    const totalHeight = (files.length - 1) * spacing
    const startY = totalHeight / 2

    return (
        <Stage adjustCamera={1.2} intensity={0.5} environment="city" shadows="contact">
            {files.map((path, i) => (
                <ModelViewer
                    key={path}
                    path={path}
                    url={`${apiHost}/api/preview?path=${encodeURIComponent(path)}&type=model`}
                    color={colors[i % colors.length]}
                    position={[0, startY - (i * spacing), 0]}
                />
            ))}
        </Stage>
    )
}

function Loader() {
    return (
        <Html center>
            <div className="flex flex-col items-center gap-2">
                <div className="w-8 h-8 rounded-full border-2 border-blue-500 border-t-transparent animate-spin"></div>
                <span className="text-[10px] font-bold text-blue-400 uppercase tracking-widest">Loading...</span>
            </div>
        </Html>
    )
}

interface ModelPreviewProps {
    selectedFiles?: string[]
}

export default function ModelPreview({ selectedFiles = [] }: ModelPreviewProps) {
    const [isFullscreen, setIsFullscreen] = useState(false)
    const colors = ["bg-blue-500", "bg-cyan-500", "bg-purple-500", "bg-red-500", "bg-green-500"]

    useEffect(() => {
        const handleEsc = (e: KeyboardEvent) => {
            if (e.key === 'Escape') setIsFullscreen(false)
        }
        window.addEventListener('keydown', handleEsc)
        return () => window.removeEventListener('keydown', handleEsc)
    }, [])

    return (
        <div className={`transition-all duration-500 ease-in-out bg-gradient-to-b from-[#111115] to-[#0a0a0c] ${isFullscreen
            ? 'fixed inset-0 z-[100] w-screen h-screen'
            : 'w-full h-[600px] glass-card rounded-3xl overflow-hidden relative'
            }`}>
            <div className="absolute top-6 right-6 z-20">
                <button
                    onClick={() => setIsFullscreen(!isFullscreen)}
                    className="p-3 bg-white/5 hover:bg-white/10 rounded-xl text-white/70 hover:text-white transition-all backdrop-blur-md border border-white/10 hover:border-white/20 shadow-lg"
                    title={isFullscreen ? "Exit Fullscreen (Esc)" : "Enter Fullscreen"}
                >
                    {isFullscreen ? <Minimize2 className="w-5 h-5" /> : <Maximize2 className="w-5 h-5" />}
                </button>
            </div>

            <div className="absolute top-6 left-6 z-10 flex flex-col gap-1 pointer-events-none">
                <span className="text-[10px] font-black uppercase tracking-widest text-blue-500 shadow-black drop-shadow-md">Geometry AI Comparison</span>
                <span className="text-xs font-bold text-gray-400">
                    {selectedFiles.length > 0 ? `comparing ${selectedFiles.length} models` : 'Overlay Analysis (Demo Mode)'}
                </span>
            </div>

            <Canvas shadows camera={{ position: [5, 0, 10], fov: 45 }}>
                <Suspense fallback={<Loader />}>
                    <ComparisonScene files={selectedFiles} />
                </Suspense>
                <OrbitControls makeDefault minPolarAngle={0} maxPolarAngle={Math.PI / 1.5} />
            </Canvas>

            {selectedFiles.length > 0 && (
                <div className="absolute bottom-6 right-6 z-10 flex flex-col gap-2 items-end">
                    {selectedFiles.map((f, i) => (
                        <div key={f} className="flex items-center gap-2 bg-black/50 p-1.5 rounded-lg border border-white/10 backdrop-blur-md max-w-[200px]">
                            <div className={`w-2 h-2 rounded-full ${colors[i % colors.length]} shadow-[0_0_8px_rgba(255,255,255,0.3)] shrink-0`} />
                            <span className="text-[8px] font-bold text-gray-300 uppercase tracking-tighter truncate">{f.split(/[/\\]/).pop()}</span>
                        </div>
                    ))}
                </div>
            )}

            {selectedFiles.length === 0 && (
                <div className="absolute bottom-6 right-6 z-10 flex gap-4">
                    <div className="flex items-center gap-2">
                        <div className="w-2 h-2 rounded-full bg-blue-500 shadow-[0_0_8px_rgba(59,130,246,0.5)]" />
                        <span className="text-[9px] font-bold text-gray-500 uppercase tracking-tighter">Model A</span>
                    </div>
                    <div className="flex items-center gap-2">
                        <div className="w-2 h-2 rounded-full bg-cyan-500 shadow-[0_0_8px_rgba(6,182,212,0.5)]" />
                        <span className="text-[9px] font-bold text-gray-500 uppercase tracking-tighter">Model B</span>
                    </div>
                </div>
            )}
        </div>
    )
}
