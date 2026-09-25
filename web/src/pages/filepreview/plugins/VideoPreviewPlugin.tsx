import React, { useState, useRef, useEffect, useCallback } from "react";
import type { FilePreviewPluginProps, PluginDefinition } from "../PluginDefinition";
import { constructRawContentUrl } from "@/lib/utils";
import { Play, Pause, Volume2, VolumeX, Maximize, Minimize, AlertTriangle } from "lucide-react";
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

const VideoPreviewPluginComponent: React.FC<FilePreviewPluginProps> = ({ info, onLoaded }) => {
    const videoRef = useRef<HTMLVideoElement>(null);
    const containerRef = useRef<HTMLDivElement>(null);
    const [isPlaying, setIsPlaying] = useState(false);
    const [progress, setProgress] = useState(0);
    const [duration, setDuration] = useState(0);
    const [volume, setVolume] = useState(1);
    const [isMuted, setIsMuted] = useState(false);
    const [isFullscreen, setIsFullscreen] = useState(false);
    const [showControls, setShowControls] = useState(true);
    const [hasError, setHasError] = useState(false);
    const controlsTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const isDraggingRef = useRef(false);

    const videoUrl = constructRawContentUrl(info.fileId);

    const togglePlay = useCallback(() => {
        if (videoRef.current) {
            if (videoRef.current.paused) {
                videoRef.current.play();
            } else {
                videoRef.current.pause();
            }
        }
    }, []);

    const handleVolumeChange = (value: number | readonly number[]) => {
        const newVolume = Array.isArray(value) ? value[0] : (value as number);
        setVolume(newVolume);
        if (videoRef.current) {
            videoRef.current.volume = newVolume;
            setIsMuted(newVolume === 0);
        }
    };

    const toggleMute = () => {
        if (videoRef.current) {
            const newMutedState = !isMuted;
            videoRef.current.muted = newMutedState;
            setIsMuted(newMutedState);
            if (newMutedState) setVolume(0);
            else setVolume(videoRef.current.volume || 1);
        }
    };

    const handleSeek = (value: number | readonly number[]) => {
        const newTime = Array.isArray(value) ? value[0] : (value as number);
        if (videoRef.current) {
            videoRef.current.currentTime = newTime;
            setProgress(newTime);
        }
    };

    const handleTimeUpdate = () => {
        if (videoRef.current) {
            setProgress(videoRef.current.currentTime);
        }
    };

    const handleLoadedMetadata = () => {
        if (videoRef.current) {
            setDuration(videoRef.current.duration);
            onLoaded();
        }
    };

    const toggleFullscreen = () => {
        if (!containerRef.current) return;
        if (!document.fullscreenElement) {
            containerRef.current.requestFullscreen().catch((err) => {
                console.error(`Error attempting to enable fullscreen: ${err.message}`);
            });
        } else {
            document.exitFullscreen();
        }
    };

    useEffect(() => {
        const handleFullscreenChange = () => {
            setIsFullscreen(!!document.fullscreenElement);
        };
        const handlePointerUp = () => {
            isDraggingRef.current = false;
        };
        document.addEventListener("fullscreenchange", handleFullscreenChange);
        window.addEventListener("pointerup", handlePointerUp);
        return () => {
            document.removeEventListener("fullscreenchange", handleFullscreenChange);
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
                title="Failed to Load Video"
                description={`Unable to play ${info.fileName}. The file may be corrupted or in an unsupported video format.`}
                variant="warning"
            />
        );
    }

    return (
        <div
            ref={containerRef}
            className="relative flex flex-col items-center justify-center w-full h-full bg-black overflow-hidden group"
            onMouseMove={resetControlsTimeout}
            onMouseLeave={() => {
                if (isPlaying && !isDraggingRef.current) setShowControls(false);
            }}
        >
            <video
                autoPlay
                ref={videoRef}
                src={videoUrl}
                className="w-full h-full object-contain cursor-pointer"
                onTimeUpdate={handleTimeUpdate}
                onLoadedMetadata={handleLoadedMetadata}
                onPlay={() => setIsPlaying(true)}
                onPause={() => setIsPlaying(false)}
                onClick={togglePlay}
                onError={() => {
                    setHasError(true);
                    onLoaded();
                }}
                onEnded={() => setIsPlaying(false)}
            />

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

                    <div className="flex items-center gap-2">
                        <Button
                            variant="ghost"
                            size="icon"
                            onClick={toggleFullscreen}
                            className="text-white hover:bg-white/20 hover:text-white"
                        >
                            {isFullscreen ? <Minimize className="w-5 h-5" /> : <Maximize className="w-5 h-5" />}
                        </Button>
                    </div>
                </div>
            </div>
        </div>
    );
};

export const VideoPreviewPlugin: PluginDefinition = {
    id: "video-previewer",
    name: "Video Previewer",
    supportedMimeTypes: ["video/*"],
    maxSizeBytes: 0, // 0 = unlimited, because videos stream via range requests naturally
    component: VideoPreviewPluginComponent,
};

export default VideoPreviewPlugin;
