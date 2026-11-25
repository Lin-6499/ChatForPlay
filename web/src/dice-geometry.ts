// Small, orthographically projected solids. Geometry is decorative: only the
// server receipt supplies a displayed point value.
type Point = [number, number, number]
type Face = { indices: number[]; normal: Point }
const dot = (a: Point, b: Point) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2]
const sub = (a: Point, b: Point): Point => [a[0] - b[0], a[1] - b[1], a[2] - b[2]]
const cross = (a: Point, b: Point): Point => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]]
const unit = (v: Point): Point => { const length = Math.hypot(...v); return v.map(n => n / length) as Point }
const center = (points: Point[]): Point => [0, 1, 2].map(i => points.reduce((sum, p) => sum + p[i], 0) / points.length) as Point

function hull(vertices: Point[]): Face[] {
  const found = new Map<string, Face>()
  for (let a = 0; a < vertices.length; a++) for (let b = a + 1; b < vertices.length; b++) for (let c = b + 1; c < vertices.length; c++) {
    let normal = cross(sub(vertices[b], vertices[a]), sub(vertices[c], vertices[a]))
    if (Math.hypot(...normal) < .0001) continue
    normal = unit(normal)
    if (dot(normal, vertices[a]) < 0) normal = normal.map(n => -n) as Point
    const distances = vertices.map(p => dot(normal, sub(p, vertices[a])))
    if (distances.some(n => n > .0001)) continue
    const indices = distances.flatMap((n, i) => Math.abs(n) < .0001 ? [i] : [])
    const key = indices.join(',')
    if (found.has(key)) continue
    const origin = center(indices.map(i => vertices[i]))
    const u = unit(sub(vertices[indices[0]], origin)), v = cross(normal, u)
    indices.sort((i, j) => Math.atan2(dot(sub(vertices[i], origin), v), dot(sub(vertices[i], origin), u)) - Math.atan2(dot(sub(vertices[j], origin), v), dot(sub(vertices[j], origin), u)))
    found.set(key, { indices, normal })
  }
  return [...found.values()]
}

const phi = (1 + Math.sqrt(5)) / 2
const icosahedron: Point[] = []
for (const a of [-1, 1]) for (const b of [-phi, phi]) icosahedron.push([0, a, b], [a, b, 0], [b, 0, a])
const dual = (vertices: Point[]) => hull(vertices).map(face => {
  const distance = dot(face.normal, vertices[face.indices[0]])
  return face.normal.map(value => value / distance) as Point
})
const antiprism: Point[] = Array.from({ length: 10 }, (_, i) => {
  const angle = i * Math.PI / 5
  return [Math.cos(angle), Math.sin(angle), (i % 2 ? -1 : 1) * .55]
})
const solids = new Map<number, Point[]>([
  [4, [[1, 1, 1], [1, -1, -1], [-1, 1, -1], [-1, -1, 1]]],
  [6, [-1, 1].flatMap(x => [-1, 1].flatMap(y => [-1, 1].map(z => [x, y, z] as Point)))],
  [8, [[1, 0, 0], [-1, 0, 0], [0, 1, 0], [0, -1, 0], [0, 0, 1], [0, 0, -1]]],
  [10, dual(antiprism)], [12, dual(icosahedron)], [20, icosahedron],
])
const meshes = new Map<number, { vertices: Point[]; faces: Face[]; label: Point }>()
for (const [sides, points] of solids) {
  const faces = hull(points), normal = faces[0].normal
  const right = unit(cross(Math.abs(normal[1]) > .9 ? [1, 0, 0] : [0, 1, 0], normal)), up = cross(normal, right)
  const radius = Math.max(...points.map(p => Math.hypot(...p)))
  // Face zero rests toward the viewer; its label can stay readable and upright.
  let vertices = points.map(p => [dot(p, right) / radius, dot(p, up) / radius, dot(p, normal) / radius] as Point)
  if (faces[0].indices.length !== 4 || sides === 10) {
    // Point triangular/pentagonal faces upward. The D10's long kite tip
    // follows the same vertical axis instead of lying on its side.
    const front = faces[0].indices.map(i => vertices[i])
    const top = sides === 10 ? front.reduce((a, b) => Math.hypot(a[0], a[1]) > Math.hypot(b[0], b[1]) ? a : b) : front[0]
    const angle = Math.PI / 2 - Math.atan2(top[1], top[0])
    vertices = vertices.map(([x, y, z]) => [x * Math.cos(angle) - y * Math.sin(angle), x * Math.sin(angle) + y * Math.cos(angle), z])
  }
  const front = faces[0].indices.map(i => vertices[i])
  let label = center(front)
  if (sides === 10) {
    // A kite's vertex average sits too close to its long tip. Its incenter
    // gives the point value equal clearance from all four edges.
    const tip = front.findIndex(p => Math.abs(p[0]) < .00001 && p[1] > 0)
    const a = front[tip], b = front[(tip + 2) % 4], c = front[(tip + 1) % 4]
    const ac = Math.hypot(...sub(a, c)), bc = Math.hypot(...sub(b, c))
    label = a.map((value, i) => (value * bc + b[i] * ac) / (ac + bc)) as Point
  }
  meshes.set(sides, { vertices, faces, label })
}

export function diceSpec(formula: string) {
  const match = formula.trim().toLowerCase().replaceAll(' ', '').match(/^([1-9][0-9]?)d([1-9][0-9]{0,3})(kh1|kl1)?([+-][0-9]{1,5})?$/)
  if (!match) return null
  const count = Number(match[1]), sides = Number(match[2]), modifier = Number(match[4] || 0)
  return count <= 50 && sides >= 2 && sides <= 1000 && Math.abs(modifier) <= 10000
    ? { count, sides, keep: match[3] || '', modifier } : null
}

export function projectDie(sides: number, progress: number, index: number) {
  const mesh = meshes.get(sides === 100 ? 10 : sides) || meshes.get(20)!
  const remaining = 1 - progress
  // Zero residual pitch/yaw: the result face is parallel to the viewport.
  // Orthographic, uniform scaling preserves regular faces without foreshortening.
  const x = remaining ** 3 * Math.PI * (6 + index % 2 * 2), y = remaining ** 3 * Math.PI * 4
  const z = remaining ** 2 * Math.PI * (index % 2 ? -2 : 2)
  const rotate = ([px, py, pz]: Point): Point => {
    const y1 = py * Math.cos(x) - pz * Math.sin(x), z1 = py * Math.sin(x) + pz * Math.cos(x)
    const x2 = px * Math.cos(y) + z1 * Math.sin(y), z2 = -px * Math.sin(y) + z1 * Math.cos(y)
    return [x2 * Math.cos(z) - y1 * Math.sin(z), x2 * Math.sin(z) + y1 * Math.cos(z), z2]
  }
  const vertices = mesh.vertices.map(rotate)
  return mesh.faces.flatMap((face, id) => {
    const points = face.indices.map(i => vertices[i]), mid = center(points)
    const label = id === 0 ? rotate(mesh.label) : mid
    const normal = unit(cross(sub(points[1], points[0]), sub(points[2], points[0])))
    if (normal[2] <= .00001) return []
    return [{
      id, depth: mid[2],
      labelX: 60 + label[0] * 48, labelY: 60 - label[1] * 48,
      points: points.map(p => `${(60 + p[0] * 48).toFixed(2)},${(60 - p[1] * 48).toFixed(2)}`).join(' '),
      light: Math.round(25 + Math.max(0, dot(normal, unit([-.5, .8, 1]))) * 65),
    }]
  }).sort((a, b) => a.depth - b.depth)
}
