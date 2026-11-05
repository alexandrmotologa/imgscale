import React, { useState } from 'react';
import { DollarSign, Server } from 'lucide-react';

export const CostCalculator: React.FC = () => {
  const [monthlyRequests, setMonthlyRequests] = useState<number>(500000);
  const [avgImageKb, setAvgImageKb] = useState<number>(850);
  const [webpSavingsPct] = useState<number>(72); // typical 70-80% WebP compression savings

  // Calculations
  const totalOriginalGb = (monthlyRequests * avgImageKb) / (1024 * 1024);
  const optimizedImageKb = avgImageKb * (1 - webpSavingsPct / 100);
  const totalOptimizedGb = (monthlyRequests * optimizedImageKb) / (1024 * 1024);
  const bandwidthSavedGb = totalOriginalGb - totalOptimizedGb;

  // Cloudinary pricing estimate:
  // Base Plus tier: $89/mo (includes 225 credits = 225k transformations or 225 GB)
  // Additional transformations: ~$0.08 per 1,000 ($80 / 1M)
  // Bandwidth overage: ~$0.09 per GB
  const cloudinaryTransformsCost = (monthlyRequests / 1000) * 0.08;
  const cloudinaryBandwidthCost = totalOptimizedGb * 0.09;
  const cloudinaryTotal = Math.max(89, Math.round(cloudinaryTransformsCost + cloudinaryBandwidthCost));

  // Imgix pricing estimate:
  // $10 per 1,000 master images + $0.08 per GB bandwidth
  const masterImages = Math.round(monthlyRequests * 0.05); // assuming 5% unique master images
  const imgixCost = Math.round((masterImages / 1000) * 10 + totalOptimizedGb * 0.08);

  // imgscale cost:
  // Runs on a $5/mo VPS (e.g. Hetzner CX22, DigitalOcean $4-6 Droplet) with < 35MB RAM footprint
  const imgscaleCost = 5;
  const monthlySavings = Math.max(0, cloudinaryTotal - imgscaleCost);
  const yearlySavings = monthlySavings * 12;

  return (
    <div className="bg-zinc-900/60 rounded-xl border border-zinc-800/80 p-4 shadow-xl">
      <div className="flex items-center justify-between mb-4 border-b border-zinc-800 pb-2.5">
        <div className="flex items-center gap-2">
          <DollarSign className="w-4 h-4 text-emerald-400" />
          <h3 className="text-xs font-semibold tracking-wide uppercase text-zinc-200">
            Cloudinary vs imgscale ROI Calculator
          </h3>
        </div>
        <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-emerald-950/80 border border-emerald-800/60 text-emerald-300">
          Save Up to 98%
        </span>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Sliders for usage */}
        <div className="space-y-3.5 bg-zinc-950/80 rounded-lg border border-zinc-800/90 p-3.5">
          <div>
            <div className="flex justify-between text-xs mb-1.5">
              <span className="text-zinc-400">Monthly Image Requests</span>
              <span className="font-mono text-zinc-200 tabular-nums">
                {monthlyRequests.toLocaleString()} / mo
              </span>
            </div>
            <input
              type="range"
              min="50000"
              max="5000000"
              step="50000"
              value={monthlyRequests}
              onChange={(e) => setMonthlyRequests(parseInt(e.target.value))}
              className="w-full accent-emerald-500 bg-zinc-900 cursor-pointer h-1.5 rounded-lg"
            />
          </div>

          <div>
            <div className="flex justify-between text-xs mb-1.5">
              <span className="text-zinc-400">Average Original Image Size</span>
              <span className="font-mono text-zinc-200 tabular-nums">{avgImageKb} KB</span>
            </div>
            <input
              type="range"
              min="100"
              max="3000"
              step="50"
              value={avgImageKb}
              onChange={(e) => setAvgImageKb(parseInt(e.target.value))}
              className="w-full accent-emerald-500 bg-zinc-900 cursor-pointer h-1.5 rounded-lg"
            />
          </div>

          <div className="pt-2 border-t border-zinc-800/80 grid grid-cols-2 gap-2 text-[11px] font-mono">
            <div className="bg-zinc-900/70 p-2 rounded border border-zinc-800">
              <span className="text-zinc-400 block text-[10px]">Optimized WebP Egress</span>
              <span className="text-zinc-200 font-semibold">{totalOptimizedGb.toFixed(1)} GB</span>
            </div>
            <div className="bg-zinc-900/70 p-2 rounded border border-zinc-800">
              <span className="text-zinc-400 block text-[10px]">Bandwidth Saved</span>
              <span className="text-emerald-400 font-semibold">{bandwidthSavedGb.toFixed(1)} GB</span>
            </div>
          </div>
        </div>

        {/* Cost Comparison breakdown */}
        <div className="bg-zinc-950/80 rounded-lg border border-zinc-800/90 p-3.5 flex flex-col justify-between">
          <div className="space-y-2">
            <div className="flex justify-between items-center text-xs">
              <span className="text-zinc-400">Cloudinary Equivalent:</span>
              <span className="font-mono text-rose-400 font-semibold tabular-nums line-through">
                ${cloudinaryTotal} / mo
              </span>
            </div>
            <div className="flex justify-between items-center text-xs">
              <span className="text-zinc-400">Imgix Equivalent:</span>
              <span className="font-mono text-amber-400 tabular-nums line-through">
                ${imgixCost} / mo
              </span>
            </div>
            <div className="flex justify-between items-center text-xs">
              <span className="text-emerald-300 font-medium flex items-center gap-1">
                <Server className="w-3.5 h-3.5 text-emerald-400" />
                imgscale Self-Hosted:
              </span>
              <span className="font-mono text-emerald-400 font-bold tabular-nums">
                ${imgscaleCost} / mo
              </span>
            </div>
          </div>

          <div className="mt-3 pt-3 border-t border-zinc-800/80 bg-emerald-950/30 rounded-lg p-2.5 border border-emerald-900/40">
            <div className="flex items-center justify-between">
              <div>
                <span className="text-[10px] uppercase font-mono tracking-wider text-emerald-300 block">
                  Net Estimated Savings
                </span>
                <div className="text-lg font-bold font-mono text-emerald-400 tabular-nums">
                  ${monthlySavings.toLocaleString()} <span className="text-xs font-normal text-emerald-300">/ mo</span>
                </div>
              </div>
              <div className="text-right">
                <span className="text-[10px] font-mono text-zinc-400 block">Annual Retention</span>
                <span className="text-sm font-bold font-mono text-zinc-100 tabular-nums">
                  +${yearlySavings.toLocaleString()} / yr
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
