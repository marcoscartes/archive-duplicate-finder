"use client"

import { Canvas, useFrame } from '@react-three/fiber'
import { OrbitControls, Float, MeshDistortMaterial, Sphere } from '@react-three/drei'
import { useRef } from 'react'
import * as THREE from 'three'

function ComparisonScene() {
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
        <>
            <ambientLight intensity={0.5} />
            <pointLight position={[10, 10, 10]} intensity={1} color="#3b82f6" />
            <pointLight position={[-10, -10, -10]} intensity={0.5} color="#06b6d4" />

            {/* Model A (Overlay 1) */}
            <mesh ref={mesh1} position={[-2, 0, 0]}>
                <octahedronGeometry args={[1.5, 2]} />
                <meshStandardMaterial
                    color="#3b82f6"
                    wireframe
                    transparent
                    opacity={0.4}
                />
            </mesh>

            {/* Model B (Overlay 2 - shifted color) */}
            <mesh ref={mesh2} position={[2, 0, 0]}>
                <octahedronGeometry args={[1.5, 2]} />
                <meshStandardMaterial
                    color="#06b6d4"
                    wireframe
                    transparent
                    opacity={0.4}
                />
            </mesh>

            <Float speed={2} rotationIntensity={1} floatIntensity={1}>
                <Sphere args={[0.2, 16, 16]} position={[0, -2, 0]}>
                    <MeshDistortMaterial
                        color="#3b82f6"
                        speed={2}
                        distort={0.4}
                        radius={1}
                    />
                </Sphere>
            </Float>

            <OrbitControls enableZoom={false} makeDefault />
        </>
    )
}

export default function ModelPreview() {
    return (
        <div className="w-full h-[400px] glass-card rounded-3xl overflow-hidden relative">
            <div className="absolute top-4 left-4 z-10 flex flex-col gap-1">
                <span className="text-[10px] font-black uppercase tracking-widest text-blue-500">Geometry AI Comparison</span>
                <span className="text-xs font-bold text-gray-400">Overlay Analysis (Stereo Mode)</span>
            </div>
            <Canvas camera={{ position: [0, 0, 5], fov: 45 }}>
                <ComparisonScene />
            </Canvas>
            <div className="absolute bottom-4 right-4 z-10 flex gap-4">
                <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full bg-blue-500 shadow-[0_0_8px_rgba(59,130,246,0.5)]" />
                    <span className="text-[9px] font-bold text-gray-500 uppercase tracking-tighter">Model A</span>
                </div>
                <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full bg-cyan-500 shadow-[0_0_8px_rgba(6,182,212,0.5)]" />
                    <span className="text-[9px] font-bold text-gray-500 uppercase tracking-tighter">Model B</span>
                </div>
            </div>
        </div>
    )
}
