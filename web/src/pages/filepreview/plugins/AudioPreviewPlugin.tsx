import React, { useState, useRef, useEffect, useCallback } from "react";
import type { FilePreviewPluginProps, PluginDefinition } from "../PluginDefinition";
import { constructRawContentUrl } from "@/lib/utils";
import { Play, Pause, Volume2, VolumeX, AlertTriangle, Music } from "lucide-react";
import { Slider } from "@/components/ui/slider";
import { Button } from "@/components/ui/button";
import { PlaceholderView } from "@/components/custom/PlaceholderView";
import { cn } from "@/lib/utils";

const formatTime = (timeInSeconds: number) => {
    if (isNaN(timeInSeconds)) return "00:00";
    const m = Math.floor(timeInSeconds / 60).toString().padStart(2, "0");
    const s = Math.floor(timeInSeconds % 60).toString().padStart(2, "0");
    return `${m}:${s}`;
};

const AudioPreviewPluginComponent: React.FC<FilePreviewPluginProps> = ({ info, onLoaded }) => {
    const audioRef = useRef<HTMLAudioElement>(null);
    const [isPlaying, setIsPlaying] = useState(false);
    const [progress, setProgress] = useState(0);
    const [duration, setDuration] = useState(0);
    const [volume, setVolume] = useState(1);
    const [isMuted, setIsMuted] = useState(false);
    const [showControls, setShowControls] = useState(true);
    const [hasError, setHasError] = useState(false);
    const controlsTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const isDraggingRef = useRef(false);

    const audioUrl = constructRawContentUrl(info.fileId);

    const togglePlay = useCallback(() => {
        if (audioRef.current) {
            if (audioRef.current.paused) {
                audioRef.current.play();
            } else {
                audioRef.current.pause();
            }
        }
    }, []);

    const handleVolumeChange = (value: number | readonly number[]) => {
        const newVolume = Array.isArray(value) ? value[0] : (value as number);
        setVolume(newVolume);
        if (audioRef.current) {
            audioRef.current.volume = newVolume;
            setIsMuted(newVolume === 0);
        }
    };

    const toggleMute = () => {
        if (audioRef.current) {
            const newMutedState = !isMuted;
            audioRef.current.muted = newMutedState;
            setIsMuted(newMutedState);
            if (newMutedState) setVolume(0);
            else setVolume(audioRef.current.volume || 1);
        }
    };

    const handleSeek = (value: number | readonly number[]) => {
        const newTime = Array.isArray(value) ? value[0] : (value as number);
        if (audioRef.current) {
            audioRef.current.currentTime = newTime;
            setProgress(newTime);
        }
    };

    const handleTimeUpdate = () => {
        if (audioRef.current) {
            setProgress(audioRef.current.currentTime);
        }
    };

    const handleLoadedMetadata = () => {
        if (audioRef.current) {
            setDuration(audioRef.current.duration);
            onLoaded();
        }
    };

    useEffect(() => {
        const handlePointerUp = () => {
            isDraggingRef.current = false;
        };
        window.addEventListener("pointerup", handlePointerUp);
        return () => {
            window.removeEventListener("pointerup", handlePointerUp);
        };
    }, []);

    const resetControlsTimeout = useCallback(() => {
        setShowControls(true);
        if (controlsTimeoutRef.current) clearTimeout(controlsTimeoutRef.current);
        controlsTimeoutRef.current = setTimeout(() => {
            if (isPlaying && !isDraggingRef.current) {
                setShowControls(false);
            }
        }, 2500);
    }, [isPlaying]);

    useEffect(() => {
        resetControlsTimeout();
        return () => {
            if (controlsTimeoutRef.current) clearTimeout(controlsTimeoutRef.current);
        };
    }, [resetControlsTimeout]);

    if (hasError) {
        return (
            <PlaceholderView
                icon={AlertTriangle}
                title="Failed to Load Audio"
                description={`Unable to play ${info.fileName}. The file may be corrupted or in an unsupported audio format.`}
                variant="warning"
            />
        );
    }

    return (
        <div
            className="relative flex flex-col items-center justify-center w-full h-full bg-black overflow-hidden group"
            onMouseMove={resetControlsTimeout}
            onMouseLeave={() => {
                if (isPlaying && !isDraggingRef.current) setShowControls(false);
            }}
        >
            <audio
                autoPlay
                ref={audioRef}
                src={audioUrl}
                className="hidden"
                onTimeUpdate={handleTimeUpdate}
                onLoadedMetadata={handleLoadedMetadata}
                onPlay={() => setIsPlaying(true)}
                onPause={() => setIsPlaying(false)}
                onError={() => {
                    setHasError(true);
                    onLoaded();
                }}
                onEnded={() => setIsPlaying(false)}
            />

            {/* Audio Icon / Visualizer Placeholder */}
            <div 
                className="w-full h-full flex items-center justify-center cursor-pointer"
                onClick={togglePlay}
            >
                <div className={cn(
                    "flex items-center justify-center w-48 h-48 rounded-full bg-white/5 border border-white/10 transition-all duration-700",
                    isPlaying ? "scale-110 shadow-[0_0_60px_rgba(255,255,255,0.1)]" : "scale-100"
                )}>
                    <Music className={cn(
                        "w-20 h-20 text-white/50 transition-all duration-500",
                        isPlaying && "animate-pulse text-white/80"
                    )} />
                </div>
            </div>

            {/* Play overlay animation */}
            <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                <div 
                    onClick={togglePlay}
                    className={cn(
                        "bg-background/20 backdrop-blur-md rounded-full p-6 shadow-xl border border-white/10 cursor-pointer transition-all duration-300",
                        !isPlaying && !hasError ? "opacity-100 scale-100 pointer-events-auto hover:bg-background/40 hover:scale-105" : "opacity-0 scale-95 pointer-events-none"
                    )}
                >
                    <Play className="w-16 h-16 text-white ml-2" />
                </div>
            </div>

            {/* Control Bar */}
            <div
                className={cn(
                    "absolute bottom-0 left-0 right-0 p-6 pt-16 bg-gradient-to-t from-black/80 via-black/40 to-transparent transition-opacity duration-300 flex flex-col gap-3",
                    showControls ? "opacity-100" : "opacity-0 pointer-events-none"
                )}
            >
                {/* Timeline */}
                <div className="flex items-center gap-3">
                    <span className="text-xs text-white/90 font-medium tabular-nums w-12 text-right">
                        {formatTime(progress)}
                    </span>
                    <Slider
                        value={[progress]}
                        max={duration || 100}
                        step={0.1}
                        onPointerDown={() => { isDraggingRef.current = true; }}
                        onValueChange={handleSeek}
                        className="flex-1 cursor-pointer"
                    />
                    <span className="text-xs text-white/90 font-medium tabular-nums w-12">
                        {formatTime(duration)}
                    </span>
                </div>

                {/* Controls */}
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <Button
                            variant="ghost"
                            size="icon"
                            onClick={togglePlay}
                            className="text-white hover:bg-white/20 hover:text-white"
                        >
                            {isPlaying ? <Pause className="w-5 h-5" /> : <Play className="w-5 h-5" />}
                        </Button>

                        <div className="flex items-center gap-2 ml-2 group/volume py-4 -my-4 px-2 -mx-2">
                            <Button
                                variant="ghost"
                                size="icon"
                                onClick={toggleMute}
                                className="text-white hover:bg-white/20 hover:text-white"
                            >
                                {isMuted || volume === 0 ? <VolumeX className="w-5 h-5" /> : <Volume2 className="w-5 h-5" />}
                            </Button>
                            <div className="w-0 overflow-hidden transition-all duration-300 group-hover/volume:w-24 flex items-center h-10">
                                <div className="w-24 px-2">
                                    <Slider
                                        value={[isMuted ? 0 : volume]}
                                        max={1}
                                        step={0.01}
                                        onPointerDown={() => { isDraggingRef.current = true; }}
                                        onValueChange={handleVolumeChange}
                                        className="w-full cursor-pointer"
                                    />
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

export const AudioPreviewPlugin: PluginDefinition = {
    id: "audio-previewer",
    name: "Audio Previewer",
    supportedMimeTypes: ["audio/*"],
    maxSizeBytes: 0, // unlimited stream
    component: AudioPreviewPluginComponent,
};

export default AudioPreviewPlugin;
