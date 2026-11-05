import React, { useState, useRef } from 'react';
import { Image as ImageIcon, Link2, CheckCircle2, Upload, Loader2 } from 'lucide-react';
import { PresetItem } from '../types';

interface Props {
  selectedPath: string;
  onSelect: (path: string) => void;
  presets: PresetItem[];
  onUploadSuccess?: (item: PresetItem) => void;
}

export const SourceExplorer: React.FC<Props> = ({
  selectedPath,
  onSelect,
  presets,
  onUploadSuccess,
}) => {
  const [customPath, setCustomPath] = useState('');
  const [isDragging, setIsDragging] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const defaultPresets: PresetItem[] = [
    { id: 'portrait', name: 'Portrait (Off-Center Subject)', category: 'Face & Crop', path: 'portrait.png' },
    { id: 'landscape', name: 'Scenic 1080p Landscape', category: 'Photography', path: 'landscape.jpg' },
    { id: 'diagram', name: 'Technical Schematic Diagram', category: 'Graphics', path: 'diagram.png' },
    { id: 'pattern', name: 'Procedural Geometric Pattern', category: 'Textures', path: 'pattern.webp' },
  ];

  const items = presets.length > 0 ? presets : defaultPresets;

  const handleCustomSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (customPath.trim()) {
      onSelect(customPath.trim());
    }
  };

  const uploadFile = async (file: File) => {
    if (!file.type.startsWith('image/')) {
      alert('Please upload an image file (JPEG, PNG, WebP, GIF)');
      return;
    }

    try {
      setUploading(true);
      const formData = new FormData();
      formData.append('image', file);

      const res = await fetch('/api/upload', {
        method: 'POST',
        body: formData,
      });

      if (res.ok) {
        const data = await res.json();
        const newItem: PresetItem = {
          id: data.path,
          name: data.name || file.name,
          category: 'Upload',
          path: data.path,
          width: data.width,
          height: data.height,
        };
        if (onUploadSuccess) {
          onUploadSuccess(newItem);
        }
        onSelect(newItem.path);
      } else {
        const errText = await res.text();
        alert('Upload failed: ' + errText);
      }
    } catch (err: any) {
      alert('Network error uploading image: ' + err.message);
    } finally {
      setUploading(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      uploadFile(e.dataTransfer.files[0]);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  return (
    <div
      onDrop={handleDrop}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      className={`flex flex-col gap-3 p-4 rounded-xl border transition-all shadow-xl ${
        isDragging
          ? 'bg-indigo-950/80 border-indigo-500 ring-2 ring-indigo-500/50'
          : 'bg-zinc-900/60 border-zinc-800/80'
      }`}
    >
      <div className="flex items-center justify-between border-b border-zinc-800 pb-2.5">
        <div className="flex items-center gap-2">
          <ImageIcon className="w-4 h-4 text-indigo-400" />
          <h3 className="text-xs font-semibold tracking-wide uppercase text-zinc-200">
            Source Image Gallery & Upload
          </h3>
        </div>

        <div className="flex items-center gap-3">
          <input
            type="file"
            ref={fileInputRef}
            onChange={(e) => {
              if (e.target.files && e.target.files.length > 0) {
                uploadFile(e.target.files[0]);
              }
            }}
            accept="image/*"
            className="hidden"
          />
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 text-xs font-medium text-zinc-200 transition-colors shadow-sm"
          >
            {uploading ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin text-indigo-400" />
            ) : (
              <Upload className="w-3.5 h-3.5" />
            )}
            <span>{uploading ? 'Uploading...' : 'Drop or Upload File'}</span>
          </button>

          <span className="text-[11px] font-mono text-zinc-400 hidden sm:inline">
            Active: <span className="text-indigo-300 font-semibold">{selectedPath}</span>
          </span>
        </div>
      </div>

      {/* Preset Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        {items.map((item) => {
          const isSelected = selectedPath === item.path;
          return (
            <button
              key={item.id}
              onClick={() => onSelect(item.path)}
              className={`p-2 rounded-lg border text-left flex flex-col justify-between transition-all ${
                isSelected
                  ? 'bg-indigo-950/70 border-indigo-500 shadow-md ring-1 ring-indigo-500/50'
                  : 'bg-zinc-950 border-zinc-800 hover:border-zinc-700 hover:bg-zinc-900/60'
              }`}
            >
              <div className="flex items-start justify-between w-full mb-1">
                <span className="text-[10px] font-mono uppercase text-zinc-400">
                  {item.category}
                </span>
                {isSelected && <CheckCircle2 className="w-3.5 h-3.5 text-indigo-400" />}
              </div>
              <div className="text-xs font-medium text-zinc-200 truncate w-full">
                {item.name}
              </div>
              <div className="text-[11px] font-mono text-zinc-400 mt-1 truncate">
                {item.path}
              </div>
            </button>
          );
        })}
      </div>

      {/* Custom Path or URL Input */}
      <form onSubmit={handleCustomSubmit} className="flex gap-2 pt-1">
        <div className="relative flex-1">
          <Link2 className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-zinc-500" />
          <input
            type="text"
            placeholder="Custom storage path (e.g. photos/banner.jpg or https://...)"
            value={customPath}
            onChange={(e) => setCustomPath(e.target.value)}
            className="w-full pl-8 pr-3 py-1.5 bg-zinc-950 border border-zinc-800 rounded-lg text-xs font-mono text-zinc-100 focus:outline-none focus:border-indigo-500"
          />
        </div>
        <button
          type="submit"
          className="px-3 py-1.5 bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 text-zinc-200 rounded-lg text-xs font-medium transition-colors"
        >
          Load
        </button>
      </form>
    </div>
  );
};
