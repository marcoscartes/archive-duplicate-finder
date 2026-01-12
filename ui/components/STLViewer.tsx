"use client"

import { useEffect, useRef, useState } from 'react'
import { Canvas, useThree } from '@react-three/fiber'
import { OrbitControls, PerspectiveCamera, Environment } from '@react-three/drei'
import * as THREE from 'three'
import { STLLoader } from 'three/examples/jsm/loaders/STLLoader.js'

function Model({ url }: { url: string }) {
    const [geometry, setGeometry] = useState<THREE.BufferGeometry | null>(null)
    const [error, setError] = useState<string | null>(null)
    const meshRef = useRef<THREE.Mesh>(null)
    const { camera } = useThree()

    useEffect(() => {
        const loader = new STLLoader()

        // Load the STL file
        loader.load(
            url,
            (loadedGeometry) => {
                console.log('STL loaded successfully', loadedGeometry)

                // Center the geometry
                loadedGeometry.center()

                // Compute normals for proper lighting
                loadedGeometry.computeVertexNormals()

                // Compute bounding box
                loadedGeometry.computeBoundingBox()
                const boundingBox = loadedGeometry.boundingBox

                if (boundingBox) {
                    const size = new THREE.Vector3()
                    boundingBox.getSize(size)
                    const maxDim = Math.max(size.x, size.y, size.z)

                    // Position camera based on model size
                    const distance = maxDim * 2
                    camera.position.set(distance, distance, distance)
                    camera.lookAt(0, 0, 0)

                    console.log('Model size:', size, 'Max dimension:', maxDim)
                }

                setGeometry(loadedGeometry)
            },
            (progress) => {
                console.log('Loading progress:', (progress.loaded / progress.total * 100).toFixed(2) + '%')
            },
            (err) => {
                console.error('Error loading STL:', err)
                setError('Failed to load 3D model')
            }
        )

        return () => {
            if (geometry) {
                geometry.dispose()
            }
        }
    }, [url, camera])

    if (error) {
        return (
            <mesh>
                <boxGeometry args={[1, 1, 1]} />
                <meshStandardMaterial color="red" />
            </mesh>
        )
    }

    if (!geometry) {
        return (
            <mesh>
                <boxGeometry args={[0.5, 0.5, 0.5]} />
                <meshStandardMaterial color="#60a5fa" wireframe />
            </mesh>
        )
    }

    return (
        <mesh ref={meshRef} geometry={geometry} scale={0.1}>
            <meshStandardMaterial
                color="#60a5fa"
                metalness={0.3}
                roughness={0.4}
                side={THREE.DoubleSide}
            />
        </mesh>
    )
}

export default function STLViewer({ url }: { url: string }) {
    const [modelUrl, setModelUrl] = useState<string>('')

    useEffect(() => {
        // If the URL is a blob, use it directly
        // Otherwise, fetch it and create a blob URL
        if (url.startsWith('blob:')) {
            setModelUrl(url)
        } else {
            fetch(url)
                .then(res => res.blob())
                .then(blob => {
                    const blobUrl = URL.createObjectURL(blob)
                    setModelUrl(blobUrl)
                    return blobUrl
                })
                .catch(err => {
                    console.error('Failed to fetch model:', err)
                })
        }

        return () => {
            if (modelUrl && modelUrl.startsWith('blob:')) {
                URL.revokeObjectURL(modelUrl)
            }
        }
    }, [url])

    if (!modelUrl) {
        return (
            <div className="w-full h-full flex items-center justify-center bg-gradient-to-br from-gray-900 to-black">
                <div className="text-white text-center">
                    <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4"></div>
                    <p>Loading 3D model...</p>
                </div>
            </div>
        )
    }

    return (
        <div className="w-full h-full bg-gradient-to-br from-gray-900 to-black">
            <Canvas shadows>
                <PerspectiveCamera makeDefault position={[3, 3, 3]} fov={50} />
                <OrbitControls
                    enablePan={true}
                    enableZoom={true}
                    enableRotate={true}
                    minDistance={0.5}
                    maxDistance={20}
                    target={[0, 0, 0]}
                />

                {/* Lighting */}
                <ambientLight intensity={0.6} />
                <directionalLight
                    position={[10, 10, 5]}
                    intensity={1}
                    castShadow
                    shadow-mapSize-width={2048}
                    shadow-mapSize-height={2048}
                />
                <directionalLight position={[-10, -10, -5]} intensity={0.5} />
                <pointLight position={[0, 10, 0]} intensity={0.5} />
                <spotLight position={[5, 5, 5]} intensity={0.3} />

                {/* Grid helper */}
                <gridHelper args={[20, 20, '#444444', '#222222']} position={[0, -0.01, 0]} />

                {/* Model */}
                <Model url={modelUrl} />

                {/* Environment for better reflections */}
                <Environment preset="city" />
            </Canvas>

            {/* Controls hint */}
            <div className="absolute bottom-4 left-4 bg-black/70 backdrop-blur-sm rounded-lg p-3 text-xs text-gray-300 border border-white/10">
                <p className="font-bold mb-2 text-blue-400">Controls:</p>
                <p className="mb-1">• <span className="text-white">Left click + drag:</span> Rotate</p>
                <p className="mb-1">• <span className="text-white">Right click + drag:</span> Pan</p>
                <p>• <span className="text-white">Scroll:</span> Zoom</p>
            </div>

            {/* Model info */}
            <div className="absolute bottom-4 right-4 bg-black/70 backdrop-blur-sm rounded-lg p-3 text-xs text-gray-300 border border-white/10">
                <p className="font-bold text-blue-400">Model Info</p>
                <p className="text-[10px] mt-1 opacity-60">STL Format</p>
            </div>
        </div>
    )
}
