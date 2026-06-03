#!/usr/bin/env node
const fs = require("fs");
const path = require("path");
const cp = require("child_process");

const root = path.resolve(__dirname, "..");
const outDir = path.join(root, ".understand-anything");
const now = new Date().toISOString();

const skip = [
  /^\.git(?:\/|$)/,
  /^\.env$/,
  /^\.env\./,
  /(?:^|\/)node_modules(?:\/|$)/,
  /(?:^|\/)dist(?:\/|$)/,
  /(?:^|\/)build(?:\/|$)/,
  /(?:^|\/)coverage(?:\/|$)/,
  /\.(?:png|jpe?g|gif|ico|woff2?|ttf|eot|pdf|zip|tar|gz|lock|sum)$/i,
];

function run(cmd, args, cwd = root) {
  try {
    return cp.execFileSync(cmd, args, { cwd, encoding: "utf8" }).trim();
  } catch {
    return "";
  }
}

function files() {
  const listed = run("git", ["ls-files"]);
  const all = listed ? listed.split(/\r?\n/) : [];
  return all
    .filter(Boolean)
    .filter((file) => !skip.some((re) => re.test(file)))
    .filter((file) => fs.existsSync(path.join(root, file)))
    .sort();
}

function ext(file) {
  return path.extname(file).toLowerCase();
}

function language(file) {
  const base = path.basename(file);
  if (base === "Dockerfile") return "dockerfile";
  if (base === "Makefile") return "makefile";
  const map = {
    ".js": "javascript",
    ".jsx": "javascript",
    ".ts": "typescript",
    ".tsx": "typescript",
    ".go": "go",
    ".py": "python",
    ".sh": "shell",
    ".sql": "sql",
    ".md": "markdown",
    ".html": "html",
    ".css": "css",
    ".json": "json",
    ".yml": "yaml",
    ".yaml": "yaml",
    ".conf": "config",
  };
  return map[ext(file)] || "unknown";
}

function typeFor(file) {
  const base = path.basename(file);
  if (file.startsWith(".github/")) return "pipeline";
  if (base === "Dockerfile" || file.startsWith("docker-compose")) return "service";
  if (ext(file) === ".md") return "document";
  if ([".json", ".yml", ".yaml", ".conf"].includes(ext(file)) || base === "Makefile" || base === ".gitignore") return "config";
  if (ext(file) === ".sql") return "schema";
  return "file";
}

function categoryFor(file) {
  const t = typeFor(file);
  if (t === "document") return "docs";
  if (t === "config") return "config";
  if (t === "service" || t === "pipeline") return "infra";
  if (t === "schema") return "data";
  if (file.startsWith("scripts/")) return "script";
  if ([".html", ".css"].includes(ext(file))) return "markup";
  return "code";
}

function lineCount(file) {
  const raw = fs.readFileSync(path.join(root, file), "utf8");
  return raw.length ? raw.split(/\r?\n/).length : 0;
}

function summary(file) {
  if (file === "readme.md") return "Project overview, architecture, tech stack, setup instructions, data sources, and feature list.";
  if (file.startsWith("client/")) return "Frontend asset for the React and Vite dashboard experience.";
  if (file.startsWith("server/")) return "Go backend/API component for authentication, storage, queues, cache, and service integration.";
  if (file.startsWith("services/")) return "Worker or service component for ingestion, analytics, scraping, or marketplace integrations.";
  if (file.startsWith("database/")) return "Database schema, migration, seed, or persistence support file.";
  if (file.startsWith("scripts/")) return "Operational script for local setup, health checks, reporting, deployment, or maintenance.";
  if (file.startsWith("docker-compose")) return "Docker Compose orchestration for local, self-hosted, or production services.";
  if (file.startsWith(".github/")) return "CI/CD workflow configuration.";
  return "Project file included in the repository architecture.";
}

function tags(file) {
  const parts = file.split("/");
  const out = new Set([categoryFor(file), language(file)]);
  if (parts.length > 1) out.add(parts[0]);
  if (file.includes("test") || file.includes("smoke")) out.add("test");
  if (file.includes("docker") || file.includes("prometheus") || file.includes("grafana")) out.add("ops");
  return [...out].filter(Boolean);
}

function layerFor(file) {
  if (file.startsWith("client/")) return "frontend";
  if (file.startsWith("server/")) return "backend";
  if (file.startsWith("services/")) return "workers";
  if (file.startsWith("database/")) return "data";
  if (file.startsWith("scripts/")) return "operations";
  if (file.startsWith(".github/") || file.startsWith("docker-compose") || file.includes("prometheus") || file.includes("grafana") || path.basename(file) === "Dockerfile") return "infrastructure";
  if (ext(file) === ".md") return "documentation";
  return "project-config";
}

function parseImports(file, fileSet) {
  const abs = path.join(root, file);
  const text = fs.readFileSync(abs, "utf8");
  const dir = path.dirname(file);
  const refs = new Set();
  const patterns = [
    /import\s+(?:[^'"]+\s+from\s+)?["']([^"']+)["']/g,
    /require\(["']([^"']+)["']\)/g,
    /from\s+["']([^"']+)["']/g,
  ];
  for (const re of patterns) {
    for (const m of text.matchAll(re)) {
      if (!m[1].startsWith(".")) continue;
      const base = path.normalize(path.join(dir, m[1])).replace(/\\/g, "/");
      for (const candidate of [base, `${base}.js`, `${base}.jsx`, `${base}.ts`, `${base}.tsx`, `${base}/index.js`, `${base}/index.jsx`]) {
        if (fileSet.has(candidate)) refs.add(candidate);
      }
    }
  }
  if (ext(file) === ".go") {
    const moduleName = "pokemontool/";
    for (const m of text.matchAll(/"pokemontool\/([^"]+)"/g)) {
      const prefix = m[0].slice(1, -1).replace(moduleName, "");
      for (const candidate of fileSet) {
        if (candidate.startsWith(prefix) && candidate.endsWith(".go")) refs.add(candidate);
      }
    }
  }
  return [...refs].filter((r) => r !== file);
}

const projectFiles = files();
const fileSet = new Set(projectFiles);
const nodes = projectFiles.map((file) => ({
  id: `${typeFor(file)}:${file}`,
  type: typeFor(file),
  name: path.basename(file),
  filePath: file,
  summary: summary(file),
  language: language(file),
  complexity: lineCount(file) > 400 ? "complex" : lineCount(file) > 150 ? "moderate" : "simple",
  tags: tags(file),
}));

const idFor = (file) => `${typeFor(file)}:${file}`;
const edges = [];
const addEdge = (source, target, type, weight = 0.6, description = "") => {
  if (!source || !target || source === target) return;
  const key = `${source}|${target}|${type}`;
  if (edges.some((e) => `${e.source}|${e.target}|${e.type}` === key)) return;
  edges.push({ source, target, type, weight, description });
};

for (const file of projectFiles) {
  for (const imported of parseImports(file, fileSet)) addEdge(idFor(file), idFor(imported), "imports", 0.7, "Local source import.");
  if (file !== "readme.md" && ext(file) === ".md") addEdge("document:readme.md", idFor(file), "documents", 0.5, "README points readers into project documentation.");
  if (file.startsWith("docker-compose") && fileSet.has("server/Dockerfile")) addEdge(idFor(file), "service:server/Dockerfile", "deploys", 0.7, "Compose configuration builds or runs backend services.");
  if (file.startsWith("docker-compose") && fileSet.has("client/Dockerfile")) addEdge(idFor(file), "service:client/Dockerfile", "deploys", 0.7, "Compose configuration builds or runs frontend services.");
  if (file.startsWith("scripts/") && fileSet.has("docker-compose.yml")) addEdge(idFor(file), "service:docker-compose.yml", "configures", 0.6, "Operational script interacts with local stack orchestration.");
}

const layerInfo = {
  frontend: ["Frontend", "React/Vite client dashboard and browser-facing assets."],
  backend: ["Backend", "Go API and backend service implementation."],
  workers: ["Workers", "Marketplace ingestion, analytics, and scraping services."],
  data: ["Data", "Database migrations, schemas, and seed assets."],
  operations: ["Operations", "Setup, reporting, health check, backup, and deployment scripts."],
  infrastructure: ["Infrastructure", "Docker, CI, monitoring, and runtime orchestration configuration."],
  documentation: ["Documentation", "Guides and project reference material."],
  "project-config": ["Project Config", "Top-level project configuration and miscellaneous repository metadata."],
};

const layers = Object.entries(layerInfo).map(([id, [name, description]]) => ({
  id: `layer:${id}`,
  name,
  description,
  nodeIds: nodes.filter((node) => layerFor(node.filePath) === id).map((node) => node.id),
})).filter((layer) => layer.nodeIds.length);

const tour = [
  {
    order: 1,
    title: "Project Overview",
    description: "Start with the README to understand the PokémonTool product, stack, and intended full-stack architecture.",
    nodeIds: ["document:readme.md"].filter((id) => nodes.some((n) => n.id === id)),
  },
  {
    order: 2,
    title: "Frontend Dashboard",
    description: "Review the Vite/React client package and source files that deliver the user-facing dashboard.",
    nodeIds: nodes.filter((n) => n.filePath === "client/package.json" || n.filePath.startsWith("client/src/")).slice(0, 8).map((n) => n.id),
  },
  {
    order: 3,
    title: "Backend Services",
    description: "Inspect the Go backend module and service files that provide APIs, persistence, queue, and cache integration.",
    nodeIds: nodes.filter((n) => n.filePath === "server/go.mod" || n.filePath.startsWith("server/")).slice(0, 8).map((n) => n.id),
  },
  {
    order: 4,
    title: "Data And Workers",
    description: "Follow database assets and worker services that support marketplace ingestion, analytics, and scraping.",
    nodeIds: nodes.filter((n) => n.filePath.startsWith("database/") || n.filePath.startsWith("services/")).slice(0, 10).map((n) => n.id),
  },
  {
    order: 5,
    title: "Operations",
    description: "Finish with Docker Compose, monitoring config, and scripts that start, verify, back up, and deploy the stack.",
    nodeIds: nodes.filter((n) => n.filePath.startsWith("docker-compose") || n.filePath.startsWith("scripts/")).slice(0, 10).map((n) => n.id),
  },
].filter((step) => step.nodeIds.length);

const graph = {
  version: "1.0.0",
  project: {
    name: "PokémonTool",
    languages: [...new Set(nodes.map((n) => n.language))].filter((l) => l !== "unknown").sort(),
    frameworks: ["React", "Vite", "Go chi", "Docker Compose", "PostgreSQL", "Redis", "RabbitMQ"],
    description: "Real-time Pokémon card vendor intelligence platform with dashboard, marketplace ingestion, analytics, alerts, and operational tooling.",
    analyzedAt: now,
    gitCommitHash: run("git", ["rev-parse", "HEAD"]),
  },
  nodes,
  edges,
  layers,
  tour,
};

fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(path.join(outDir, "knowledge-graph.json"), JSON.stringify(graph, null, 2));
fs.writeFileSync(path.join(outDir, "meta.json"), JSON.stringify({
  lastAnalyzedAt: now,
  gitCommitHash: graph.project.gitCommitHash,
  version: "1.0.0",
  analyzedFiles: nodes.length,
  generatedBy: "scripts/ua_fallback_graph_builder.js",
}, null, 2));

const stats = {
  files: nodes.length,
  edges: edges.length,
  layers: layers.length,
  tourSteps: tour.length,
  categories: nodes.reduce((acc, node) => {
    const cat = categoryFor(node.filePath);
    acc[cat] = (acc[cat] || 0) + 1;
    return acc;
  }, {}),
  nodeTypes: nodes.reduce((acc, node) => {
    acc[node.type] = (acc[node.type] || 0) + 1;
    return acc;
  }, {}),
};
console.log(JSON.stringify(stats, null, 2));
