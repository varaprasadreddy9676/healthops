# HealthOps Demo Video

This directory contains the tools to record and produce the HealthOps demo video.

## How it works

1. **Playwright** records a browser session navigating through the app
2. **React Remotion** adds an animated intro, scene labels, and outro
3. Final MP4 is output to `out/healthops-demo.mp4`

## Prerequisites

- Node.js 18+
- ffmpeg (`brew install ffmpeg`)
- HealthOps running on `localhost:8080`

## Re-record

```bash
# 1. Start HealthOps
cd ../backend && FRONTEND_DIR=../frontend/dist go run ./cmd/healthops &

# 2. Record the raw browser video
node record-demo.mjs

# 3. Copy recording to Remotion's public dir
cp recordings/healthops-raw.webm public/

# 4. Render the final edited video
npx remotion render src/index.ts HealthOpsDemo out/healthops-demo.mp4 --codec h264

# 5. Copy to docs
cp out/healthops-demo.mp4 ../docs/assets/
```

## Customizing

- Edit `record-demo.mjs` to change which pages are visited and timing
- Edit `src/HealthOpsDemo.tsx` to customize intro/outro cards, scene labels, and transitions
- Edit `src/Root.tsx` to change resolution or FPS
