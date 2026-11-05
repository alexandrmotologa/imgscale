export interface TransformParams {
  width: number;
  height: number;
  fit: 'cover' | 'contain' | 'fill' | 'inside' | 'outside' | 'crop';
  gravity: 'center' | 'north' | 'south' | 'east' | 'west' | 'smart';
  format: 'auto' | 'webp' | 'jpeg' | 'png' | 'gif';
  quality: number;
  dpr: number;
  blur: number;
  sharpen: number;
  gray: boolean;
  rot: number;
  debug: '' | 'entropy';
  secret: string;
}

export interface ImageTelemetry {
  originalBytes: number;
  transformedBytes: number;
  originalWidth: number;
  originalHeight: number;
  outputWidth: number;
  outputHeight: number;
  durationMs: number;
  cacheSource: string;
  etag: string;
  contentType: string;
}

export interface PresetItem {
  id: string;
  name: string;
  category: string;
  path: string;
  width?: number;
  height?: number;
}
