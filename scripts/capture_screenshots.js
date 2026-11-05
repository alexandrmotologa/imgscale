const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const root = path.join(__dirname, '..');
const imgDir = path.join(root, 'docs', 'images');
const framesDir = path.join(root, 'docs', 'images', 'frames');
fs.mkdirSync(imgDir, { recursive: true });
fs.mkdirSync(framesDir, { recursive: true });

const browserPath = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';

function capture(filePath, width, height, url, timeBudget = 3500) {
  const cmd = `"${browserPath}" --headless --disable-gpu --no-sandbox --window-size=${width},${height} --virtual-time-budget=${timeBudget} --screenshot="${filePath}" "${url}"`;
  execSync(cmd, { stdio: 'inherit' });
}

async function main() {
  console.log('1. Capturing high-resolution screenshots...');

  // Desktop studio
  console.log('-> Desktop Studio...');
  capture(path.join(imgDir, 'imgscale_studio_desktop.png'), 1728, 1080, 'http://localhost:8999/ui/?image=portrait.png&w=400&h=400&fit=cover&gravity=smart', 4500);

  // Saliency Heatmap
  console.log('-> Saliency Heatmap...');
  capture(path.join(imgDir, 'imgscale_saliency_heatmap.png'), 1728, 1080, 'http://localhost:8999/ui/?image=portrait.png&debug=entropy', 4500);

  // Landscape / Cost Calculator
  console.log('-> Landscape & Metrics...');
  capture(path.join(imgDir, 'imgscale_cost_calculator.png'), 1728, 1080, 'http://localhost:8999/ui/?image=landscape.jpg&w=600&h=400&fit=cover', 4500);

  // Mobile layout
  console.log('-> Mobile Responsive View...');
  capture(path.join(imgDir, 'imgscale_studio_mobile.png'), 390, 844, 'http://localhost:8999/ui/?image=portrait.png', 4000);

  // 2. Generating frames for demo GIF
  console.log('2. Generating demo GIF frames...');
  const demoConfigs = [
    { name: 'frame_01.png', url: 'http://localhost:8999/ui/?image=portrait.png&w=400&h=400&fit=cover&gravity=center' },
    { name: 'frame_02.png', url: 'http://localhost:8999/ui/?image=portrait.png&w=400&h=400&fit=cover&gravity=smart' },
    { name: 'frame_03.png', url: 'http://localhost:8999/ui/?image=portrait.png&w=400&h=400&fit=cover&gravity=fp&fp_x=0.68&fp_y=0.33' },
    { name: 'frame_04.png', url: 'http://localhost:8999/ui/?image=portrait.png&w=400&h=400&fit=cover&gravity=smart&debug=entropy' },
    { name: 'frame_05.png', url: 'http://localhost:8999/ui/?image=diagram.png&w=600&h=400&fit=contain&format=webp&q=85' },
    { name: 'frame_06.png', url: 'http://localhost:8999/ui/?image=landscape.jpg&w=600&h=350&fit=cover&blur=2' },
    { name: 'frame_07.png', url: 'http://localhost:8999/ui/?image=portrait.png&w=300&h=300&wm=true&wm_image=diagram.png&wm_scale=20' },
  ];

  for (let i = 0; i < demoConfigs.length; i++) {
    const cfg = demoConfigs[i];
    console.log(`-> Frame ${i + 1}/${demoConfigs.length}: ${cfg.name}`);
    capture(path.join(framesDir, cfg.name), 1280, 720, cfg.url, 3500);
  }

  // 3. Compile animated demo GIF with ffmpeg
  console.log('3. Compiling docs/images/imgscale_demo.gif with ffmpeg...');
  const gifOut = path.join(imgDir, 'imgscale_demo.gif');
  // 1 frame per 1.5 seconds, with high quality palettegen
  const palette = path.join(framesDir, 'palette.png');
  const inputPattern = path.join(framesDir, 'frame_%02d.png');

  execSync(`ffmpeg -y -framerate 0.65 -i "${inputPattern}" -vf "fps=10,scale=1024:-1:flags=lanczos,palettegen" "${palette}"`, { stdio: 'inherit' });
  execSync(`ffmpeg -y -framerate 0.65 -i "${inputPattern}" -i "${palette}" -filter_complex "fps=10,scale=1024:-1:flags=lanczos[x];[x][1:v]paletteuse" "${gifOut}"`, { stdio: 'inherit' });

  // Clean up frames dir
  fs.rmSync(framesDir, { recursive: true, force: true });
  console.log(`✓ Generated animated demo GIF at ${gifOut}`);
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
