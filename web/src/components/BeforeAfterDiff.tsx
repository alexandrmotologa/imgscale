import React, { useState, useRef, useEffect, useCallback } from 'react';
import { Columns, SplitSquareVertical, ZoomIn, ZoomOut, Maximize2, Zap, Database, HardDrive, RefreshCw } from 'lucide-react';
import { ImageTelemetry } from '../types';

interface Props {
  originalUrl: string;
  transformedUrl: string;
  telemetry: ImageTelemetry | null;
  loading: boolean;
  onRefresh: () => void;
}

export const BeforeAfterDiff: React.FC<Props> = ({
  originalUrl,
  transformedUrl,
  telemetry,
  loading,
  onRefresh,
}) => {
  const [sliderPos, setSliderPos] = useState(50);
  const [isDragging, setIsDragging] = useState(false);
  const [viewMode, setViewMode] = useState<'split' | 'side-by-side'>('split');
  const [zoom, setZoom] = useState<number>(1);
  const containerRef = useRef<HTMLDivElement>(null);

  const handleMove = useCallback((clientX: number) => {
    if (!containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    const x = clientX - rect.left;
    const pct = Math.max(0, Math.min(100, (x / rect.width) * 100));
    setSliderPos(pct);
  }, []);

  const handleTouchMove = useCallback((e: TouchEvent) => {
    if (isDragging && e.touches.length > 0) {
      handleMove(e.touches[0].clientX);
    }
  }, [isDragging, handleMove]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (isDragging) {
      handleMove(e.clientX);
    }
  }, [isDragging, handleMove]);

  const handleMouseUp = useCallback(() => {
    setIsDragging(false);
  }, []);

  useEffect(() => {
    if (isDragging) {
      window.addEventListener('mousemove', handleMouseMove);
      window.addEventListener('mouseup', handleMouseUp);
      window.addEventListener('touchmove', handleTouchMove);
      window.addEventListener('touchend', handleMouseUp);
    }
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
      window.removeEventListener('touchmove', handleTouchMove);
      window.removeEventListener('touchend', handleMouseUp);
    };
  }, [isDragging, handleMouseMove, handleMouseUp, handleTouchMove]);

  // Compute savings
  const origBytes = telemetry?.originalBytes || 0;
  const transBytes = telemetry?.transformedBytes || 0;
  const savingsPct = origBytes > 0 && transBytes > 0
    ? ((origBytes - transBytes) / origBytes) * 100
    : 0;

  const formatBytes = (bytes: number) => {
    if (bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const getCacheIcon = (source?: string) => {
    if (source === 'L1_MEMORY') return <Zap className="w-3.5 h-3.5 text-amber-400" />;
    if (source === 'L2_DISK') return <HardDrive className="w-3.5 h-3.5 text-blue-400" />;
    return <Database className="w-3.5 h-3.5 text-purple-400" />;
  };

  return (
    <div className="flex flex-col h-full bg-zinc-900/60 rounded-xl border border-zinc-800/80 overflow-hidden shadow-2xl">
      {/* Top Bar Controls & Telemetry */}
      <div className="px-4 py-3 border-b border-zinc-800 flex flex-wrap items-center justify-between gap-3 bg-zinc-900/90 backdrop-blur">
        <div className="flex items-center gap-2">
          <div className="flex bg-zinc-950 p-0.5 rounded-lg border border-zinc-800">
            <button
              onClick={() => setViewMode('split')}
              className={`flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${
                viewMode === 'split'
                  ? 'bg-zinc-800 text-zinc-100 shadow-sm'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
              title="Interactive split-screen slider"
            >
              <SplitSquareVertical className="w-3.5 h-3.5" />
              Split Slider
            </button>
            <button
              onClick={() => setViewMode('side-by-side')}
              className={`flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${
                viewMode === 'side-by-side'
                  ? 'bg-zinc-800 text-zinc-100 shadow-sm'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
              title="Side-by-side view"
            >
              <Columns className="w-3.5 h-3.5" />
              Side by Side
            </button>
          </div>

          <div className="flex items-center bg-zinc-950 rounded-lg border border-zinc-800 p-0.5 text-xs text-zinc-400">
            <button
              onClick={() => setZoom(Math.max(0.5, zoom - 0.25))}
              className="p-1 hover:text-zinc-100 transition-colors"
              title="Zoom out"
            >
              <ZoomOut className="w-3.5 h-3.5" />
            </button>
            <span className="px-2 font-mono text-[11px] tabular-nums text-zinc-300">
              {Math.round(zoom * 100)}%
            </span>
            <button
              onClick={() => setZoom(Math.min(3.0, zoom + 0.25))}
              className="p-1 hover:text-zinc-100 transition-colors"
              title="Zoom in"
            >
              <ZoomIn className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => setZoom(1.0)}
              className="p-1 hover:text-zinc-100 transition-colors border-l border-zinc-800 ml-0.5"
              title="Reset Zoom"
            >
              <Maximize2 className="w-3.5 h-3.5" />
            </button>
          </div>

          <button
            onClick={onRefresh}
            className="p-1.5 rounded-lg border border-zinc-800 bg-zinc-950 text-zinc-400 hover:text-zinc-100 transition-colors"
            title="Refresh transformation"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-indigo-400' : ''}`} />
          </button>
        </div>

        {/* Real-time Telemetry Metrics Pill */}
        <div className="flex items-center gap-2 text-xs font-mono">
          {telemetry && (
            <>
              <div className="flex items-center gap-1.5 px-2.5 py-1 bg-zinc-950 rounded-md border border-zinc-800/80">
                {getCacheIcon(telemetry.cacheSource)}
                <span className="text-zinc-300 text-[11px] font-semibold">
                  {telemetry.cacheSource.replace('_', ' ')}
                </span>
                <span className="text-zinc-500">|</span>
                <span className="text-emerald-400 tabular-nums font-semibold">
                  {telemetry.durationMs.toFixed(1)}ms
                </span>
              </div>

              <div className="hidden sm:flex items-center gap-1.5 px-2.5 py-1 bg-zinc-950 rounded-md border border-zinc-800/80">
                <span className="text-zinc-400 text-[11px]">
                  {formatBytes(origBytes)} → {formatBytes(transBytes)}
                </span>
                {savingsPct > 0 ? (
                  <span className="text-emerald-400 font-semibold text-[11px]">
                    (-{savingsPct.toFixed(1)}%)
                  </span>
                ) : savingsPct < 0 ? (
                  <span className="text-amber-400 font-semibold text-[11px]">
                    (+{Math.abs(savingsPct).toFixed(1)}%)
                  </span>
                ) : null}
              </div>

              <div className="hidden md:flex items-center px-2 py-1 bg-zinc-950 rounded-md border border-zinc-800/80 text-[11px] text-zinc-400 uppercase">
                {telemetry.contentType.replace('image/', '')}
              </div>
            </>
          )}
        </div>
      </div>

      {/* Main Canvas Viewer */}
      <div className="relative flex-1 bg-zinc-950 overflow-hidden flex items-center justify-center p-4">
        {/* Subtle grid pattern background */}
        <div
          className="absolute inset-0 opacity-20 pointer-events-none"
          style={{
            backgroundImage:
              'radial-gradient(#52525b 1px, transparent 1px), radial-gradient(#52525b 1px, #09090b 1px)',
            backgroundSize: '24px 24px',
          }}
        />

        {loading && (
          <div className="absolute top-4 right-4 z-30 flex items-center gap-2 px-3 py-1.5 rounded-full bg-indigo-950/80 border border-indigo-700/60 text-indigo-300 text-xs shadow-lg backdrop-blur">
            <RefreshCw className="w-3.5 h-3.5 animate-spin" />
            <span>Processing...</span>
          </div>
        )}

        {viewMode === 'split' ? (
          <div
            ref={containerRef}
            className="relative select-none rounded-lg overflow-hidden border border-zinc-800 shadow-2xl transition-transform duration-100"
            style={{
              transform: `scale(${zoom})`,
              maxWidth: '100%',
              maxHeight: '100%',
            }}
          >
            {/* Original Image (Left / Base Layer) */}
            <div className="relative">
              <img
                src={originalUrl}
                alt="Original"
                className="max-h-[68vh] object-contain pointer-events-none block"
              />
              <div className="absolute top-2 left-2 px-2 py-0.5 rounded bg-black/75 backdrop-blur text-[11px] font-mono text-zinc-300 border border-white/10">
                Original {telemetry?.originalWidth ? `(${telemetry.originalWidth}×${telemetry.originalHeight})` : ''}
              </div>
            </div>

            {/* Transformed Image (Right / Clipped Overlay) */}
            <div
              className="absolute inset-0 overflow-hidden pointer-events-none"
              style={{
                clipPath: `polygon(${sliderPos}% 0, 100% 0, 100% 100%, ${sliderPos}% 100%)`,
              }}
            >
              <img
                src={transformedUrl}
                alt="Transformed"
                className="max-h-[68vh] object-contain pointer-events-none block"
              />
              <div className="absolute top-2 right-2 px-2 py-0.5 rounded bg-indigo-950/85 backdrop-blur text-[11px] font-mono text-indigo-200 border border-indigo-700/40">
                imgscale {telemetry?.outputWidth ? `(${telemetry.outputWidth}×${telemetry.outputHeight})` : ''}
              </div>
            </div>

            {/* Draggable Divider Line & Knob */}
            <div
              className="absolute top-0 bottom-0 w-0.5 bg-indigo-400 cursor-ew-resize z-20"
              style={{ left: `${sliderPos}%` }}
              onMouseDown={() => setIsDragging(true)}
              onTouchStart={() => setIsDragging(true)}
            >
              <div className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2 w-7 h-7 rounded-full bg-zinc-900 border-2 border-indigo-400 shadow-xl flex items-center justify-center cursor-ew-resize text-indigo-300 hover:scale-110 active:scale-95 transition-transform">
                <SplitSquareVertical className="w-3.5 h-3.5" />
              </div>
            </div>
          </div>
        ) : (
          /* Side by Side Mode */
          <div
            className="grid grid-cols-1 md:grid-cols-2 gap-4 w-full h-full p-2 overflow-auto"
            style={{ transform: `scale(${zoom})`, transformOrigin: 'center center' }}
          >
            <div className="flex flex-col items-center justify-center p-2 rounded-lg bg-zinc-900/40 border border-zinc-800">
              <div className="mb-2 text-xs font-mono text-zinc-400 flex items-center gap-2">
                <span className="font-semibold text-zinc-200">Origin Source</span>
                {telemetry?.originalWidth && (
                  <span>{telemetry.originalWidth}×{telemetry.originalHeight} px</span>
                )}
                {origBytes > 0 && <span>({formatBytes(origBytes)})</span>}
              </div>
              <img
                src={originalUrl}
                alt="Original"
                className="max-h-[60vh] object-contain rounded border border-zinc-800/80"
              />
            </div>

            <div className="flex flex-col items-center justify-center p-2 rounded-lg bg-zinc-900/40 border border-zinc-800">
              <div className="mb-2 text-xs font-mono text-indigo-300 flex items-center gap-2">
                <span className="font-semibold text-indigo-400">Transformed Output</span>
                {telemetry?.outputWidth && (
                  <span>{telemetry.outputWidth}×{telemetry.outputHeight} px</span>
                )}
                {transBytes > 0 && <span>({formatBytes(transBytes)})</span>}
              </div>
              <img
                src={transformedUrl}
                alt="Transformed"
                className="max-h-[60vh] object-contain rounded border border-indigo-900/40"
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
