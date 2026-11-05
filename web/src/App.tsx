import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { UrlBuilder } from './components/UrlBuilder';
import { BeforeAfterDiff } from './components/BeforeAfterDiff';
import { SourceExplorer } from './components/SourceExplorer';
import { CacheMetrics } from './components/CacheMetrics';
import { TransformParams, ImageTelemetry, PresetItem } from './types';
import { BookOpen, Layers } from 'lucide-react';

const GithubIcon = () => (
  <svg className="w-3.5 h-3.5 fill-current" viewBox="0 0 24 24">
    <path fillRule="evenodd" clipRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
  </svg>
);

export const App: React.FC = () => {
  const [selectedPath, setSelectedPath] = useState<string>('portrait.png');
  const [presets, setPresets] = useState<PresetItem[]>([]);
  const [telemetry, setTelemetry] = useState<ImageTelemetry | null>(null);
  const [loading, setLoading] = useState(false);
  const [signing, setSigning] = useState(false);
  const [signature, setSignature] = useState<string>('');

  const [params, setParams] = useState<TransformParams>({
    width: 400,
    height: 400,
    fit: 'cover',
    gravity: 'smart',
    format: 'auto',
    quality: 80,
    dpr: 1.0,
    blur: 0,
    sharpen: 0,
    gray: false,
    rot: 0,
    debug: '',
    secret: '',
  });

  // Fetch presets on load
  useEffect(() => {
    fetch('/api/presets')
      .then((res) => (res.ok ? res.json() : []))
      .then((data: PresetItem[]) => {
        if (data && data.length > 0) {
          setPresets(data);
          if (!data.some((d) => d.path === selectedPath)) {
            setSelectedPath(data[0].path);
          }
        }
      })
      .catch(() => {});
  }, []);

  // Build transformed URL
  const transformedUrl = useMemo(() => {
    const q = new URLSearchParams();
    if (params.width > 0) q.set('w', params.width.toString());
    if (params.height > 0) q.set('h', params.height.toString());
    if (params.fit) q.set('fit', params.fit);
    if (params.gravity) q.set('gravity', params.gravity);
    if (params.format && params.format !== 'auto') q.set('format', params.format);
    if (params.quality !== 80) q.set('q', params.quality.toString());
    if (params.dpr !== 1.0) q.set('dpr', params.dpr.toString());
    if (params.blur > 0) q.set('blur', params.blur.toString());
    if (params.sharpen > 0) q.set('sharpen', params.sharpen.toString());
    if (params.gray) q.set('gray', 'true');
    if (params.rot !== 0) q.set('rot', params.rot.toString());
    if (params.debug) q.set('debug', params.debug);
    if (signature) q.set('s', signature);

    const qs = q.toString();
    const endpoint = params.debug === 'entropy' ? '/v1/entropy/' : '/v1/images/';
    return `${window.location.origin}${endpoint}${selectedPath}${qs ? '?' + qs : ''}`;
  }, [selectedPath, params, signature]);

  // Original image URL
  const originalUrl = useMemo(() => {
    return `${window.location.origin}/v1/images/${selectedPath}?fit=inside`;
  }, [selectedPath]);

  // Measure telemetry by issuing a HEAD/GET request
  const refreshTelemetry = useCallback(async () => {
    try {
      setLoading(true);
      const start = performance.now();
      const res = await fetch(transformedUrl);
      const elapsed = performance.now() - start;

      if (res.ok) {
        const blob = await res.blob();
        const durationHeader = res.headers.get('X-Transform-Duration-Ms');
        const cacheHeader = res.headers.get('X-Cache') || 'ORIGIN_COMPUTE';
        const etag = res.headers.get('ETag') || '';

        // Read image natural dimensions
        const img = new Image();
        img.src = URL.createObjectURL(blob);
        img.onload = () => {
          setTelemetry({
            originalBytes: 150 * 1024, // baseline fallback
            transformedBytes: blob.size,
            originalWidth: 800,
            originalHeight: 800,
            outputWidth: img.naturalWidth,
            outputHeight: img.naturalHeight,
            durationMs: durationHeader ? parseFloat(durationHeader) : elapsed,
            cacheSource: cacheHeader,
            etag,
            contentType: res.headers.get('Content-Type') || 'image/jpeg',
          });
        };
      }
    } catch {
      // Ignored
    } finally {
      setLoading(false);
    }
  }, [transformedUrl]);

  useEffect(() => {
    // Reset signature when params change
    setSignature('');
    const timer = setTimeout(refreshTelemetry, 250);
    return () => clearTimeout(timer);
  }, [params, selectedPath, refreshTelemetry]);

  const handleSign = async () => {
    if (!params.secret) {
      alert('Please enter a secret key to compute HMAC-SHA256 signature.');
      return;
    }
    try {
      setSigning(true);
      const res = await fetch('/api/sign', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url: transformedUrl,
          secret: params.secret,
        }),
      });
      if (res.ok) {
        const data = await res.json();
        setSignature(data.signature);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setSigning(false);
    }
  };

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 flex flex-col font-sans">
      {/* Top Application Header */}
      <header className="px-6 py-3.5 border-b border-zinc-800/80 bg-zinc-900/60 backdrop-blur sticky top-0 z-40 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-tr from-indigo-600 to-indigo-400 flex items-center justify-center shadow-lg shadow-indigo-500/20 text-white font-bold font-mono text-sm">
            <Layers className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="font-bold tracking-tight text-sm text-zinc-100">imgscale</h1>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-indigo-950/80 border border-indigo-700/50 text-indigo-300">
                v1.0.0
              </span>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-emerald-950/80 border border-emerald-700/50 text-emerald-300">
                &lt; 35MB RAM
              </span>
            </div>
            <p className="text-[11px] text-zinc-400">
              Real-time SIMD Resizing & Sobel Saliency Smart Crop Studio
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <a
            href="https://github.com/alexandrmotologa/imgscale"
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-zinc-800 bg-zinc-900 hover:bg-zinc-800 text-xs font-medium text-zinc-300 hover:text-white transition-colors"
          >
            <GithubIcon />
            <span>GitHub</span>
          </a>
          <a
            href="https://github.com/alexandrmotologa/imgscale/tree/main/docs"
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-zinc-800 bg-zinc-900 hover:bg-zinc-800 text-xs font-medium text-zinc-300 hover:text-white transition-colors"
          >
            <BookOpen className="w-3.5 h-3.5" />
            <span>Docs</span>
          </a>
        </div>
      </header>

      {/* Main Workspace Layout */}
      <main className="flex-1 p-4 max-w-[1728px] mx-auto w-full flex flex-col gap-4">
        {/* Source Image Selector */}
        <SourceExplorer
          selectedPath={selectedPath}
          onSelect={(p) => setSelectedPath(p)}
          presets={presets}
        />

        {/* Primary Two-Column Cockpit */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-4 flex-1">
          {/* Left Column: Parameter Tuner (4 cols) */}
          <div className="lg:col-span-4">
            <UrlBuilder
              params={params}
              onChange={(updated) => setParams((prev) => ({ ...prev, ...updated }))}
              generatedUrl={transformedUrl}
              onSignUrl={handleSign}
              signing={signing}
            />
          </div>

          {/* Right Column: Visual Comparison & Saliency Viewer (8 cols) */}
          <div className="lg:col-span-8 flex flex-col gap-4">
            <BeforeAfterDiff
              originalUrl={originalUrl}
              transformedUrl={transformedUrl}
              telemetry={telemetry}
              loading={loading}
              onRefresh={refreshTelemetry}
            />

            {/* Live Cache & System Telemetry */}
            <CacheMetrics />
          </div>
        </div>
      </main>
    </div>
  );
};
