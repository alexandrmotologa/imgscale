import React, { useState } from 'react';
import {
  Sliders,
  Crop,
  Sparkles,
  Copy,
  Check,
  ShieldCheck,
  Code2,
  Lock,
  Unlock,
  Flame,
  Layers,
} from 'lucide-react';
import { TransformParams } from '../types';

interface Props {
  params: TransformParams;
  onChange: (updated: Partial<TransformParams>) => void;
  generatedUrl: string;
  onSignUrl: () => Promise<void>;
  signing: boolean;
}

export const UrlBuilder: React.FC<Props> = ({
  params,
  onChange,
  generatedUrl,
  onSignUrl,
  signing,
}) => {
  const [aspectLocked, setAspectLocked] = useState(false);
  const [aspectRatio, setAspectRatio] = useState(params.width > 0 && params.height > 0 ? params.width / params.height : 1);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [activeSnippetTab, setActiveSnippetTab] = useState<'url' | 'html' | 'curl' | 'nextjs'>('url');

  const copyToClipboard = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const handleWidthChange = (w: number) => {
    if (aspectLocked && w > 0 && aspectRatio > 0) {
      onChange({ width: w, height: Math.round(w / aspectRatio) });
    } else {
      onChange({ width: w });
      if (w > 0 && params.height > 0) {
        setAspectRatio(w / params.height);
      }
    }
  };

  const handleHeightChange = (h: number) => {
    if (aspectLocked && h > 0 && aspectRatio > 0) {
      onChange({ height: h, width: Math.round(h * aspectRatio) });
    } else {
      onChange({ height: h });
      if (params.width > 0 && h > 0) {
        setAspectRatio(params.width / h);
      }
    }
  };

  const dimensionPresets = [
    { label: 'Thumbnail', w: 150, h: 150 },
    { label: 'Avatar', w: 300, h: 300 },
    { label: 'Card', w: 600, h: 400 },
    { label: 'Hero', w: 1200, h: 630 },
  ];

  const getHtmlSnippet = () => `<img src="${generatedUrl}" alt="Optimized via imgscale" loading="lazy" />`;
  const getCurlSnippet = () => `curl -O "${generatedUrl}"`;
  const getNextJsSnippet = () => `<Image src="${generatedUrl}" alt="Media" width={${params.width || 800}} height={${params.height || 600}} />`;

  return (
    <div className="flex flex-col gap-5 p-4 bg-zinc-900/60 rounded-xl border border-zinc-800/80 shadow-xl overflow-y-auto max-h-[calc(100vh-140px)]">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-zinc-800 pb-3">
        <div className="flex items-center gap-2">
          <Sliders className="w-4 h-4 text-indigo-400" />
          <h2 className="text-sm font-semibold tracking-wide uppercase text-zinc-200">
            Transformation Pipeline
          </h2>
        </div>
        <span className="text-[11px] font-mono px-2 py-0.5 rounded bg-indigo-950/60 border border-indigo-800/50 text-indigo-300">
          Pure Go SIMD
        </span>
      </div>

      {/* Target Dimensions */}
      <div className="space-y-2">
        <div className="flex items-center justify-between text-xs">
          <label className="font-medium text-zinc-300">Dimensions (px)</label>
          <button
            onClick={() => {
              if (!aspectLocked && params.width > 0 && params.height > 0) {
                setAspectRatio(params.width / params.height);
              }
              setAspectLocked(!aspectLocked);
            }}
            className={`flex items-center gap-1 text-[11px] px-1.5 py-0.5 rounded border transition-colors ${
              aspectLocked
                ? 'bg-indigo-950 border-indigo-700/60 text-indigo-300'
                : 'border-zinc-800 text-zinc-500 hover:text-zinc-300'
            }`}
          >
            {aspectLocked ? <Lock className="w-3 h-3" /> : <Unlock className="w-3 h-3" />}
            {aspectLocked ? 'Aspect Locked' : 'Unlocked'}
          </button>
        </div>

        <div className="grid grid-cols-2 gap-2">
          <div>
            <div className="text-[11px] text-zinc-400 mb-1">Width</div>
            <input
              type="number"
              min="0"
              max="8192"
              value={params.width || ''}
              placeholder="auto"
              onChange={(e) => handleWidthChange(parseInt(e.target.value) || 0)}
              className="w-full bg-zinc-950 border border-zinc-800 rounded-lg px-3 py-1.5 text-xs font-mono focus:outline-none focus:border-indigo-500 text-zinc-100"
            />
          </div>
          <div>
            <div className="text-[11px] text-zinc-400 mb-1">Height</div>
            <input
              type="number"
              min="0"
              max="8192"
              value={params.height || ''}
              placeholder="auto"
              onChange={(e) => handleHeightChange(parseInt(e.target.value) || 0)}
              className="w-full bg-zinc-950 border border-zinc-800 rounded-lg px-3 py-1.5 text-xs font-mono focus:outline-none focus:border-indigo-500 text-zinc-100"
            />
          </div>
        </div>

        {/* Dimension Preset Pills */}
        <div className="flex flex-wrap gap-1.5 pt-1">
          {dimensionPresets.map((p) => (
            <button
              key={p.label}
              onClick={() => {
                onChange({ width: p.w, height: p.h });
                setAspectRatio(p.w / p.h);
              }}
              className="text-[11px] px-2 py-0.5 rounded bg-zinc-950 hover:bg-zinc-800 border border-zinc-800 text-zinc-400 hover:text-zinc-200 transition-colors"
            >
              {p.label} ({p.w}×{p.h})
            </button>
          ))}
        </div>
      </div>

      {/* Fit Strategy */}
      <div className="space-y-1.5">
        <label className="text-xs font-medium text-zinc-300">Fit Strategy</label>
        <div className="grid grid-cols-3 gap-1">
          {(['cover', 'contain', 'fill', 'inside', 'outside', 'crop'] as const).map((mode) => (
            <button
              key={mode}
              onClick={() => onChange({ fit: mode })}
              className={`text-xs py-1.5 px-2 rounded-lg font-mono transition-colors text-center border capitalize ${
                params.fit === mode
                  ? 'bg-indigo-600 border-indigo-500 text-white font-medium shadow-sm'
                  : 'bg-zinc-950 border-zinc-800 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/60'
              }`}
            >
              {mode}
            </button>
          ))}
        </div>
      </div>

      {/* Gravity Anchor / Smart Sobel Entropy Crop */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="text-xs font-medium text-zinc-300 flex items-center gap-1.5">
            <Crop className="w-3.5 h-3.5 text-indigo-400" />
            Gravity / Focal Point
          </label>
          {params.gravity === 'smart' && (
            <span className="text-[10px] text-emerald-400 font-mono flex items-center gap-1">
              <Sparkles className="w-3 h-3" /> Sobel Active
            </span>
          )}
        </div>

        {/* Smart Crop Highlight Button */}
        <button
          onClick={() => onChange({ gravity: 'smart' })}
          className={`w-full py-2 px-3 rounded-lg text-xs font-medium flex items-center justify-center gap-2 border transition-all ${
            params.gravity === 'smart'
              ? 'bg-gradient-to-r from-indigo-900/80 to-purple-900/80 border-indigo-500/80 text-indigo-200 shadow-md ring-1 ring-indigo-500/40'
              : 'bg-zinc-950 border-zinc-800 text-zinc-400 hover:text-zinc-200 hover:border-zinc-700'
          }`}
        >
          <Sparkles className="w-3.5 h-3.5 text-indigo-400" />
          <span>Smart Crop (Sobel Visual Entropy)</span>
        </button>

        {/* 2D Compass Grid for directional gravity */}
        <div className="grid grid-cols-3 gap-1 pt-1">
          {[
            { label: 'North', val: 'north' },
            { label: 'Center', val: 'center' },
            { label: 'South', val: 'south' },
            { label: 'West', val: 'west' },
            { label: 'East', val: 'east' },
          ].map((item) => (
            <button
              key={item.val}
              onClick={() => onChange({ gravity: item.val as any })}
              className={`text-xs py-1 px-2 rounded border font-mono transition-colors ${
                params.gravity === item.val
                  ? 'bg-zinc-800 border-zinc-600 text-zinc-100 font-medium'
                  : 'bg-zinc-950 border-zinc-800 text-zinc-500 hover:text-zinc-300'
              }`}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      {/* Format & Compression Quality */}
      <div className="space-y-3 pt-2 border-t border-zinc-800/80">
        <div>
          <label className="text-xs font-medium text-zinc-300 block mb-1.5">Output Format</label>
          <div className="grid grid-cols-5 gap-1">
            {(['auto', 'webp', 'jpeg', 'png', 'gif'] as const).map((fmt) => (
              <button
                key={fmt}
                onClick={() => onChange({ format: fmt })}
                className={`text-xs py-1.5 rounded-lg font-mono uppercase text-center border transition-colors ${
                  params.format === fmt
                    ? 'bg-indigo-600 border-indigo-500 text-white font-medium shadow-sm'
                    : 'bg-zinc-950 border-zinc-800 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/60'
                }`}
              >
                {fmt}
              </button>
            ))}
          </div>
        </div>

        {/* Quality Slider */}
        <div>
          <div className="flex justify-between items-center text-xs mb-1">
            <span className="text-zinc-400">Quality</span>
            <span className="font-mono text-zinc-200 tabular-nums font-semibold">{params.quality}%</span>
          </div>
          <input
            type="range"
            min="1"
            max="100"
            value={params.quality}
            onChange={(e) => onChange({ quality: parseInt(e.target.value) })}
            className="w-full accent-indigo-500 bg-zinc-950 cursor-pointer h-1.5 rounded-lg"
          />
        </div>

        {/* DPR Selector */}
        <div>
          <div className="flex justify-between items-center text-xs mb-1.5">
            <span className="text-zinc-400">Device Pixel Ratio (DPR)</span>
            <span className="font-mono text-zinc-200 tabular-nums">{params.dpr}x</span>
          </div>
          <div className="grid grid-cols-4 gap-1">
            {[1.0, 1.5, 2.0, 3.0].map((d) => (
              <button
                key={d}
                onClick={() => onChange({ dpr: d })}
                className={`text-xs py-1 rounded border font-mono transition-colors ${
                  params.dpr === d
                    ? 'bg-zinc-800 border-zinc-600 text-zinc-100 font-medium'
                    : 'bg-zinc-950 border-zinc-800 text-zinc-500 hover:text-zinc-300'
                }`}
              >
                {d}x
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Filters Section (Blur, Sharpen, Grayscale, Rotation) */}
      <div className="space-y-3 pt-2 border-t border-zinc-800/80">
        <label className="text-xs font-medium text-zinc-300 flex items-center gap-1.5">
          <Layers className="w-3.5 h-3.5 text-indigo-400" />
          Post-Processing Filters
        </label>

        {/* Gaussian Blur */}
        <div>
          <div className="flex justify-between items-center text-xs mb-1">
            <span className="text-zinc-400">Gaussian Blur</span>
            <span className="font-mono text-zinc-200 tabular-nums">{params.blur}px</span>
          </div>
          <input
            type="range"
            min="0"
            max="50"
            value={params.blur}
            onChange={(e) => onChange({ blur: parseFloat(e.target.value) })}
            className="w-full accent-indigo-500 bg-zinc-950 cursor-pointer h-1.5 rounded-lg"
          />
        </div>

        {/* Sharpen */}
        <div>
          <div className="flex justify-between items-center text-xs mb-1">
            <span className="text-zinc-400">Unsharp Mask</span>
            <span className="font-mono text-zinc-200 tabular-nums">{params.sharpen}</span>
          </div>
          <input
            type="range"
            min="0"
            max="50"
            value={params.sharpen}
            onChange={(e) => onChange({ sharpen: parseFloat(e.target.value) })}
            className="w-full accent-indigo-500 bg-zinc-950 cursor-pointer h-1.5 rounded-lg"
          />
        </div>

        {/* Grayscale & Rotation controls */}
        <div className="flex items-center gap-2 pt-1">
          <button
            onClick={() => onChange({ gray: !params.gray })}
            className={`flex-1 py-1.5 px-2 rounded-lg text-xs font-medium border transition-colors ${
              params.gray
                ? 'bg-zinc-800 border-zinc-600 text-zinc-100 shadow-sm'
                : 'bg-zinc-950 border-zinc-800 text-zinc-400 hover:text-zinc-200'
            }`}
          >
            Grayscale: {params.gray ? 'ON' : 'OFF'}
          </button>

          <div className="flex items-center bg-zinc-950 rounded-lg border border-zinc-800 p-0.5">
            {[0, 90, 180, 270].map((deg) => (
              <button
                key={deg}
                onClick={() => onChange({ rot: deg })}
                className={`text-[11px] px-2 py-1 rounded font-mono transition-colors ${
                  params.rot === deg
                    ? 'bg-zinc-800 text-zinc-100 font-semibold'
                    : 'text-zinc-500 hover:text-zinc-300'
                }`}
                title={`Rotate ${deg}°`}
              >
                {deg}°
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Debug Mode: Saliency Heatmap */}
      <div className="pt-2 border-t border-zinc-800/80">
        <div className="flex items-center justify-between">
          <label className="text-xs font-medium text-zinc-300 flex items-center gap-1.5">
            <Flame className="w-3.5 h-3.5 text-amber-400" />
            Saliency Heatmap Debug
          </label>
          <button
            onClick={() => onChange({ debug: params.debug === 'entropy' ? '' : 'entropy' })}
            className={`text-xs px-2.5 py-1 rounded-md border font-medium transition-colors ${
              params.debug === 'entropy'
                ? 'bg-amber-950 border-amber-600/80 text-amber-200'
                : 'bg-zinc-950 border-zinc-800 text-zinc-400 hover:text-zinc-200'
            }`}
          >
            {params.debug === 'entropy' ? 'Heatmap: ACTIVE' : 'Enable Heatmap'}
          </button>
        </div>
      </div>

      {/* URL Signature Generator */}
      <div className="space-y-2 pt-2 border-t border-zinc-800/80">
        <label className="text-xs font-medium text-zinc-300 flex items-center gap-1.5">
          <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
          HMAC-SHA256 Signature Guard
        </label>
        <div className="flex gap-1.5">
          <input
            type="password"
            placeholder="Signing Secret Key..."
            value={params.secret}
            onChange={(e) => onChange({ secret: e.target.value })}
            className="flex-1 bg-zinc-950 border border-zinc-800 rounded-lg px-3 py-1.5 text-xs font-mono text-zinc-100 focus:outline-none focus:border-indigo-500"
          />
          <button
            onClick={onSignUrl}
            disabled={signing}
            className="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white rounded-lg text-xs font-medium transition-colors shadow-sm"
          >
            {signing ? 'Signing...' : 'Sign'}
          </button>
        </div>
      </div>

      {/* Code Snippets & URL Export */}
      <div className="space-y-2 pt-2 border-t border-zinc-800/80">
        <div className="flex items-center justify-between text-xs">
          <span className="text-zinc-400 font-medium flex items-center gap-1.5">
            <Code2 className="w-3.5 h-3.5 text-zinc-300" />
            Integration Snippets
          </span>
          <div className="flex gap-1 bg-zinc-950 p-0.5 rounded border border-zinc-800 text-[10px]">
            {(['url', 'html', 'curl', 'nextjs'] as const).map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveSnippetTab(tab)}
                className={`px-1.5 py-0.5 rounded uppercase font-mono ${
                  activeSnippetTab === tab
                    ? 'bg-zinc-800 text-zinc-100 font-semibold'
                    : 'text-zinc-500 hover:text-zinc-300'
                }`}
              >
                {tab}
              </button>
            ))}
          </div>
        </div>

        <div className="relative group">
          <pre className="bg-zinc-950 border border-zinc-800/90 rounded-lg p-2.5 text-[11px] font-mono text-zinc-300 overflow-x-auto whitespace-pre-wrap break-all max-h-24">
            {activeSnippetTab === 'url' && generatedUrl}
            {activeSnippetTab === 'html' && getHtmlSnippet()}
            {activeSnippetTab === 'curl' && getCurlSnippet()}
            {activeSnippetTab === 'nextjs' && getNextJsSnippet()}
          </pre>

          <button
            onClick={() => {
              const text =
                activeSnippetTab === 'url'
                  ? generatedUrl
                  : activeSnippetTab === 'html'
                  ? getHtmlSnippet()
                  : activeSnippetTab === 'curl'
                  ? getCurlSnippet()
                  : getNextJsSnippet();
              copyToClipboard(text, activeSnippetTab);
            }}
            className="absolute top-2 right-2 p-1.5 rounded-md bg-zinc-900 border border-zinc-700 text-zinc-300 hover:text-white shadow transition-colors"
            title="Copy snippet"
          >
            {copiedKey === activeSnippetTab ? (
              <Check className="w-3.5 h-3.5 text-emerald-400" />
            ) : (
              <Copy className="w-3.5 h-3.5" />
            )}
          </button>
        </div>
      </div>
    </div>
  );
};
