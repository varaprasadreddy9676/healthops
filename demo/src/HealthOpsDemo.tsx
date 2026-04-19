import {
  AbsoluteFill,
  Sequence,
  Video,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
  interpolate,
  spring,
  Easing,
} from "remotion";

const FPS = 30;
const INTRO_DURATION = 4 * FPS; // 4 seconds
const RECORDING_DURATION = 55 * FPS; // 55 seconds
const OUTRO_DURATION = 4 * FPS; // 4 seconds

// Scene config: label, caption, timing, and optional zoom target
// zoom: { scale, originX%, originY%, startSec (relative to scene), durationSec }
const SCENES: {
  label: string;
  caption: string;
  time: number;
  duration?: number;
  zoom?: { scale: number; originX: number; originY: number; start: number; dur: number };
}[] = [
  {
    label: "Dashboard",
    caption: "Real-time overview of all your infrastructure health",
    time: 0,
    duration: 9,
    zoom: { scale: 1.4, originX: 65, originY: 25, start: 3, dur: 4 }, // zoom into status cards
  },
  {
    label: "Health Checks",
    caption: "7 check types: API, TCP, process, command, log, MySQL, SSH",
    time: 10,
    duration: 3,
    zoom: { scale: 1.5, originX: 55, originY: 40, start: 1, dur: 2 }, // zoom into check list
  },
  {
    label: "Check Detail",
    caption: "Drill into any check — response times, history, and status",
    time: 14,
    duration: 4,
    zoom: { scale: 1.6, originX: 60, originY: 35, start: 1, dur: 2.5 }, // zoom into chart
  },
  {
    label: "Servers",
    caption: "Manage and monitor remote servers with SSH-based checks",
    time: 19,
    duration: 2,
  },
  {
    label: "Incidents",
    caption: "Auto-created incidents with full lifecycle tracking",
    time: 22,
    duration: 2,
  },
  {
    label: "Analytics",
    caption: "Uptime trends, response time percentiles, and failure rates",
    time: 25,
    duration: 6,
    zoom: { scale: 1.5, originX: 55, originY: 50, start: 2, dur: 3 }, // zoom into charts
  },
  {
    label: "AI Analysis",
    caption: "BYOK — bring your own key from OpenAI, Anthropic, Google, or Ollama",
    time: 32,
    duration: 3,
    zoom: { scale: 1.4, originX: 55, originY: 40, start: 0.5, dur: 2 },
  },
  {
    label: "MySQL Monitoring",
    caption: "Deep MySQL monitoring — connections, queries, threads, replication",
    time: 36,
    duration: 2,
  },
  {
    label: "Settings",
    caption: "Configure checks, alerts, notifications, and integrations",
    time: 39,
    duration: 5,
    zoom: { scale: 1.3, originX: 55, originY: 35, start: 1.5, dur: 2.5 },
  },
  {
    label: "Dark Mode",
    caption: "Beautiful dark mode with seamless toggle",
    time: 45,
    duration: 9,
    zoom: { scale: 1.3, originX: 55, originY: 30, start: 3, dur: 4 }, // zoom into dark dashboard
  },
];

/* ── Intro Title Card ─────────────────────────────────── */
const Intro: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const titleY = spring({ frame, fps, from: 40, to: 0, durationInFrames: 30 });
  const titleOpacity = interpolate(frame, [0, 20], [0, 1], { extrapolateRight: "clamp" });
  const subtitleOpacity = interpolate(frame, [20, 45], [0, 1], { extrapolateRight: "clamp" });
  const taglineOpacity = interpolate(frame, [40, 65], [0, 1], { extrapolateRight: "clamp" });

  // Fade out at the end
  const fadeOut = interpolate(frame, [INTRO_DURATION - 20, INTRO_DURATION], [1, 0], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill
      style={{
        background: "linear-gradient(135deg, #0f172a 0%, #1e3a5f 50%, #0f172a 100%)",
        justifyContent: "center",
        alignItems: "center",
        opacity: fadeOut,
      }}
    >
      {/* Animated grid background */}
      <div
        style={{
          position: "absolute",
          inset: 0,
          backgroundImage:
            "linear-gradient(rgba(59,130,246,0.08) 1px, transparent 1px), linear-gradient(90deg, rgba(59,130,246,0.08) 1px, transparent 1px)",
          backgroundSize: "60px 60px",
        }}
      />

      {/* Glowing orb */}
      <div
        style={{
          position: "absolute",
          width: 400,
          height: 400,
          borderRadius: "50%",
          background: "radial-gradient(circle, rgba(59,130,246,0.3) 0%, transparent 70%)",
          filter: "blur(60px)",
          transform: `scale(${interpolate(frame, [0, INTRO_DURATION], [0.8, 1.3])})`,
        }}
      />

      <div style={{ textAlign: "center", zIndex: 1 }}>
        {/* Logo / Icon */}
        <div
          style={{
            fontSize: 64,
            marginBottom: 20,
            opacity: titleOpacity,
            transform: `translateY(${titleY}px)`,
          }}
        >
          💙
        </div>

        {/* Title */}
        <h1
          style={{
            fontSize: 72,
            fontWeight: 800,
            color: "white",
            margin: 0,
            letterSpacing: "-2px",
            opacity: titleOpacity,
            transform: `translateY(${titleY}px)`,
            fontFamily: "system-ui, -apple-system, sans-serif",
          }}
        >
          HealthOps
        </h1>

        {/* Subtitle */}
        <p
          style={{
            fontSize: 28,
            color: "#93c5fd",
            marginTop: 16,
            opacity: subtitleOpacity,
            fontWeight: 500,
            fontFamily: "system-ui, -apple-system, sans-serif",
          }}
        >
          Open-Source Infrastructure Monitoring
        </p>

        {/* Tagline */}
        <div
          style={{
            display: "flex",
            gap: 24,
            marginTop: 32,
            opacity: taglineOpacity,
            justifyContent: "center",
          }}
        >
          {["AI-Powered", "Real-Time", "Beautiful UI"].map((tag) => (
            <span
              key={tag}
              style={{
                padding: "8px 20px",
                borderRadius: 20,
                background: "rgba(59,130,246,0.2)",
                border: "1px solid rgba(59,130,246,0.3)",
                color: "#93c5fd",
                fontSize: 16,
                fontWeight: 500,
                fontFamily: "system-ui, -apple-system, sans-serif",
              }}
            >
              {tag}
            </span>
          ))}
        </div>
      </div>
    </AbsoluteFill>
  );
};

/* ── Scene Label Overlay ─────────────────────────────── */
const SceneLabel: React.FC<{ label: string }> = ({ label }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const slideIn = spring({ frame, fps, from: -60, to: 0, durationInFrames: 15 });
  const opacity = interpolate(frame, [0, 10, 50, 65], [0, 1, 1, 0], {
    extrapolateRight: "clamp",
  });

  return (
    <div
      style={{
        position: "absolute",
        bottom: 90,
        left: 40,
        opacity,
        transform: `translateX(${slideIn}px)`,
        zIndex: 10,
      }}
    >
      <div
        style={{
          background: "rgba(15, 23, 42, 0.85)",
          backdropFilter: "blur(12px)",
          borderRadius: 12,
          padding: "10px 22px",
          border: "1px solid rgba(59,130,246,0.4)",
          boxShadow: "0 8px 32px rgba(0,0,0,0.3)",
        }}
      >
        <span
          style={{
            color: "#93c5fd",
            fontSize: 13,
            fontWeight: 600,
            letterSpacing: "0.08em",
            textTransform: "uppercase",
            fontFamily: "system-ui, -apple-system, sans-serif",
          }}
        >
          ▶ {label}
        </span>
      </div>
    </div>
  );
};

/* ── Caption Bar ─────────────────────────────────────── */
const Caption: React.FC<{ text: string }> = ({ text }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const fadeIn = interpolate(frame, [0, 15], [0, 1], { extrapolateRight: "clamp" });
  const fadeOut = interpolate(frame, [60, 75], [1, 0], { extrapolateRight: "clamp" });
  const opacity = Math.min(fadeIn, fadeOut);
  const slideUp = spring({ frame, fps, from: 20, to: 0, durationInFrames: 18 });

  return (
    <div
      style={{
        position: "absolute",
        bottom: 0,
        left: 0,
        right: 0,
        zIndex: 20,
        display: "flex",
        justifyContent: "center",
        opacity,
        transform: `translateY(${slideUp}px)`,
      }}
    >
      <div
        style={{
          background: "linear-gradient(180deg, transparent 0%, rgba(0,0,0,0.75) 100%)",
          width: "100%",
          padding: "40px 60px 24px",
          textAlign: "center",
        }}
      >
        <p
          style={{
            color: "white",
            fontSize: 22,
            fontWeight: 500,
            margin: 0,
            textShadow: "0 2px 8px rgba(0,0,0,0.6)",
            fontFamily: "system-ui, -apple-system, sans-serif",
            lineHeight: 1.4,
          }}
        >
          {text}
        </p>
      </div>
    </div>
  );
};

/* ── Auto-Zoom Wrapper ───────────────────────────────── */
const AutoZoomVideo: React.FC = () => {
  const frame = useCurrentFrame();

  // Build zoom keyframes from SCENES
  let scale = 1;
  let originX = 50;
  let originY = 50;

  for (const scene of SCENES) {
    if (!scene.zoom) continue;
    const z = scene.zoom;
    const zoomStartFrame = scene.time * FPS + z.start * FPS;
    const zoomPeakFrame = zoomStartFrame + (z.dur * FPS) / 2;
    const zoomEndFrame = zoomStartFrame + z.dur * FPS;

    if (frame >= zoomStartFrame && frame <= zoomEndFrame) {
      // Ease in to peak, ease out back to 1
      if (frame <= zoomPeakFrame) {
        const progress = interpolate(frame, [zoomStartFrame, zoomPeakFrame], [0, 1], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
        });
        // Smooth ease (sine)
        const eased = 0.5 - 0.5 * Math.cos(progress * Math.PI);
        scale = 1 + (z.scale - 1) * eased;
        originX = 50 + (z.originX - 50) * eased;
        originY = 50 + (z.originY - 50) * eased;
      } else {
        const progress = interpolate(frame, [zoomPeakFrame, zoomEndFrame], [0, 1], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
        });
        const eased = 0.5 - 0.5 * Math.cos(progress * Math.PI);
        scale = z.scale - (z.scale - 1) * eased;
        originX = z.originX - (z.originX - 50) * eased;
        originY = z.originY - (z.originY - 50) * eased;
      }
      break;
    }
  }

  return (
    <AbsoluteFill
      style={{
        transformOrigin: `${originX}% ${originY}%`,
        transform: `scale(${scale})`,
        willChange: "transform",
      }}
    >
      <Video
        src={staticFile("healthops-raw.webm")}
        style={{ width: "100%", height: "100%" }}
      />
    </AbsoluteFill>
  );
};

/* ── Outro Card ──────────────────────────────────────── */
const Outro: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const fadeIn = interpolate(frame, [0, 20], [0, 1], { extrapolateRight: "clamp" });
  const scaleIn = spring({ frame, fps, from: 0.9, to: 1, durationInFrames: 25 });

  return (
    <AbsoluteFill
      style={{
        background: "linear-gradient(135deg, #0f172a 0%, #1e3a5f 50%, #0f172a 100%)",
        justifyContent: "center",
        alignItems: "center",
        opacity: fadeIn,
      }}
    >
      {/* Grid background */}
      <div
        style={{
          position: "absolute",
          inset: 0,
          backgroundImage:
            "linear-gradient(rgba(59,130,246,0.08) 1px, transparent 1px), linear-gradient(90deg, rgba(59,130,246,0.08) 1px, transparent 1px)",
          backgroundSize: "60px 60px",
        }}
      />

      <div style={{ textAlign: "center", zIndex: 1, transform: `scale(${scaleIn})` }}>
        <h1
          style={{
            fontSize: 56,
            fontWeight: 800,
            color: "white",
            margin: 0,
            letterSpacing: "-1px",
            fontFamily: "system-ui, -apple-system, sans-serif",
          }}
        >
          Get Started
        </h1>

        <div
          style={{
            marginTop: 28,
            padding: "14px 32px",
            borderRadius: 12,
            background: "rgba(59,130,246,0.15)",
            border: "1px solid rgba(59,130,246,0.3)",
          }}
        >
          <code
            style={{
              color: "#93c5fd",
              fontSize: 24,
              fontFamily: "'SF Mono', 'Fira Code', monospace",
            }}
          >
            docker compose up -d
          </code>
        </div>

        <p
          style={{
            marginTop: 24,
            color: "#64748b",
            fontSize: 20,
            fontFamily: "system-ui, -apple-system, sans-serif",
          }}
        >
          github.com/your-org/healthops
        </p>

        <div
          style={{
            display: "flex",
            gap: 16,
            marginTop: 24,
            justifyContent: "center",
            opacity: interpolate(frame, [30, 50], [0, 1], { extrapolateRight: "clamp" }),
          }}
        >
          {["⭐ Star", "🍴 Fork", "📖 Docs"].map((action) => (
            <span
              key={action}
              style={{
                padding: "8px 20px",
                borderRadius: 8,
                background: "rgba(255,255,255,0.05)",
                border: "1px solid rgba(255,255,255,0.1)",
                color: "#94a3b8",
                fontSize: 16,
                fontFamily: "system-ui, -apple-system, sans-serif",
              }}
            >
              {action}
            </span>
          ))}
        </div>
      </div>
    </AbsoluteFill>
  );
};

/* ── Main Composition ────────────────────────────────── */
export const HealthOpsDemo: React.FC = () => {
  return (
    <AbsoluteFill style={{ background: "#000" }}>
      {/* Intro */}
      <Sequence from={0} durationInFrames={INTRO_DURATION}>
        <Intro />
      </Sequence>

      {/* Screen Recording with auto-zoom */}
      <Sequence from={INTRO_DURATION} durationInFrames={RECORDING_DURATION}>
        <AutoZoomVideo />

        {/* Scene Labels */}
        {SCENES.map((scene) => (
          <Sequence
            key={scene.label}
            from={scene.time * FPS}
            durationInFrames={3 * FPS}
          >
            <SceneLabel label={scene.label} />
          </Sequence>
        ))}

        {/* Captions */}
        {SCENES.map((scene) => (
          <Sequence
            key={`cap-${scene.label}`}
            from={scene.time * FPS}
            durationInFrames={(scene.duration ?? 3) * FPS}
          >
            <Caption text={scene.caption} />
          </Sequence>
        ))}
      </Sequence>

      {/* Outro */}
      <Sequence
        from={INTRO_DURATION + RECORDING_DURATION}
        durationInFrames={OUTRO_DURATION}
      >
        <Outro />
      </Sequence>
    </AbsoluteFill>
  );
};
