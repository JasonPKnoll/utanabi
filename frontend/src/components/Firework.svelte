<script lang="ts">
  import { rays, tips } from "../lib/firework";

  // The visualizer peony, animated: the loader for long waits, such as building a song. It has no background of its
  // own and takes its colours from the theme, so it sits on any screen, light or dark. With reduced motion on it holds
  // still.
  let { size = 200 }: { size?: number } = $props();

  const uid = $props.id();
  const id = `fw-${uid}`;
  // The hand-drawn boil is an SVG animation, which CSS can't pause, so it's left out for reduced motion.
  const still = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
</script>

<div class="stage" style:width="{size}px" style:height="{size}px" aria-hidden="true">
  <svg width={size} height={size} viewBox="-100 -100 200 200">
    <defs>
      <!-- Wobbles every line a little, a different way each tenth of a second, and adds a soft glow. -->
      <filter id="{id}-rough" x="-30%" y="-30%" width="160%" height="160%">
        <feTurbulence type="fractalNoise" baseFrequency="0.8" numOctaves="2" seed="6" result="noise">
          {#if !still}
            <animate attributeName="seed" values="6;1;9;3" dur="0.4s" repeatCount="indefinite" calcMode="discrete" />
          {/if}
        </feTurbulence>
        <feDisplacementMap
          in="SourceGraphic"
          in2="noise"
          scale="2.2"
          xChannelSelector="R"
          yChannelSelector="G"
          result="rough"
        />
        <feGaussianBlur in="rough" stdDeviation="1.5" result="glow" />
        <feMerge><feMergeNode in="glow" /><feMergeNode in="rough" /></feMerge>
      </filter>
    </defs>

    <g class="breathe">
      <g class="spin">
        <g filter="url(#{id}-rough)" stroke-linecap="round">
          {#each rays as ray, i (i)}
            <g transform="rotate({ray.angle})">
              <line
                class="ray c{ray.colour}"
                x1="0"
                y1="-16"
                x2="0"
                y2={-16 - ray.length}
                stroke-width={ray.width}
                pathLength="100"
                style:animation-duration="{ray.duration}s"
                style:animation-delay="-{ray.offset}s"
              />
            </g>
          {/each}
        </g>
        <g class="tips" filter="url(#{id}-rough)">
          {#each tips as tip, i (i)}
            <circle class="t{tip.colour}" cx={tip.x} cy={tip.y} r={tip.r} style:animation-delay="-{tip.offset}s" />
          {/each}
        </g>
      </g>
    </g>

    <g class="record">
      <circle class="vinyl" r="13" />
      <circle class="groove" r="10.5" />
      <circle class="groove" r="8" />
      <circle class="label" r="5" />
      <rect class="shine" x="-0.6" y="-5" width="1.2" height="2.4" />
      <circle class="hole" r="1" />
    </g>
  </svg>
</div>

<style>
  .stage {
    flex: none;
  }

  svg {
    display: block;
  }

  .ray {
    stroke-dasharray: 100 100;
    stroke-dashoffset: 30;
    animation-name: jump;
    animation-timing-function: ease-in-out;
    animation-iteration-count: infinite;
  }

  .c0 {
    stroke: var(--link);
  }

  .c1 {
    stroke: var(--link-strong);
  }

  .c2 {
    stroke: var(--spark);
  }

  .t0 {
    fill: var(--fg);
  }

  .t1 {
    fill: var(--link-strong);
  }

  .t2 {
    fill: var(--spark);
  }

  .tips circle {
    animation: twinkle 0.4s steps(2) infinite;
  }

  .spin,
  .breathe,
  .record {
    transform-origin: 0 0;
  }

  .spin {
    animation: spin 16s linear infinite;
  }

  /* A kick on every beat, at 120 bpm. */
  .breathe {
    animation: kick 0.5s cubic-bezier(0.2, 0.8, 0.3, 1) infinite;
  }

  .record {
    animation: spin 1.8s linear infinite;
  }

  .vinyl {
    fill: var(--surface-strong);
    stroke: var(--border);
    stroke-width: 0.6;
  }

  .groove {
    fill: none;
    stroke: var(--muted);
    stroke-opacity: 0.35;
    stroke-width: 0.5;
  }

  .label {
    fill: var(--spark);
  }

  .shine {
    fill: var(--bg);
  }

  .hole {
    fill: var(--bg);
  }

  /* The ray's dash grows out from the record: a smaller offset shows more of it. */
  @keyframes jump {
    0%,
    100% {
      stroke-dashoffset: 72;
    }
    50% {
      stroke-dashoffset: 4;
    }
  }

  @keyframes twinkle {
    to {
      opacity: 0.1;
    }
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @keyframes kick {
    0% {
      transform: scale(1.05);
    }
    40%,
    100% {
      transform: scale(1);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .ray,
    .tips circle,
    .spin,
    .breathe,
    .record {
      animation: none;
    }
  }
</style>
