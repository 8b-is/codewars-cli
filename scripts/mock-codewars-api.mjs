/**
 * mock-codewars-api.mjs — local stand-in for the Codewars API v1 used to
 * exercise `codewars webhook --record` and `codewars bender` without hitting
 * the real platform (replay / CI / offline).
 *
 * Usage:
 *   node scripts/mock-codewars-api.mjs [--port 8801] [--completed fixtures/completed.json]
 *
 * Serves:
 *   GET /users/:user/code-challenges/completed?page=N
 *   GET /code-challenges/:id
 */
import { createServer } from "node:http";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const args = process.argv.slice(2);
const flag = (name, fallback) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : fallback;
};
const port = Number(flag("port", "8801"));
const completedPath = flag(
  "completed",
  fileURLToPath(new URL("../fixtures/completed.sample.json", import.meta.url)),
);

const completed = JSON.parse(readFileSync(completedPath, "utf8"));
const challenges = new Map();
for (const entry of completed.data ?? []) {
  challenges.set(entry.id, {
    id: entry.id,
    name: entry.name,
    slug: entry.slug,
    url: `https://www.codewars.com/kata/${entry.id}`,
    category: "algorithms",
    languages: entry.completedLanguages ?? [],
    rank: { id: -7, name: "7 kyu", color: "white" },
    tags: [],
    description: `(mock) ${entry.name}`,
    totalAttempts: 0,
    totalCompleted: 0,
  });
}

const json = (res, status, body) => {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(body));
};

createServer((req, res) => {
  const url = new URL(req.url, `http://${req.headers.host}`);
  const parts = url.pathname.split("/").filter(Boolean);

  if (parts[0] === "users" && parts[2] === "code-challenges" && parts[3] === "completed") {
    return json(res, 200, completed);
  }
  if (parts[0] === "users" && parts[1]) {
    return json(res, 200, {
      id: "mock-user",
      username: parts[1],
      honor: 0,
      ranks: { overall: { rank: -3, name: "3 kyu", color: "blue", score: 1 }, languages: {} },
      codeChallenges: { totalAuthored: 0, totalCompleted: completed.data?.length ?? 0 },
    });
  }
  if (parts[0] === "code-challenges" && parts[1]) {
    const ch = challenges.get(parts[1]);
    if (!ch) return json(res, 404, { success: false, reason: "not found" });
    return json(res, 200, ch);
  }
  json(res, 404, { success: false, reason: "not found" });
}).listen(port, "127.0.0.1", () => {
  console.log(`mock codewars api on http://127.0.0.1:${port} (${completed.data?.length ?? 0} completed entries)`);
});