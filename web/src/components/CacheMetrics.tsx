import React, { useEffect, useState } from 'react';
import { Zap, HardDrive, Cpu, Activity, RefreshCw } from 'lucide-react';

interface StatsResponse {
  uptime_sec: number;
  cache: {
    l1: {
      hits: number;
      misses: number;
      evictions: number;
      total_bytes: number;
      item_count: number;
      hit_ratio: number;
    };
    l2: {
      hits: number;
      misses: number;
      writes: number;
      total_bytes: number;
      file_count: number;
      hit_ratio: number;
    };
  };
  system: {
    alloc_mb: number;
    sys_mb: number;
    heap_in_use_mb: number;
    num_goroutine: number;
    num_gc: number;
  };
}

export const CacheMetrics: React.FC = () => {
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [loading, setLoading] = useState(false);

  const fetchStats = async () => {
    try {
      setLoading(true);
      const res = await fetch('/api/stats');
      if (res.ok) {
        const data = await res.json();
        setStats(data);
      }
    } catch {
      // Endpoint may be offline in dev
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStats();
    if (!autoRefresh) return;
    const timer = setInterval(fetchStats, 2500);
    return () => clearInterval(timer);
  }, [autoRefresh]);

  const formatBytes = (bytes: number) => {
    if (bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div className="bg-zinc-900/60 rounded-xl border border-zinc-800/80 p-4 shadow-xl">
      <div className="flex items-center justify-between mb-4 border-b border-zinc-800 pb-2.5">
        <div className="flex items-center gap-2">
          <Activity className="w-4 h-4 text-emerald-400" />
          <h3 className="text-xs font-semibold tracking-wide uppercase text-zinc-200">
            Live Gateway Telemetry
          </h3>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`text-[11px] font-mono px-2 py-0.5 rounded border transition-colors ${
              autoRefresh
                ? 'bg-emerald-950/60 border-emerald-800/60 text-emerald-300'
                : 'bg-zinc-950 border-zinc-800 text-zinc-500'
            }`}
          >
            Auto-Poll: {autoRefresh ? 'ON' : 'OFF'}
          </button>
          <button
            onClick={fetchStats}
            className="p-1 text-zinc-400 hover:text-zinc-200 transition-colors"
            title="Refresh stats"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-indigo-400' : ''}`} />
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* Tier 1 Memory LRU */}
        <div className="bg-zinc-950/80 rounded-lg border border-zinc-800/90 p-3 flex flex-col justify-between">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-zinc-300 flex items-center gap-1.5">
              <Zap className="w-3.5 h-3.5 text-amber-400" />
              L1 Memory LRU
            </span>
            <span className="text-[11px] font-mono text-emerald-400 font-semibold tabular-nums">
              {stats ? (stats.cache.l1.hit_ratio * 100).toFixed(1) : 0}% Hit
            </span>
          </div>

          <div className="space-y-1 text-[11px] font-mono text-zinc-400">
            <div className="flex justify-between">
              <span>RAM Used:</span>
              <span className="text-zinc-200 tabular-nums">
                {stats ? formatBytes(stats.cache.l1.total_bytes) : '0 B'}
              </span>
            </div>
            <div className="flex justify-between">
              <span>Hot Items:</span>
              <span className="text-zinc-200 tabular-nums">
                {stats ? stats.cache.l1.item_count : 0}
              </span>
            </div>
            <div className="flex justify-between">
              <span>Hits / Misses:</span>
              <span className="text-zinc-200 tabular-nums">
                {stats ? `${stats.cache.l1.hits} / ${stats.cache.l1.misses}` : '0 / 0'}
              </span>
            </div>
          </div>
        </div>

        {/* Tier 2 Disk Cache */}
        <div className="bg-zinc-950/80 rounded-lg border border-zinc-800/90 p-3 flex flex-col justify-between">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-zinc-300 flex items-center gap-1.5">
              <HardDrive className="w-3.5 h-3.5 text-blue-400" />
              L2 Persistent Disk
            </span>
            <span className="text-[11px] font-mono text-blue-400 font-semibold tabular-nums">
              {stats ? (stats.cache.l2.hit_ratio * 100).toFixed(1) : 0}% Hit
            </span>
          </div>

          <div className="space-y-1 text-[11px] font-mono text-zinc-400">
            <div className="flex justify-between">
              <span>Disk Usage:</span>
              <span className="text-zinc-200 tabular-nums">
                {stats ? formatBytes(stats.cache.l2.total_bytes) : '0 B'}
              </span>
            </div>
            <div className="flex justify-between">
              <span>Cached Blocks:</span>
              <span className="text-zinc-200 tabular-nums">
                {stats ? stats.cache.l2.file_count : 0}
              </span>
            </div>
            <div className="flex justify-between">
              <span>Writes / Hits:</span>
              <span className="text-zinc-200 tabular-nums">
                {stats ? `${stats.cache.l2.writes} / ${stats.cache.l2.hits}` : '0 / 0'}
              </span>
            </div>
          </div>
        </div>

        {/* Process & Go Runtime */}
        <div className="bg-zinc-950/80 rounded-lg border border-zinc-800/90 p-3 flex flex-col justify-between">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-zinc-300 flex items-center gap-1.5">
              <Cpu className="w-3.5 h-3.5 text-indigo-400" />
              Runtime Footprint
            </span>
            <span className="text-[11px] font-mono text-indigo-300 font-semibold tabular-nums">
              {stats ? stats.system.alloc_mb.toFixed(1) : '< 25'} MB RAM
            </span>
          </div>

          <div className="space-y-1 text-[11px] font-mono text-zinc-400">
            <div className="flex justify-between">
              <span>Heap In-Use:</span>
              <span className="text-zinc-200 tabular-nums">
                {stats ? `${stats.system.heap_in_use_mb.toFixed(1)} MB` : '0 MB'}
              </span>
            </div>
            <div className="flex justify-between">
              <span>Goroutines:</span>
              <span className="text-zinc-200 tabular-nums">
                {stats ? stats.system.num_goroutine : 1}
              </span>
            </div>
            <div className="flex justify-between">
              <span>Uptime:</span>
              <span className="text-zinc-200 tabular-nums">
                {stats ? `${stats.uptime_sec}s` : '0s'}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
