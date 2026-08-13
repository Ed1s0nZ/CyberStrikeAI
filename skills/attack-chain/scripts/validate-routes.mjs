#!/usr/bin/env node

/**
 * attack-chain 机械验证器。
 *
 * 校验内容：
 *  1. 自身 SKILL.md frontmatter name；文档中 skill:<name> 引用可解析且 name 一致；
 *     不允许出现第二个 attack-chain 路由器（沿用旧版检查）。
 *  2. references/enterprise-attack-v19.json 固定快照结构（15 个 tactic、technique 表完整）。
 *  3. references/attack-v19-routes.json 注册表结构与必填字段；
 *     ATT&CK ID 必须存在快照且未 deprecated/revoked；sub-technique 父类与 tactic 归属一致；
 *     skill 目录/frontmatter 一致；router/specialist 数量限制；禁止 wildcard；
 *     technique 映射不得产生同优先级歧义；WSTG 分类必须指向真实 specialist。
 *  4. 由注册表反向重算 15 个 tactic 的 supported/gap 覆盖，输出 coverage summary；
 *     gap 不计为已覆盖。--write-coverage 时重写 references/attack-v19-coverage.json。
 *  5. tests/route-cases.json 路由夹具：按文档化路由算法仿真，验证期望 route/specialist、
 *     execution class、must-not-route 与 routing gap 行为。
 *
 * 用法：
 *   node validate-routes.mjs [--skills-root <dir>] [--write-coverage]
 */

import { readFile, readdir, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const skillDir = resolve(scriptDir, "..");
const defaultSkillsRoot = resolve(skillDir, "..");
const args = process.argv.slice(2);
const rootFlag = args.indexOf("--skills-root");
const skillsRoot = rootFlag >= 0 && args[rootFlag + 1]
  ? resolve(args[rootFlag + 1])
  : defaultSkillsRoot;
const writeCoverage = args.includes("--write-coverage");

const SNAPSHOT_ID = "enterprise-attack-19";
const EXPECTED_PHASES = [
  "target-context-init",
  "recon-discovery",
  "threat-modeling",
  "vulnerability-analysis",
  "controlled-exploitation",
  "post-exploitation",
  "cleanup-verification",
  "reporting"
];
const EXPECTED_WSTG = [
  "WSTG-INFO", "WSTG-CONF", "WSTG-IDNT", "WSTG-ATHN", "WSTG-ATHZ", "WSTG-SESS",
  "WSTG-INPV", "WSTG-ERRH", "WSTG-CRYP", "WSTG-BUSL", "WSTG-CLNT", "WSTG-APIT"
];
const EXECUTION_CLASSES = ["fast", "standard", "heavy"];
const GAP_NOTES = {
  TA0003: "不自动执行 Persistence；技能库无自动路由",
  TA0005: "不自动执行 Stealth；技能库无自动路由",
  TA0011: "不自动执行 Command and Control；技能库无自动路由",
  TA0010: "不自动执行 Exfiltration；技能库无自动路由",
  TA0040: "不自动执行 Impact；技能库无自动路由"
};

const errors = [];
const warnings = [];
const fail = (msg) => errors.push(msg);
const warn = (msg) => warnings.push(msg);

function frontmatterName(text) {
  const block = text.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/);
  if (!block) return undefined;
  const match = block[1].match(/^name:\s*([a-z0-9-]+)\s*$/m);
  return match?.[1];
}

async function markdownFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== "scripts" && entry.name !== "tests") files.push(...await markdownFiles(path));
    } else if (entry.isFile() && entry.name.endsWith(".md")) {
      files.push(path);
    }
  }
  return files;
}

async function readJson(path, label) {
  try {
    return JSON.parse(await readFile(path, "utf8"));
  } catch (error) {
    fail(`${label} 不是合法 JSON：${path}（${error.message}）`);
    return null;
  }
}

async function skillNameResolves(name) {
  if (!/^[a-z0-9-]+$/.test(name) || /[*?[\]{}]/.test(name)) {
    return { ok: false, reason: `非法或通配 skill 名：${name}` };
  }
  const target = join(skillsRoot, name, "SKILL.md");
  if (!existsSync(target)) return { ok: false, reason: `skill 不存在：${name}` };
  const declared = frontmatterName(await readFile(target, "utf8"));
  if (declared !== name) return { ok: false, reason: `skill ${name} 目录与 frontmatter name(${declared ?? "缺失"}) 不一致` };
  return { ok: true };
}

/* ---------------------------------------------------------------- 1. 旧版检查 */

const ownText = await readFile(join(skillDir, "SKILL.md"), "utf8");
const ownName = frontmatterName(ownText);
if (ownName !== "attack-chain") fail(`SKILL.md frontmatter name=${ownName ?? "缺失"}，应为 attack-chain`);

const referencedSkills = new Set();
for (const path of await markdownFiles(skillDir)) {
  const text = await readFile(path, "utf8");
  for (const match of text.matchAll(/skill:([a-z0-9-]+)/g)) referencedSkills.add(match[1]);
}
for (const name of [...referencedSkills].sort()) {
  const check = await skillNameResolves(name);
  if (!check.ok) fail(`文档引用 ${check.reason}`);
}

for (const entry of await readdir(skillsRoot, { withFileTypes: true })) {
  if (!entry.isDirectory() || entry.name === "attack-chain") continue;
  const candidate = join(skillsRoot, entry.name, "SKILL.md");
  if (!existsSync(candidate)) continue;
  const declared = frontmatterName(await readFile(candidate, "utf8"));
  if (entry.name.includes("attack-chain") || declared?.includes("attack-chain")) {
    fail(`发现第二个 attack-chain 路由器：${entry.name}（declared=${declared ?? "缺失"}）`);
  }
}

/* ---------------------------------------------------------------- 2. 快照 */

const snapshotPath = join(skillDir, "references", "enterprise-attack-v19.json");
const snapshot = existsSync(snapshotPath) ? await readJson(snapshotPath, "ATT&CK 快照") : null;
if (!snapshot) {
  fail(`缺少 ATT&CK 快照：${snapshotPath}`);
} else {
  if (snapshot.snapshot !== SNAPSHOT_ID) fail(`快照 id=${snapshot.snapshot}，应为 ${SNAPSHOT_ID}`);
  if (!snapshot.source?.url || !/^[0-9a-f]{64}$/.test(snapshot.source?.sha256 ?? "")) {
    fail("快照 source 缺 url 或 sha256（64 位 hex）");
  }
  if (!Array.isArray(snapshot.tactics) || snapshot.tactics.length !== 15) {
    fail(`快照 tactic 数量=${snapshot.tactics?.length}，应为 15`);
  }
  for (const tactic of snapshot.tactics ?? []) {
    if (!/^TA\d{4}$/.test(tactic.id ?? "") || !tactic.name || !tactic.shortname) {
      fail(`快照 tactic 结构不完整：${JSON.stringify(tactic)}`);
    }
  }
  if (!snapshot.techniques || typeof snapshot.techniques !== "object" || Array.isArray(snapshot.techniques)) {
    fail("快照 techniques 必须为以 ID 为键的对象");
  } else {
    for (const [id, tech] of Object.entries(snapshot.techniques)) {
      if (!/^T\d{4}(\.\d{3})?$/.test(id)) fail(`快照 technique ID 非法：${id}`);
      if (!tech.name || !Array.isArray(tech.tactics) || !Array.isArray(tech.platforms)) {
        fail(`快照 technique ${id} 结构不完整`);
      }
      if (typeof tech.deprecated !== "boolean" || typeof tech.revoked !== "boolean") {
        fail(`快照 technique ${id} 缺 deprecated/revoked 布尔标记`);
      }
      if (id.includes(".")) {
        const parent = id.split(".")[0];
        if (tech.parent !== parent) fail(`快照 sub-technique ${id} parent=${tech.parent}，应为 ${parent}`);
        if (!snapshot.techniques[parent]) fail(`快照 sub-technique ${id} 的父类 ${parent} 缺失`);
      }
    }
  }
}

const tacticById = new Map((snapshot?.tactics ?? []).map((t) => [t.id, t]));
const tacticByShort = new Map((snapshot?.tactics ?? []).map((t) => [t.shortname, t]));
const techniqueTable = snapshot?.techniques ?? {};

function techniqueTacticIds(techId) {
  const tech = techniqueTable[techId];
  if (!tech) return [];
  return tech.tactics.map((short) => tacticByShort.get(short)?.id).filter(Boolean);
}

/* ---------------------------------------------------------------- 3. 注册表 */

const registryPath = join(skillDir, "references", "attack-v19-routes.json");
const registry = existsSync(registryPath) ? await readJson(registryPath, "路由注册表") : null;
if (!registry) {
  fail(`缺少路由注册表：${registryPath}`);
}

const overlaySkills = new Set();
const manualOnly = new Set();
const routedSkills = new Map();
const coverageMap = new Map();

if (registry) {
  if (registry.snapshot !== SNAPSHOT_ID) fail(`注册表 snapshot=${registry.snapshot}，应为 ${SNAPSHOT_ID}`);
  const phaseIds = (registry.method_phases ?? []).map((p) => p.id);
  if (JSON.stringify(phaseIds) !== JSON.stringify(EXPECTED_PHASES)) {
    fail(`注册表 method_phases=${JSON.stringify(phaseIds)}，应为固定的 8 阶段`);
  }
  const wstgIds = (registry.wstg_categories ?? []).map((c) => c.id);
  if (JSON.stringify(wstgIds) !== JSON.stringify(EXPECTED_WSTG)) {
    fail(`注册表 wstg_categories 必须为固定 12 类，当前=${JSON.stringify(wstgIds)}`);
  }
  const targetKinds = new Set(registry.target_kinds ?? []);
  if (targetKinds.size === 0) fail("注册表缺 target_kinds 枚举");

  for (const name of registry.manual_only_skills ?? []) manualOnly.add(name);
  for (const overlay of registry.domain_overlays ?? []) {
    for (const name of overlay.skills ?? []) {
      if (overlaySkills.has(name)) fail(`skill ${name} 出现在多个 overlay`);
      overlaySkills.add(name);
    }
  }

  const routeIds = new Set();
  const routes = registry.routes ?? [];
  if (!Array.isArray(routes) || routes.length === 0) fail("注册表 routes 为空");

  for (const route of routes) {
    const label = route.id ?? "<无 id>";
    if (!/^route:[a-z0-9-]+$/.test(route.id ?? "")) fail(`route id 非法：${label}`);
    if (routeIds.has(route.id)) fail(`route id 重复：${route.id}`);
    routeIds.add(route.id);

    for (const field of ["method_phases", "tactics", "techniques", "subtechniques", "platforms", "target_kinds", "skills", "required", "excludes", "outputs", "handoff", "default_execution_class"]) {
      if (route[field] === undefined) fail(`${label} 缺必填字段 ${field}`);
    }

    for (const phase of route.method_phases ?? []) {
      if (!EXPECTED_PHASES.includes(phase)) fail(`${label} 使用未知 method_phase：${phase}`);
    }
    for (const kind of route.target_kinds ?? []) {
      if (!targetKinds.has(kind)) fail(`${label} 使用未知 target_kind：${kind}`);
    }
    for (const phase of route.handoff ?? []) {
      if (!EXPECTED_PHASES.includes(phase)) fail(`${label} handoff 指向未知阶段：${phase}`);
    }
    if (!EXECUTION_CLASSES.includes(route.default_execution_class)) {
      fail(`${label} default_execution_class=${route.default_execution_class} 非法`);
    }

    const allTechIds = [...(route.techniques ?? []), ...(route.subtechniques ?? [])];
    for (const techId of allTechIds) {
      if (!/^T\d{4}(\.\d{3})?$/.test(techId)) { fail(`${label} ATT&CK ID 非法：${techId}`); continue; }
      const tech = techniqueTable[techId];
      if (!tech) { fail(`${label} 引用了快照中不存在的 ATT&CK ID：${techId}`); continue; }
      if (tech.deprecated || tech.revoked) fail(`${label} 引用了已废弃/撤销的 ATT&CK ID：${techId}`);
      if (techId.includes(".")) {
        const parent = techId.split(".")[0];
        if (!(route.techniques ?? []).includes(parent)) {
          fail(`${label} sub-technique ${techId} 的父类 ${parent} 未列入 techniques`);
        }
      }
    }
    for (const tacticId of route.tactics ?? []) {
      if (!tacticById.has(tacticId)) { fail(`${label} 引用了快照中不存在的 tactic：${tacticId}`); continue; }
      if (allTechIds.length > 0) {
        const allowed = new Set(allTechIds.flatMap(techniqueTacticIds));
        if (!allowed.has(tacticId)) {
          fail(`${label} tactic ${tacticId} 与其 technique 在快照中的 kill chain 归属不一致`);
        }
      }
    }
    if ((route.techniques ?? []).length === 0 && (route.tactics ?? []).length > 0) {
      fail(`${label} 声明了 tactic 但没有 technique`);
    }

    const skills = route.skills ?? [];
    const routers = skills.filter((s) => s.role === "router");
    const specialists = skills.filter((s) => s.role === "specialist");
    if (routers.length > 1) fail(`${label} 含 ${routers.length} 个 router（最多 1 个）`);
    if (specialists.length > 1) fail(`${label} 含 ${specialists.length} 个 specialist（最多 1 个）`);
    for (const skill of skills) {
      if (!/^[a-z0-9-]+$/.test(skill.name ?? "") || /[*?[\]{}]/.test(skill.name ?? "")) {
        fail(`${label} 含非法/通配 skill 名：${skill.name}`);
        continue;
      }
      if (!Number.isInteger(skill.priority)) fail(`${label} skill ${skill.name} 缺整数 priority`);
      const check = await skillNameResolves(skill.name);
      if (!check.ok) fail(`${label} ${check.reason}`);
      if (overlaySkills.has(skill.name)) fail(`${label} skill ${skill.name} 同时在 domain_overlays 中`);
      if (manualOnly.has(skill.name)) fail(`${label} skill ${skill.name} 是 manual_only，不得自动路由`);
      if (routedSkills.has(skill.name) && routedSkills.get(skill.name) !== skill.role) {
        fail(`skill ${skill.name} 在不同 route 中角色不一致`);
      }
      routedSkills.set(skill.name, skill.role);
    }

    const required = route.required ?? {};
    for (const key of ["capabilities", "evidence", "tools_any", "task"]) {
      if (required[key] === undefined) fail(`${label} required 缺 ${key}`);
    }
    const excludes = route.excludes ?? {};
    if (typeof excludes.single_stage !== "boolean" || typeof excludes.report_only !== "boolean") {
      fail(`${label} excludes.single_stage/report_only 必须为布尔值`);
    }
    const outputs = route.outputs ?? {};
    if (!Array.isArray(outputs.evidence_types) || !Array.isArray(outputs.artifact_types) || typeof outputs.exit !== "string") {
      fail(`${label} outputs 缺 evidence_types/artifact_types/exit`);
    }

    /* coverage 以 route 显式声明的 tactics 为准：snapshot kill chain 中
       未被 route 声明的战术归属不计为已覆盖（如 T1078.004 虽属 stealth/persistence，
       但本技能库不为这两个战术提供自动路由）。 */
    for (const tacticId of route.tactics ?? []) {
      if (!tacticById.has(tacticId)) continue;
      if (!coverageMap.has(tacticId)) coverageMap.set(tacticId, { techniques: new Set(), routes: new Set() });
      for (const techId of allTechIds) coverageMap.get(tacticId).techniques.add(techId);
      coverageMap.get(tacticId).routes.add(route.id);
    }
  }

  /* technique 歧义检查：同 technique+阶段+等价条件时不得同优先级并列 */
  for (let i = 0; i < routes.length; i += 1) {
    for (let j = i + 1; j < routes.length; j += 1) {
      const a = routes[i];
      const b = routes[j];
      const sharedTech = (a.techniques ?? []).filter((t) => (b.techniques ?? []).includes(t));
      if (sharedTech.length === 0) continue;
      const sharedPhase = (a.method_phases ?? []).filter((p) => (b.method_phases ?? []).includes(p));
      if (sharedPhase.length === 0) continue;
      const sameVulnClass = (a.required?.task?.vuln_class ?? null) === (b.required?.task?.vuln_class ?? null);
      if (!sameVulnClass) continue;
      const targetOverlap = (a.target_kinds ?? []).some((k) => (b.target_kinds ?? []).includes(k));
      if (!targetOverlap) continue;
      const aW = a.wstg_categories ?? [];
      const bW = b.wstg_categories ?? [];
      const wstgOverlap = aW.length === 0 && bW.length === 0 ? true : aW.some((w) => bW.includes(w));
      if (!wstgOverlap) continue;
      const aPri = Math.min(...(a.skills ?? []).map((s) => s.priority), 99);
      const bPri = Math.min(...(b.skills ?? []).map((s) => s.priority), 99);
      const samePreferred = (a.required?.task?.preferred_skill ?? null) === (b.required?.task?.preferred_skill ?? null);
      if (aPri === bPri && samePreferred) {
        fail(`technique ${sharedTech.join("/")} 在阶段 ${sharedPhase.join("/")} 存在同优先级歧义路由：${a.id} 与 ${b.id}`);
      } else {
        warn(`technique ${sharedTech.join("/")} 被多条 route 覆盖（${a.id} p${aPri} / ${b.id} p${bPri}），按 priority+preferred_skill 消歧`);
      }
    }
  }

  /* WSTG 分类必须指向真实 specialist */
  const referencedWstg = new Set(routes.flatMap((r) => r.wstg_categories ?? []));
  for (const wstgId of referencedWstg) {
    if (!EXPECTED_WSTG.includes(wstgId)) { fail(`route 引用了未知 WSTG 分类：${wstgId}`); continue; }
    const hasSpecialist = routes.some((r) =>
      (r.wstg_categories ?? []).includes(wstgId) && (r.skills ?? []).some((s) => s.role === "specialist"));
    if (!hasSpecialist) fail(`WSTG 分类 ${wstgId} 没有任何真实 specialist route`);
  }

  /* overlay / manual_only skill 可解析 */
  for (const name of [...overlaySkills, ...manualOnly]) {
    const check = await skillNameResolves(name);
    if (!check.ok) fail(`overlay/manual_only ${check.reason}`);
  }

  if (!registry.trigger_rules?.load_attack_chain_when || !registry.trigger_rules?.bypass_attack_chain_when) {
    fail("注册表缺 trigger_rules");
  }
}

/* ---------------------------------------------------------------- 4. coverage */

let coverageResult = null;
if (registry && snapshot) {
  const tactics = (snapshot.tactics ?? []).map((tactic) => {
    const hit = coverageMap.get(tactic.id);
    if (hit) {
      return {
        id: tactic.id,
        name: tactic.name,
        status: "supported",
        techniques: [...hit.techniques].sort(),
        routes: [...hit.routes].sort()
      };
    }
    return {
      id: tactic.id,
      name: tactic.name,
      status: "gap",
      reason: GAP_NOTES[tactic.id] ?? "技能库无真实支持的 technique，显式 coverage gap"
    };
  });
  const supported = tactics.filter((t) => t.status === "supported").length;
  coverageResult = {
    version: 1,
    snapshot: SNAPSHOT_ID,
    generated_by: "scripts/validate-routes.mjs --write-coverage",
    tactics,
    summary: { total: tactics.length, supported, gap: tactics.length - supported, note: "gap 不计为已覆盖" }
  };

  const coveragePath = join(skillDir, "references", "attack-v19-coverage.json");
  if (writeCoverage) {
    await writeFile(coveragePath, `${JSON.stringify(coverageResult, null, 2)}\n`, "utf8");
  } else if (!existsSync(coveragePath)) {
    fail("缺少 attack-v19-coverage.json；用 --write-coverage 生成");
  } else {
    const existing = await readJson(coveragePath, "coverage");
    if (existing) {
      const norm = (c) => JSON.stringify(c.tactics);
      if (existing.snapshot !== SNAPSHOT_ID || norm(existing) !== norm(coverageResult)) {
        fail("attack-v19-coverage.json 与注册表重算结果不一致；用 --write-coverage 重新生成");
      }
    }
  }
}

/* ---------------------------------------------------------------- 5. 路由夹具 */

function intersects(a, b) {
  return a.some((x) => b.includes(x));
}

function requiredSatisfied(required, ctx) {
  for (const cap of required.capabilities ?? []) {
    if (!(ctx.capabilities ?? []).includes(cap)) return false;
  }
  for (const ev of required.evidence ?? []) {
    if (!(ctx.evidence ?? []).includes(ev)) return false;
  }
  const toolsAny = required.tools_any ?? [];
  if (toolsAny.length > 0 && !intersects(toolsAny, ctx.tools ?? [])) return false;
  for (const [key, value] of Object.entries(required.task ?? {})) {
    if (ctx.task?.[key] !== value) return false;
  }
  return true;
}

function simulate(ctx) {
  let candidates = (registry.routes ?? []).filter((route) => {
    if (!route.method_phases.includes(ctx.method_phase)) return false;
    if (ctx.single_stage && route.excludes?.single_stage) return false;
    if (ctx.report_only && route.excludes?.report_only) return false;
    if (ctx.techniques?.length) {
      const routeTechs = [...(route.techniques ?? [])];
      if (!intersects(routeTechs, ctx.techniques)) return false;
    }
    if (ctx.wstg_categories?.length && !intersects(route.wstg_categories ?? [], ctx.wstg_categories)) return false;
    if (ctx.target_kind && !(route.target_kinds ?? []).includes(ctx.target_kind)) return false;
    if (ctx.platform && (route.platforms ?? []).length > 0 && !route.platforms.includes(ctx.platform)) return false;
    return requiredSatisfied(route.required ?? {}, ctx);
  });
  if (ctx.task?.preferred_skill) {
    candidates = candidates.filter((route) =>
      (route.skills ?? []).some((s) => s.name === ctx.task.preferred_skill));
  }
  if (candidates.length === 0) return { status: "gap", reason: "no route matches filters" };
  candidates.sort((a, b) => {
    const pa = Math.min(...a.skills.map((s) => s.priority), 99);
    const pb = Math.min(...b.skills.map((s) => s.priority), 99);
    return pa - pb;
  });
  const top = candidates[0];
  const topPriority = Math.min(...top.skills.map((s) => s.priority), 99);
  const tied = candidates.filter((r) => Math.min(...r.skills.map((s) => s.priority), 99) === topPriority);
  const topRouter = tied.flatMap((r) => r.skills.filter((s) => s.role === "router")).map((s) => s.name);
  const topSpecialist = tied.flatMap((r) => r.skills.filter((s) => s.role === "specialist")).map((s) => s.name);
  if (new Set(topRouter).size > 1 || new Set(topSpecialist).size > 1) {
    return { status: "gap", reason: `ambiguous candidates at priority ${topPriority}` };
  }
  return {
    status: "routed",
    route: top.id,
    router: topRouter[0] ?? null,
    specialist: topSpecialist[0] ?? null,
    execution_class: top.default_execution_class
  };
}

const fixturesPath = join(skillDir, "tests", "route-cases.json");
let fixtureSummary = { total: 0, passed: 0 };
if (!existsSync(fixturesPath)) {
  fail(`缺少路由夹具：${fixturesPath}`);
} else {
  const fixtures = await readJson(fixturesPath, "路由夹具");
  const cases = fixtures?.cases ?? [];
  fixtureSummary.total = cases.length;
  for (const testCase of cases) {
    const label = testCase.id ?? "<无 id>";
    const ctx = testCase.context ?? {};
    const expect = testCase.expect ?? {};
    const caseErrors = [];
    if (expect.load_attack_chain === false) {
      const direct = expect.specialist;
      if (direct) {
        const check = await skillNameResolves(direct);
        if (!check.ok) caseErrors.push(check.reason);
        const inOverlay = [...overlaySkills].includes(direct);
        const simulated = simulate(ctx);
        if (simulated.status === "routed" && simulated.specialist && simulated.specialist !== direct && !inOverlay) {
          caseErrors.push(`单阶段场景注册表仍会路由到 ${simulated.specialist}，与直达专项 ${direct} 不一致`);
        }
      } else {
        caseErrors.push("load_attack_chain=false 但未给 expect.specialist");
      }
    } else {
      const result = simulate(ctx);
      if (expect.status === "gap") {
        if (result.status !== "gap") caseErrors.push(`期望 routing gap，实际路由到 ${result.route}`);
      } else {
        if (result.status !== "routed") {
          caseErrors.push(`期望 routed，实际 gap（${result.reason}）`);
        } else {
          if (expect.route && result.route !== expect.route) caseErrors.push(`route 期望 ${expect.route}，实际 ${result.route}`);
          if (expect.specialist !== undefined && result.specialist !== expect.specialist) {
            caseErrors.push(`specialist 期望 ${expect.specialist}，实际 ${result.specialist}`);
          }
          if (expect.router !== undefined && result.router !== expect.router) {
            caseErrors.push(`router 期望 ${expect.router}，实际 ${result.router}`);
          }
          if (expect.execution_class && result.execution_class !== expect.execution_class) {
            caseErrors.push(`execution_class 期望 ${expect.execution_class}，实际 ${result.execution_class}`);
          }
          for (const banned of expect.must_not_route ?? []) {
            if (result.specialist === banned || result.router === banned) {
              caseErrors.push(`must_not_route 被违反：${banned}`);
            }
          }
        }
      }
    }
    if (testCase.handoff_sample !== undefined) {
      const sample = testCase.handoff_sample;
      if (typeof sample.suggestedNextGoal !== "string" || sample.suggestedNextGoal.trim() === "") {
        caseErrors.push("handoff_sample.suggestedNextGoal 必须是非空字符串");
      }
    }
    if (caseErrors.length > 0) {
      fail(`夹具 ${label} 失败：${caseErrors.join("；")}`);
    } else {
      fixtureSummary.passed += 1;
    }
  }
}

/* ---------------------------------------------------------------- 汇总 */

const coverageLine = coverageResult
  ? `coverage: ${coverageResult.summary.supported}/${coverageResult.summary.total} tactics supported, ${coverageResult.summary.gap} explicit gap (${coverageResult.tactics.filter((t) => t.status === "gap").map((t) => t.id).join(", ") || "无"})`
  : "coverage: 未计算";

const result = {
  skill: { directory: skillDir, declaredName: ownName ?? null, validName: ownName === "attack-chain" },
  skillsRoot,
  referencedDocSkills: referencedSkills.size,
  routes: registry?.routes?.length ?? 0,
  overlays: registry?.domain_overlays?.length ?? 0,
  manualOnly: registry?.manual_only_skills?.length ?? 0,
  coverage: coverageResult?.summary ?? null,
  fixtures: fixtureSummary,
  warnings,
  errors,
  ok: errors.length === 0
};

process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
process.stdout.write(`${coverageLine}\n`);
if (errors.length > 0) process.exitCode = 1;
