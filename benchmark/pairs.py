#!/usr/bin/env python3
"""The benchmark kit's one resolver for cases, configurations, aliases and
pins. Every kit script that needs to know WHAT is being run (provision,
run-cohort, extract, grade, compare, validate-kit) goes through here, so the
join from a case version and a configuration version to the merged manifest
provisioning consumes is written once. See benchmark/README.md, sections
"Vocabulary" and "The join", and plans/benchmark-case-configuration-design.md.

Vocabulary: a CASE (what is built and judged) has immutable versions under
cases/<caseId>/<caseVersion>/; a CONFIGURATION (who runs it, under what
limits) has immutable versions at configurations/<configId>/<configVersion>.json;
a RUN is one case version under one configuration version; an ALIAS is a
retired spec id resolving to a pair (aliases.json). versions.lock is the
append-only registry of every version's git object id.

CLI (all output is JSON on stdout; refusals are one line on stderr, exit 2):
  pairs.py resolve --case ID@V --config ID@V [--kit DIR]
  pairs.py resolve --spec LEGACY [--kit DIR]              # alias mode
  pairs.py list [--kit DIR]
  pairs.py registry-check [--kit DIR] [--history]        # kit validation
  pairs.py merged --case ID@V --config ID@V --out FILE   # merged manifest only
"""
import argparse, json, os, re, subprocess, sys
from pathlib import Path

ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]{0,31}$")
VER_RE = re.compile(r"^[0-9]+(\.[0-9]+){0,3}$")

def refuse(msg, code=2):
    print(f"benchmark pairs: {msg}", file=sys.stderr); sys.exit(code)

def kit_dir(arg):
    return Path(arg).resolve() if arg else Path(__file__).resolve().parent

def parse_ref(text, what):
    """'taskrun@0.1' -> ('taskrun', '0.1'). A bare id is refused: versions are
    always pinned on the command line and in every record (design §8)."""
    if not isinstance(text, str) or "@" not in text:
        refuse(f"--{what} must be pinned as <id>@<version> (got {text!r})")
    ident, version = text.split("@", 1)
    if not ID_RE.match(ident):
        refuse(f"--{what} id must match {ID_RE.pattern}: {ident!r}")
    if not VER_RE.match(version) or len(version) > 16:
        refuse(f"--{what} version must match {VER_RE.pattern} and be at most 16 characters: {version!r}")
    return ident, version

def git(kit, *args, check=True):
    proc = subprocess.run(["git", "-C", str(kit), *args], capture_output=True, text=True)
    if check and proc.returncode != 0:
        refuse(f"git {' '.join(args)} failed: {proc.stderr.strip()}")
    return proc.stdout.strip()

def repo_rel(kit, path):
    top = Path(git(kit, "rev-parse", "--show-toplevel"))
    return str(Path(path).resolve().relative_to(top))

def load_json(path, label):
    try:
        return json.load(open(path, encoding="utf-8"))
    except FileNotFoundError:
        refuse(f"{label} does not exist: {path}")
    except json.JSONDecodeError as exc:
        refuse(f"{label} is not valid JSON: {path}: {exc}")

def case_dir(kit, ident, version):
    return kit / "cases" / ident / version

def config_file(kit, ident, version):
    return kit / "configurations" / ident / f"{version}.json"

def object_id(kit, path, kind):
    """The git object id of PATH at HEAD, refusing a dirty or untracked path:
    a run pins what it ran, and provisioning copies from that object, so the
    working tree must not differ from it (design §2, r4 H-1 / r3 G-13)."""
    rel = repo_rel(kit, path)
    dirty = git(kit, "status", "--porcelain", "--ignored=no", "--", rel)
    if dirty:
        refuse(f"{kind} {rel} has uncommitted or untracked changes; commit first so the recorded object id identifies exactly what runs:\n{dirty}")
    oid = git(kit, "rev-parse", "--verify", "--quiet", f"HEAD:{rel}", check=False)
    if not oid:
        refuse(f"{kind} {rel} is not in HEAD; commit it first")
    return oid

def registry(kit):
    path = kit / "versions.lock"
    if not path.exists():
        return {"schemaVersion": 1, "entries": {}}
    return load_json(path, "versions.lock")

def registry_check_current(kit, doc=None):
    """Every registered id's object at HEAD equals its registered object."""
    reg = doc or registry(kit)
    problems = []
    for key, oid in reg.get("entries", {}).items():
        ident, version = key.split("@", 1)
        if key.startswith("configuration:"):
            pass
        rel = None
        if ":" in ident:
            kind, ident = ident.split(":", 1)
        else:
            kind = "case"
        if kind == "case":
            rel = repo_rel(kit, case_dir(kit, ident, version))
        else:
            rel = repo_rel(kit, config_file(kit, ident, version))
        now = git(kit, "rev-parse", "--verify", "--quiet", f"HEAD:{rel}", check=False)
        if now != oid:
            problems.append(f"{kind} {ident}@{version} is registered as {oid} but HEAD has {now or 'nothing'}: edited in place — add a new version instead")
    return problems

def registry_check_history(kit):
    """Append-only across git history: every id -> object pair the lock file
    ever held must still hold at HEAD (r7 K-1). Requires complete history:
    a shallow or grafted clone cannot prove it and is refused (r8 L-1)."""
    if git(kit, "rev-parse", "--is-shallow-repository") == "true":
        return ["the clone is shallow; the registry's append-only history cannot be verified — run `git fetch --unshallow` and retry"]
    grafts = git(kit, "replace", "--list", check=False)
    if grafts:
        return ["the clone carries git replace/graft objects; the registry's append-only history cannot be verified — remove them (`git replace -d`) and retry"]
    rel = "benchmark/versions.lock"
    head = registry(kit).get("entries", {})
    problems = []
    for commit in git(kit, "rev-list", "HEAD", "--", rel).split():
        blob = git(kit, "show", f"{commit}:{rel}", check=False)
        if not blob:
            continue
        try:
            past = json.loads(blob).get("entries", {})
        except json.JSONDecodeError:
            problems.append(f"versions.lock at {commit[:12]} is not valid JSON")
            continue
        for key, oid in past.items():
            if head.get(key) != oid:
                problems.append(f"{key} was registered as {oid} at {commit[:12]} and is {head.get(key) or 'absent'} at HEAD: the registry is append-only")
    return sorted(set(problems))

def resolve_case(kit, ident, version):
    d = case_dir(kit, ident, version)
    if not d.is_dir():
        have = sorted(p.name for p in (kit / "cases" / ident).iterdir()) if (kit / "cases" / ident).is_dir() else []
        refuse(f"case {ident}@{version} does not exist under {kit/'cases'}; versions available: {have or 'none'}")
    doc = load_json(d / "case.json", "case.json")
    if doc.get("id") != ident or doc.get("version") != version:
        refuse(f"case.json in {d} names {doc.get('id')}@{doc.get('version')}, not {ident}@{version}")
    tree = object_id(kit, d, "case version directory")
    reg = registry(kit).get("entries", {}).get(f"case:{ident}@{version}")
    if reg is None:
        refuse(f"case {ident}@{version} is not registered in versions.lock; register it (see README, 'add a case version')")
    if reg != tree:
        refuse(f"case {ident}@{version} at HEAD is {tree} but versions.lock registered {reg}: edited in place — add a new version")
    return {"id": ident, "version": version, "dir": str(d), "tree": tree, "doc": doc}

def resolve_config(kit, ident, version):
    f = config_file(kit, ident, version)
    if not f.is_file():
        have = sorted(p.stem for p in (kit / "configurations" / ident).glob("*.json")) if (kit / "configurations" / ident).is_dir() else []
        refuse(f"configuration {ident}@{version} does not exist under {kit/'configurations'}; versions available: {have or 'none'}")
    doc = load_json(f, "configuration")
    if doc.get("id") != ident or doc.get("version") != version:
        refuse(f"{f} names {doc.get('id')}@{doc.get('version')}, not {ident}@{version}")
    blob = object_id(kit, f, "configuration file")
    reg = registry(kit).get("entries", {}).get(f"configuration:{ident}@{version}")
    if reg is None:
        refuse(f"configuration {ident}@{version} is not registered in versions.lock; register it (see README, 'add a configuration')")
    if reg != blob:
        refuse(f"configuration {ident}@{version} at HEAD is {blob} but versions.lock registered {reg}: edited in place — add a new version")
    return {"id": ident, "version": version, "file": str(f), "tree": blob, "doc": doc}

def resolve_alias(kit, legacy):
    table = load_json(kit / "aliases.json", "aliases.json").get("aliases", {})
    if legacy not in table:
        refuse(f"unknown spec id {legacy!r}; the kit runs cases under configurations now (see README) — known aliases: {', '.join(sorted(table)) or 'none'}")
    return table[legacy]

def compatibility(case_doc, config_doc):
    """Design §6. Returns (refusals, warnings)."""
    refusals, warnings = [], []
    needs = case_doc.get("needs", {})
    env = config_doc.get("environment", {})
    net_need = needs.get("network", "either")
    net_allow = env.get("delegateNetwork")
    if net_need == "required" and net_allow == "denied":
        refusals.append("the case requires delegate network access and the configuration denies it")
    if net_need == "forbidden" and net_allow == "allowed":
        refusals.append("the case forbids delegate network access and the configuration allows it")
    case_os = needs.get("os", "any")
    cfg_os = (config_doc.get("machineConstraint") or {}).get("os", "any")
    if case_os != "any" and cfg_os != "any" and case_os != cfg_os:
        refusals.append(f"the case needs os={case_os} and the configuration is constrained to os={cfg_os}")
    fences = config_doc.get("fences", {})
    for name, floor in (needs.get("minFences") or {}).items():
        have = fences.get(name)
        if have is None or have < floor:
            refusals.append(f"the case needs fences.{name} >= {floor}; the configuration has {have}")
    roster = config_doc.get("roster", {})
    roles = roster.get("delegateRoles")
    if roles:
        impl = (roles.get("implementer", {}).get("runtime"), roles.get("implementer", {}).get("model"))
        crit = (roles.get("code-critic", {}).get("runtime"), roles.get("code-critic", {}).get("model"))
        same = impl == crit
    else:
        same = len(roster.get("delegates", {})) == 1
    if same and roster.get("independence") != "session-only":
        (refusals if roles else warnings).append("the implementer and the code critic resolve to one effective model without roster.independence=session-only; the merge check would refuse every critique (docs/orchestration.md step 4)")
    gate = case_doc.get("mission", {}).get("gate", {})
    binary = str(gate.get("threshold", "")).replace(" ", "") == ">=1" and gate.get("noiseFloor", 0) == 0
    if binary and fences.get("ledgerNoGainBudget", 0) < fences.get("cycles", 0) and fences.get("acceptBinaryGateFuse") is not True:
        warnings.append(f"binary gate with ledgerNoGainBudget={fences.get('ledgerNoGainBudget')} below cycles={fences.get('cycles')} and no acceptBinaryGateFuse: the contract will not seal until a human raises the budget or acknowledges the fuse (issues #4/#8)")
    return refusals, warnings

def merged_manifest(case, config, mode, alias=None):
    """The join (design §5): a manifest of exactly the shape provisioning has
    always consumed, assembled from the two halves. Alias mode keeps the
    legacy id, version label and title so a pre-migration cohort stays
    uniform (r5 I-2)."""
    c, k = case["doc"], config["doc"]
    m = {}
    if mode == "alias":
        m["id"] = alias["legacyId"]; m["version"] = alias["legacyVersionLabel"]; m["title"] = alias.get("legacyTitle") or f"{c['title']} — {k['title']}"
    else:
        m["id"] = c["id"]; m["version"] = c["version"]; m["title"] = f"{c['title']} — {k['title']}"
    for key in ("comparisonEligible", "comparisonEligibleNote", "product", "seededGap", "metrics", "metricsNote", "noiseFloors", "noiseFloorsNote"):
        if key in c: m[key] = c[key]
    if "acceptanceOnly" in k: m["acceptanceOnly"] = k["acceptanceOnly"]
    if "machineConstraint" in k: m["machineConstraint"] = k["machineConstraint"]
    fences = dict(k["fences"])
    caps = k.get("hostCaps") or {}
    if "maxTurns" in caps: fences["hostMaxTurns"] = caps["maxTurns"]
    if "maxBudgetUsd" in caps: fences["hostMaxBudgetUsd"] = caps["maxBudgetUsd"]
    m["fences"] = fences
    notes = k.get("notes") or {}
    if "fences" in notes: m["fencesNote"] = notes["fences"]
    m["roster"] = k["roster"]
    if "roster" in notes: m["rosterNote"] = notes["roster"]
    for key in ("seed", "grader", "language", "deferredMetrics", "watches", "completionGate"):
        if key in c: m[key] = c[key]
    needs = c.get("needs", {})
    env = {"delegateNetwork": k["environment"]["delegateNetwork"]}
    if needs.get("networkNote"): env["delegateNetworkReason"] = needs["networkNote"]
    env.update(needs.get("environmentNotes") or {})
    m["environment"] = env
    mission = dict(c["mission"])
    mission["envelope"] = {"dependencies": needs["dependencies"]}
    mission["exposure"] = k["exposure"]["statement"]
    if k["exposure"].get("note"): mission["exposureNote"] = k["exposure"]["note"]
    m["missionContract"] = mission
    m["benchmarkPair"] = pair_record(case, config, mode, alias)
    return m

def pair_record(case, config, mode, alias=None):
    rec = {"schemaVersion": 1, "caseId": case["id"], "caseVersion": case["version"], "caseTree": case["tree"],
           "configId": config["id"], "configVersion": config["version"], "configTree": config["tree"], "mode": mode}
    if mode == "alias":
        rec["legacyId"] = alias["legacyId"]; rec["legacyVersionLabel"] = alias["legacyVersionLabel"]
    return rec

def cmd_resolve(args):
    kit = kit_dir(args.kit)
    if args.spec:
        if args.case or args.config:
            refuse("--spec (alias mode) cannot be combined with --case/--config")
        legacy = args.spec
        a = resolve_alias(kit, legacy)
        alias = {"legacyId": legacy, "legacyVersionLabel": a["legacyVersionLabel"], "legacyTitle": a.get("legacyTitle")}
        case = resolve_case(kit, a["case"], a["caseVersion"]); config = resolve_config(kit, a["config"], a["configVersion"])
        mode = "alias"
        print(f"benchmark pairs: {legacy} is an alias for {case['id']}@{case['version']} under {config['id']}@{config['version']} (legacy naming kept); use --case/--config for new work", file=sys.stderr)
    else:
        if not (args.case and args.config):
            refuse("both --case <id>@<version> and --config <id>@<version> are required (or --spec <alias>)")
        ci, cv = parse_ref(args.case, "case"); ki, kv = parse_ref(args.config, "config")
        case = resolve_case(kit, ci, cv); config = resolve_config(kit, ki, kv); alias = None; mode = "pair"
    refusals, warnings = compatibility(case["doc"], config["doc"])
    for w in warnings: print(f"warning: {config['id']}@{config['version']}: {w}", file=sys.stderr)
    if refusals:
        for r in refusals: print(f"benchmark pairs: {case['id']}@{case['version']} under {config['id']}@{config['version']}: {r}", file=sys.stderr)
        sys.exit(2)
    out = {"case": {k: v for k, v in case.items() if k != "doc"}, "config": {k: v for k, v in config.items() if k != "doc"},
           "mode": mode, "pair": pair_record(case, config, mode, alias), "manifest": merged_manifest(case, config, mode, alias)}
    if args.out:
        json.dump(out["manifest"], open(args.out, "w"), indent=2); open(args.out, "a").write("\n")
    json.dump(out, sys.stdout, indent=2); print()

def cmd_register(args):
    """Append a version's object id to versions.lock, computed from the INDEX
    so the files and their registry line can land in ONE commit (after which
    HEAD and the registry agree, which is what resolve/registry-check verify).
    Refuses to change an existing entry: the registry is append-only."""
    kit = kit_dir(args.kit)
    reg = registry(kit); entries = reg.setdefault("entries", {})
    added = []
    for ref in args.case or []:
        ident, version = parse_ref(ref, "case"); rel = repo_rel(kit, case_dir(kit, ident, version))
        if not case_dir(kit, ident, version).is_dir(): refuse(f"no such case directory: {rel}")
        if git(kit, "status", "--porcelain", "--ignored=no", "--", rel).replace("A ", "").strip() and any(l[:2] not in ("A ", "M ", "R ") for l in git(kit, "status", "--porcelain", "--", rel).splitlines()):
            refuse(f"{rel} has unstaged changes; `git add` the whole directory first")
        oid = git(kit, "write-tree", f"--prefix={rel}/")
        key = f"case:{ident}@{version}"
        if key in entries and entries[key] != oid: refuse(f"{key} is already registered as {entries[key]}; the registry is append-only — add a new version")
        entries[key] = oid; added.append((key, oid))
    for ref in args.config or []:
        ident, version = parse_ref(ref, "config"); f = config_file(kit, ident, version); rel = repo_rel(kit, f)
        if not f.is_file(): refuse(f"no such configuration file: {rel}")
        oid = git(kit, "rev-parse", "--verify", "--quiet", f":{rel}", check=False)
        if not oid: refuse(f"{rel} is not staged; `git add` it first")
        key = f"configuration:{ident}@{version}"
        if key in entries and entries[key] != oid: refuse(f"{key} is already registered as {entries[key]}; the registry is append-only — add a new version")
        entries[key] = oid; added.append((key, oid))
    reg["entries"] = dict(sorted(entries.items()))
    reg.setdefault("schemaVersion", 1)
    reg.setdefault("note", "Append-only registry of every benchmark case version and configuration version and the git object id (tree for a case directory, blob for a configuration file) it was registered with. validate-kit refuses when HEAD differs from a registered id (edited in place) or when any id ever registered maps to a different object at HEAD (rewritten history). Add entries with `benchmark/pairs.py register`; never edit by hand.")
    json.dump(reg, open(kit / "versions.lock", "w"), indent=2); open(kit / "versions.lock", "a").write("\n")
    for key, oid in added: print(f"registered {key} -> {oid}")

def cmd_list(args):
    kit = kit_dir(args.kit); out = {"cases": {}, "configurations": {}, "aliases": {}}
    for d in sorted((kit / "cases").glob("*/*/case.json")):
        out["cases"].setdefault(d.parent.parent.name, []).append(d.parent.name)
    for f in sorted((kit / "configurations").glob("*/*.json")):
        out["configurations"].setdefault(f.parent.name, []).append(f.stem)
    if (kit / "aliases.json").exists():
        out["aliases"] = load_json(kit / "aliases.json", "aliases.json").get("aliases", {})
    json.dump(out, sys.stdout, indent=2); print()

def cmd_registry_check(args):
    kit = kit_dir(args.kit)
    problems = registry_check_current(kit)
    if args.history: problems += registry_check_history(kit)
    for p in problems: print(f"benchmark pairs: registry: {p}", file=sys.stderr)
    sys.exit(1 if problems else 0)

def main():
    ap = argparse.ArgumentParser(prog="pairs.py")
    sub = ap.add_subparsers(dest="cmd", required=True)
    r = sub.add_parser("resolve"); r.add_argument("--case"); r.add_argument("--config"); r.add_argument("--spec"); r.add_argument("--kit"); r.add_argument("--out")
    l = sub.add_parser("list"); l.add_argument("--kit")
    g = sub.add_parser("registry-check"); g.add_argument("--kit"); g.add_argument("--history", action="store_true")
    e = sub.add_parser("register"); e.add_argument("--kit"); e.add_argument("--case", action="append"); e.add_argument("--config", action="append")
    a = ap.parse_args()
    {"resolve": cmd_resolve, "list": cmd_list, "registry-check": cmd_registry_check, "register": cmd_register}[a.cmd](a)

if __name__ == "__main__":
    main()
