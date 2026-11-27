// The visualizer peony, Utanabi's mark: a firework whose rays jump like a sound spectrum around a spinning record.
// The rays were scattered once with a seeded random pick, so the angles, lengths, widths, and rhythms are uneven on
// purpose, and the same on every screen.

export interface Ray {
  // Degrees clockwise from straight up.
  angle: number;
  // How far the ray reaches past the record, in the 200-unit drawing.
  length: number;
  width: number;
  // 0 the accent, 1 the strong accent, 2 the spark.
  colour: 0 | 1 | 2;
  // Seconds for one jump, and how far into it the ray starts, so no two move together.
  duration: number;
  offset: number;
}

// [angle, length, width, colour, duration, offset]
const table: [number, number, number, Ray["colour"], number, number][] = [
  [-0.1, 59.7, 1, 0, 0.47, 0.38],
  [7.4, 43.7, 1.2, 2, 0.61, 0.52],
  [15.4, 50.5, 1.5, 1, 0.38, 0.13],
  [22.9, 41.4, 1.5, 0, 0.7, 0.18],
  [34.8, 61.9, 1.5, 2, 0.61, 0.11],
  [39.4, 38.4, 1.2, 1, 0.7, 0.07],
  [47.8, 42.7, 1, 0, 0.38, 0.6],
  [56.8, 46.8, 1, 2, 0.61, 0.22],
  [66.7, 49.0, 1.2, 1, 0.47, 0.55],
  [73.8, 56.5, 1, 0, 0.38, 0.22],
  [80.6, 46.4, 1.5, 2, 0.61, 0.76],
  [91.7, 58.7, 0.8, 1, 0.47, 0.03],
  [99.2, 59.3, 1.5, 0, 0.7, 0.39],
  [104.2, 60.2, 1.5, 2, 0.61, 0.2],
  [112.6, 40.0, 0.8, 1, 0.79, 0.56],
  [124.5, 59.3, 0.8, 0, 0.7, 0.62],
  [128.4, 39.3, 0.8, 2, 0.79, 0.57],
  [141.4, 52.3, 1.5, 1, 0.79, 0.61],
  [145.3, 43.8, 1, 0, 0.61, 0.21],
  [156.9, 36.0, 1.2, 2, 0.61, 0.22],
  [162.7, 57.8, 1, 1, 0.61, 0.42],
  [172.1, 36.8, 1.5, 0, 0.79, 0.52],
  [177.8, 41.0, 1.5, 2, 0.38, 0.21],
  [189.0, 60.1, 1, 1, 0.61, 0.03],
  [195.6, 46.9, 1.5, 0, 0.47, 0.01],
  [203.5, 58.0, 0.8, 2, 0.47, 0.4],
  [211.2, 55.9, 1, 1, 0.47, 0.37],
  [219.7, 59.1, 0.8, 0, 0.7, 0.4],
  [227.5, 41.8, 1.5, 2, 0.79, 0.05],
  [237.7, 60.0, 0.8, 1, 0.47, 0.02],
  [245.9, 46.8, 0.8, 0, 0.47, 0.31],
  [255.6, 59.0, 0.8, 2, 0.38, 0.75],
  [261.0, 40.8, 1.5, 1, 0.38, 0.25],
  [271.1, 57.8, 1.2, 0, 0.61, 0.14],
  [275.7, 43.3, 1.2, 2, 0.61, 0.76],
  [284.5, 61.1, 1, 1, 0.61, 0.29],
  [296.2, 57.4, 1.5, 0, 0.38, 0.04],
  [302.6, 45.7, 1.5, 2, 0.47, 0.26],
  [312.1, 48.3, 1.5, 1, 0.47, 0.65],
  [320.4, 37.1, 0.8, 0, 0.61, 0.05],
  [329.4, 42.7, 0.8, 2, 0.7, 0.27],
  [334.3, 60.9, 0.8, 1, 0.53, 0.6],
  [344.6, 60.0, 1.2, 0, 0.38, 0.58],
  [352.3, 56.9, 0.8, 2, 0.38, 0.66],
];

export const rays: Ray[] = table.map(([angle, length, width, colour, duration, offset]) => ({
  angle,
  length,
  width,
  colour,
  duration,
  offset,
}));

export interface Tip {
  x: number;
  y: number;
  r: number;
  // 0 the text colour, 1 the strong accent, 2 the spark.
  colour: 0 | 1 | 2;
  offset: number;
}

// The glitter just past the rays, every 30 degrees, with four smaller ones between.
export const tips: Tip[] = [
  { x: 0, y: -86, r: 1.2, colour: 0, offset: 0.1 },
  { x: 43, y: -74.5, r: 0.8, colour: 1, offset: 0.3 },
  { x: 74.5, y: -43, r: 1.3, colour: 2, offset: 0.2 },
  { x: 86, y: 0, r: 0.9, colour: 0, offset: 0.05 },
  { x: 74.5, y: 43, r: 1.1, colour: 1, offset: 0.25 },
  { x: 43, y: 74.5, r: 1.3, colour: 0, offset: 0.15 },
  { x: 0, y: 86, r: 0.8, colour: 2, offset: 0.35 },
  { x: -43, y: 74.5, r: 1.2, colour: 1, offset: 0.1 },
  { x: -74.5, y: 43, r: 0.9, colour: 0, offset: 0.3 },
  { x: -86, y: 0, r: 1.3, colour: 2, offset: 0.2 },
  { x: -74.5, y: -43, r: 0.8, colour: 1, offset: 0.05 },
  { x: -43, y: -74.5, r: 1.1, colour: 0, offset: 0.25 },
  { x: 22, y: -90, r: 0.7, colour: 1, offset: 0.18 },
  { x: 90, y: 22, r: 0.7, colour: 2, offset: 0.28 },
  { x: -22, y: 90, r: 0.7, colour: 0, offset: 0.08 },
  { x: -90, y: -22, r: 0.7, colour: 1, offset: 0.33 },
];
