#!/usr/bin/env python3
"""Token diagnosis for 2026-09-15 (Europe/Amsterdam day) from Claude Code transcripts. Read-only."""
import os, json, re, collections, datetime, sys
UTC = datetime.timezone.utc
BASE = os.path.expanduser('~/.claude/projects')
DAY0 = '2026-09-14T22:00:00'          # 2026-09-15 00:00 CEST
NOW = datetime.datetime.now(UTC)
SCAN_FROM = datetime.datetime(2026, 9, 13, 22, tzinfo=UTC).timestamp()   # for cross-check windows
TODAY_FROM = datetime.datetime(2026, 9, 14, 22, tzinfo=UTC).timestamp()
COORD = '9292cf37-9f88-4700-a7f9-1fe2e45bf000'
BIG = 20000
METHOD = """- **Sources.** Every `~/.claude/projects/*` directory with a transcript modified since 2026-09-14T22:00Z. The seat directories are `-Users-wido-LocalStorage-GitHub-agentic-tools-m1b`, `-m1c` and `-m1e`. No worktree-named, /private/tmp-named, hact-20260912 or unsuffixed agentic-tools directory had 09-15 activity. Two other directories did (section 9). Main files are `<dir>/<session>.jsonl`. Delegate files are `<dir>/<session>/subagents/agent-*.jsonl`, and every isSidechain entry is in those files. Type, description, model and the launching toolUseId come from `agent-*.meta.json`.
- **Window.** Entries timestamped at or after 2026-09-14T22:00Z (00:00 CEST), up to generation time. Whole files are parsed, so a turn that started before the window keeps its cause. Only calls inside the window are counted.
- **API calls.** One call is one `message.id` (falling back to `requestId`) on assistant entries. Repeated entries for content blocks collapse to one call, taking the maximum output_tokens. No id appeared in more than one file. `<synthetic>` entries are skipped because they are not API calls. All-in = input + cache_creation + cache_read + output. Context = all-in minus output.
- **Turn starter.** The latest user-role entry that is not a tool_result.
  - Stop-hook refusal: text starting `Stop hook feedback:`. Each has a matching `hook_blocking_error` attachment with hookEvent Stop.
  - Notification: `origin.kind` task-notification, or text starting `<task-notification>`.
  - Peer: `origin.kind` peer, or text starting `Another Claude session sent a message` with `<cross-session-message from-name=...>`.
  - Human: `origin.kind` human, `promptSource` typed or queued, local-command entries, or `[Request interrupted by user]`.
  - Other: compaction summaries (`This session is being continued`) and usage-limit auto-continuations (`origin.kind` auto-continuation).
  - isMeta entries matching none of these (skill bodies) do not start a turn. No starter was left unclassified.
- **Engine jobs.** m1b's five `entrypoint: sdk-cli` sessions, briefed with `# Task Direction`, are headless engine jobs. They get their own row. Table 2b re-attributes subagent tokens to the cause of the turn that issued the Agent call.
- **Mid-turn deliveries.** A message that arrives while the model is working is written as a `queued_command` attachment. It does not start a turn under the rule. Table 2c shows the ranking when it does.
- **Large reads.** Tool results over 20,000 characters in main and engine sessions. Cost = characters / 4 x later calls in the window, in the same file, before the next compact_boundary. Results from before the window count only their re-reads inside it. For very large outputs the transcript keeps only the harness's truncated preview, so those estimates are a floor.
- **Resumed delegate.** A SendMessage result names the agent (`resumedAgentId`, or `queued for delivery to <id>`), or the file holds a coordinator message.
- **Not counted.** Codex usage, which is not in Claude transcripts. Cache reads count at full weight, as the spend fence counts them. Section 8 also shows totals without cache_read and with cache_read at 0.1 weight.
- **In-flight sessions.** The m1e coordinator 9292cf37 and this delegate are included and labelled. Their totals grow while the report is written.
"""
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'token-diagnosis-2026-09-15.md')

def seat_of(d):
    m = re.search(r'agentic-tools-(m1[bce])', d)
    return m.group(1) if m else None

def text_of(c):
    if isinstance(c, str): return c
    out = []
    for x in c or []:
        if isinstance(x, dict) and x.get('type') == 'text': out.append(x.get('text', ''))
    return '\n'.join(out)

def summ(inp):
    if not isinstance(inp, dict): return str(inp)[:80]
    for k in ('command', 'file_path', 'path', 'pattern', 'url', 'description', 'query'):
        if inp.get(k): return re.sub(r'\s+', ' ', str(inp[k]))[:80]
    return json.dumps(inp)[:80]

def classify(e, txt, sub, first):
    o = (e.get('origin') or {}).get('kind'); ps = e.get('promptSource'); s = txt.lstrip()
    if sub and first: return 'delegate_brief'
    if s.startswith('Stop hook feedback'): return 'stop_hook'
    if o == 'task-notification' or s.startswith('<task-notification>') or s.startswith('[SYSTEM NOTIFICATION'): return 'notification'
    if o == 'peer' or s.startswith('Another Claude session sent a message') or '<cross-session-message' in s[:400]: return 'peer'
    if o == 'coordinator' or s.startswith('The coordinator sent a message'): return 'coordinator_msg'
    if s.startswith('This session is being continued'): return 'other:compaction_summary'
    if o == 'auto-continuation' or s.startswith('Your claude.ai usage limit'): return 'other:usage_limit_resume'
    if ps == 'sdk' or s.startswith('# Task Direction'): return 'engine_job_brief'
    if o == 'human' or ps in ('typed', 'queued') or s.startswith(('<local-command', '<command-name>', '<bash-input>', '<bash-stdout>', '[Request interrupted')):
        return 'human'
    if e.get('isMeta'): return None     # skill expansion etc.: continuation of the current turn
    return 'other:unclassified'

def sublabel(k, txt):
    t = re.sub(r'\s+', ' ', txt or '')
    if k == 'stop_hook':
        m = re.search(r'Task:? ([^;]+); Stop (\w+); needs ([^;]+);', t)
        return f"{m.group(1)[:48]} / needs {m.group(3)[:36]}" if m else t[20:80]
    if k == 'notification':
        m = re.search(r'<summary>(.*?)</summary>', t)
        x = m.group(1) if m else t[:70]
        x = re.sub(r'"[^"]*"', '"..."', x); x = re.sub(r'\b[\w-]*\d[\w-]*\b', '#', x)
        return x[:60]
    if k == 'peer':
        m = re.search(r'from-name="([^"]+)"', t); return 'from ' + (m.group(1) if m else '?')
    return ''

def qclass(a):
    o = (a.get('origin') or {}).get('kind'); pr = a.get('prompt'); pr = pr if isinstance(pr, str) else json.dumps(pr)
    if a.get('commandMode') == 'task-notification' or pr.lstrip().startswith('<task-notification>'): return 'notification'
    if o == 'peer' or '<cross-session-message' in pr[:400]: return 'peer'
    if o == 'human': return 'human'
    if o == 'coordinator': return 'coordinator_msg'
    return 'other:queued'

calls = {}            # id -> record
files = {}            # path -> info
bigreads = []         # main sessions
starters = collections.Counter(); starter_samples = {}; unclassified = collections.Counter()
queued = collections.Counter()
agent_launch_cause = {}   # tool_use_id -> cause of launching turn
resumed_ids = collections.Counter()
stop_variants = collections.Counter()
meta = {}
subturns = collections.Counter()

for d in sorted(os.listdir(BASE)):
    dp = os.path.join(BASE, d)
    if not os.path.isdir(dp): continue
    seat = seat_of(d)
    for root, ds, fs in os.walk(dp):
        for f in fs:
            p = os.path.join(root, f)
            if f.endswith('.meta.json') and '/subagents/' in p:
                try: meta[f[len('agent-'):-len('.meta.json')]] = json.load(open(p))
                except Exception: pass
                continue
            if not f.endswith('.jsonl'): continue
            mt = os.stat(p).st_mtime
            if mt < (SCAN_FROM if seat else TODAY_FROM): continue
            sub = '/subagents/' in p
            sid = f[:-6]
            info = dict(path=p, dir=d, seat=seat or 'other', sub=sub, sid=sid,
                        parent=p.split('/')[-3] if sub else None, eps=collections.Counter(),
                        ordinals=[], boundaries=[], starters=[], first=None, last=None, bt=0)
            files[p] = info
            tooluse = {}; cause = None; alt = None; idpos = {}; sublab = ''
            for line in open(p, errors='replace'):
                try: e = json.loads(line)
                except Exception: continue
                t = e.get('type'); ts = e.get('timestamp') or ''
                if t == 'assistant':
                    m = e.get('message') or {}
                    if e.get('entrypoint'): info['eps'][e['entrypoint']] += 1
                    for x in m.get('content') or []:
                        if isinstance(x, dict) and x.get('type') == 'tool_use':
                            tooluse[x.get('id')] = (x.get('name'), summ(x.get('input')))
                            if x.get('name') in ('Agent', 'Task'): agent_launch_cause[x.get('id')] = cause
                    u = m.get('usage'); mid = m.get('id') or e.get('requestId')
                    if not u or not mid or m.get('model') == '<synthetic>': continue
                    rec = dict(id=mid, path=p, ts=ts, model=m.get('model'), inp=u.get('input_tokens') or 0,
                               cc=u.get('cache_creation_input_tokens') or 0, cr=u.get('cache_read_input_tokens') or 0,
                               out=u.get('output_tokens') or 0, cause=cause, alt=alt, sub=sublab)
                    if mid in idpos:
                        r = calls.get(mid)
                        if r and r['path'] == p: r['out'] = max(r['out'], rec['out'])
                        continue
                    idpos[mid] = len(info['ordinals']); info['ordinals'].append(ts)
                    old = calls.get(mid)
                    if old is None or ts < old['ts']: calls[mid] = rec
                    if ts >= DAY0:
                        info['first'] = info['first'] or ts; info['last'] = ts
                elif t == 'user':
                    m = e.get('message') or {}; c = m.get('content')
                    if isinstance(c, list) and any(isinstance(x, dict) and x.get('type') == 'tool_result' for x in c):
                        r = e.get('toolUseResult')
                        if isinstance(r, dict):
                            if r.get('resumedAgentId'): resumed_ids[r['resumedAgentId']] += 1
                            mm = re.search(r'queued for delivery to (a[0-9a-f]{8,})', str(r.get('message', '')))
                            if mm: resumed_ids[mm.group(1)] += 1
                        if not sub:
                            for x in c:
                                if not (isinstance(x, dict) and x.get('type') == 'tool_result'): continue
                                cont = x.get('content'); n = len(cont) if isinstance(cont, str) else len(text_of(cont))
                                if n > BIG:
                                    nm, sm = tooluse.get(x.get('tool_use_id'), ('?', '?'))
                                    bigreads.append(dict(path=p, pos=len(info['ordinals']), chars=n, tool=nm, target=sm, ts=ts))
                        continue
                    txt = text_of(c)
                    k = classify(e, txt, sub, first=not info['starters'])
                    if k:
                        cause = k; alt = k; info['starters'].append((ts, k)); sublab = sublabel(k, txt)
                        if ts >= DAY0 and not sub: subturns[(k, sublab)] += 1
                        if ts >= DAY0:
                            starters[(info['seat'], sub, k)] += 1
                            starter_samples.setdefault(k, re.sub(r'\s+', ' ', txt)[:150])
                            if k == 'other:unclassified': unclassified[re.sub(r'\s+', ' ', txt)[:100]] += 1
                            if k == 'stop_hook': stop_variants[re.sub(r'\d+', '#', re.sub(r'\s+', ' ', txt))[:160]] += 1
                elif t == 'attachment':
                    a = e.get('attachment') or {}
                    if a.get('type') == 'queued_command':
                        k = qclass(a); alt = k
                        if ts >= DAY0: queued[(info['seat'], sub, k)] += 1
                elif t == 'system' and e.get('subtype') == 'compact_boundary':
                    info['boundaries'].append(len(info['ordinals'])); info['bt'] += ts >= DAY0
            ep = info['eps'].most_common(1)
            info['kind'] = 'sub' if sub else ('engine' if ep and ep[0][0] == 'sdk-cli' else 'main')

# ---------- aggregation ----------
def allin(r): return r['inp'] + r['cc'] + r['cr'] + r['out']
def ctx(r): return r['inp'] + r['cc'] + r['cr']
today = [r for r in calls.values() if r['ts'] >= DAY0]
def fi(r): return files[r['path']]

def bucket(r, key='cause'):
    f = fi(r)
    if f['kind'] == 'sub': return 'delegates (subagents)'
    if f['kind'] == 'engine': return 'engine jobs (headless sdk-cli)'
    return r[key] or 'other:before_first_prompt'

SEATS = ['m1b', 'm1c', 'm1e']
def agg(rows, keyf):
    A = collections.defaultdict(lambda: dict(n=0, t=0, cr=0, ctx=0, peak=0))
    for r in rows:
        a = A[keyf(r)]; a['n'] += 1; a['t'] += allin(r); a['cr'] += r['cr']; a['ctx'] += ctx(r); a['peak'] = max(a['peak'], ctx(r))
    return A
M = lambda x: f"{x/1e6:,.1f}M"
K = lambda x: f"{x/1e3:,.0f}k"
def cest(ts): return (datetime.datetime.fromisoformat(ts.replace('Z', '+00:00')) + datetime.timedelta(hours=2)).strftime('%H:%M') if ts else '-'

L = []
w = L.append
seat_tot = agg(today, lambda r: fi(r)['seat'])
w('# Claude token diagnosis, 2026-09-15 (Europe/Amsterdam day)\n')
w(f'Generated {NOW.strftime("%Y-%m-%d %H:%M")}Z by `token_diag.py`. Window: 2026-09-14T22:00Z to generation time. Goal: seats-spend-tokens-in-bounded-sessions (seat m1e).\n')
w('<!--FINDINGS-->\n')
w('## 1. Per-seat day totals\n')
w('| seat | API calls | all-in tokens | cache_read | cache_read share | main | delegates | engine jobs |')
w('|---|---:|---:|---:|---:|---:|---:|---:|')
grand = sum(a['t'] for s, a in seat_tot.items() if s in SEATS)
for s in SEATS + sorted(x for x in seat_tot if x not in SEATS):
    a = seat_tot.get(s)
    if not a: continue
    kinds = agg([r for r in today if fi(r)['seat'] == s], lambda r: fi(r)['kind'])
    w(f"| {s} | {a['n']} | {M(a['t'])} | {M(a['cr'])} | {a['cr']/a['t']:.1%} | {M(kinds['main']['t'])} | {M(kinds['sub']['t'])} | {M(kinds['engine']['t'])} |")
w(f"| **m1b+m1c+m1e** | {sum(seat_tot[s]['n'] for s in SEATS if s in seat_tot)} | **{M(grand)}** | {M(sum(seat_tot[s]['cr'] for s in SEATS if s in seat_tot))} | | | | |\n")

def cause_table(rows, keyname, title, total_by_seat):
    w(title + '\n')
    for s in SEATS:
        sr = [r for r in rows if fi(r)['seat'] == s]
        if not sr: continue
        A = agg(sr, lambda r: bucket(r, keyname)); T = total_by_seat[s]['t']
        w(f'**{s}** (day {M(T)})\n')
        w('| cause | calls | all-in | share | cache_read | mean context/call | peak context |')
        w('|---|---:|---:|---:|---:|---:|---:|')
        for k, a in sorted(A.items(), key=lambda kv: -kv[1]['t']):
            w(f"| {k} | {a['n']} | {M(a['t'])} | {a['t']/T:.1%} | {M(a['cr'])} | {K(a['ctx']/a['n'])} | {K(a['peak'])} |")
        w('')
cause_table(today, 'cause', '## 2. Cause split per seat (turn-starter attribution)', seat_tot)

w('### 2a. Causes across the three seats\n')
AC = agg([r for r in today if fi(r)['seat'] in SEATS], lambda r: bucket(r))
w('| cause | calls | all-in | share of 3-seat day | cache_read | mean context/call |')
w('|---|---:|---:|---:|---:|---:|')
for k, a in sorted(AC.items(), key=lambda kv: -kv[1]['t']):
    w(f"| {k} | {a['n']} | {M(a['t'])} | {a['t']/grand:.1%} | {M(a['cr'])} | {K(a['ctx']/a['n'])} |")
w('')

# full attribution: delegate tokens folded into the launching turn's cause
def launch_cause(r):
    f = fi(r)
    if f['kind'] == 'sub':
        mt = meta.get(f['sid'][len('agent-'):], {})
        c = agent_launch_cause.get(mt.get('toolUseId'))
        return (c or 'unknown-launcher') + ' (via delegate)'
    return bucket(r)
w('### 2b. Sensitivity: delegate tokens folded into the cause of the turn that launched them\n')
AF = agg([r for r in today if fi(r)['seat'] in SEATS], launch_cause)
w('| cause | calls | all-in | share |'); w('|---|---:|---:|---:|')
for k, a in sorted(AF.items(), key=lambda kv: -kv[1]['t']):
    w(f"| {k} | {a['n']} | {M(a['t'])} | {a['t']/grand:.1%} |")
w('')
w('### 2c. Sensitivity: mid-turn queued deliveries (queued_command attachments) also start a segment\n')
AA = agg([r for r in today if fi(r)['seat'] in SEATS], lambda r: bucket(r, 'alt'))
w('| cause | calls | all-in | share | change vs 2a |'); w('|---|---:|---:|---:|---:|')
for k, a in sorted(AA.items(), key=lambda kv: -kv[1]['t']):
    w(f"| {k} | {a['n']} | {M(a['t'])} | {a['t']/grand:.1%} | {M(a['t'] - AC.get(k, {'t': 0})['t']) if k in AC else 'new'} |")
w('')
w('Turn starters and mid-turn queued deliveries counted today:\n')
w('| seat | file kind | starter or queued | cause | count |'); w('|---|---|---|---|---:|')
for (s, sub, k), v in sorted(starters.items()): w(f"| {s} | {'sub' if sub else 'main/engine'} | starter | {k} | {v} |")
for (s, sub, k), v in sorted(queued.items()): w(f"| {s} | {'sub' if sub else 'main/engine'} | queued mid-turn | {k} | {v} |")
w('')
if unclassified:
    w('Unclassified starters: ' + '; '.join(f'`{k[:80]}` x{v}' for k, v in unclassified.most_common(8)) + '\n')

# calls per turn per cause (main sessions)
w('### 2d. Calls per turn by cause (main interactive sessions, today)\n')
tp = collections.defaultdict(lambda: [0, 0, 0])
for r in today:
    f = fi(r)
    if f['kind'] != 'main' or f['seat'] not in SEATS: continue
    tp[r['cause']][1] += 1; tp[r['cause']][2] += allin(r)
for (s, sub, k), v in starters.items():
    if not sub and s in SEATS: tp[k][0] += v
w('| cause | turns | calls | calls/turn | all-in/turn |'); w('|---|---:|---:|---:|---:|')
for k, (nt, nc, tt) in sorted(tp.items(), key=lambda kv: -kv[1][2]):
    w(f"| {k} | {nt} | {nc} | {nc/max(nt,1):.1f} | {M(tt/max(nt,1))} |")
w('')
w('### 2e. What the stop-hook, notification and peer turns were (main interactive sessions, today)\n')
SL = agg([r for r in today if fi(r)['kind'] == 'main' and fi(r)['seat'] in SEATS and r['cause'] in ('stop_hook', 'notification', 'peer')], lambda r: (r['cause'], r['sub']))
w('| cause | sub-label (hook task / needs; notification summary; peer sender) | turns | calls | all-in |'); w('|---|---|---:|---:|---:|')
for c in ('notification', 'stop_hook', 'peer'):
    for k, a in sorted([kv for kv in SL.items() if kv[0][0] == c], key=lambda kv: -kv[1]['t'])[:7]:
        w(f"| {c} | {k[1].replace('|', '/')} | {subturns[k]} | {a['n']} | {M(a['t'])} |")
w('')
w('Stop-hook feedback variants (digits normalised):\n')
for k, v in stop_variants.most_common(5): w(f'- x{v}: `{k[:160]}`')
w('')

# context bands
w('## 3. Context-size bands (tokens by per-call context)\n')
def band(r):
    c = ctx(r)
    return '>=700k' if c >= 700e3 else '400-700k' if c >= 400e3 else '150-400k' if c >= 150e3 else '<150k'
w('| seat | kind | <150k | 150-400k | 400-700k | >=700k |'); w('|---|---|---:|---:|---:|---:|')
for s in SEATS:
    for kind in ('main', 'sub', 'engine'):
        rr = [r for r in today if fi(r)['seat'] == s and fi(r)['kind'] == kind]
        if not rr: continue
        B = agg(rr, band)
        w(f"| {s} | {kind} | " + ' | '.join(f"{M(B[b]['t'])} ({B[b]['n']})" if b in B else '-' for b in ('<150k', '150-400k', '400-700k', '>=700k')) + ' |')
w('')

# models
mains = [r for r in today if fi(r)['kind'] == 'main' and fi(r)['seat'] in SEATS]
BIG_T = sum(allin(r) for r in mains if ctx(r) >= 400e3); MAIN_T = sum(allin(r) for r in mains)
w(f'Main interactive sessions: {M(MAIN_T)} of {M(grand)} ({MAIN_T/grand:.1%}); calls at >=400k context carry {M(BIG_T)} ({BIG_T/MAIN_T:.1%} of main).\n')
w('## 4. Models\n')
w('| seat | kind | model | calls | all-in |'); w('|---|---|---|---:|---:|')
AM = agg(today, lambda r: (fi(r)['seat'], fi(r)['kind'], r['model']))
for k, a in sorted(AM.items()): w(f"| {k[0]} | {k[1]} | {k[2]} | {a['n']} | {M(a['t'])} |")
w('')

# sessions
w('## 5. Sessions and delegate files active today\n')
FS = agg(today, lambda r: r['path'])
w('| seat | kind | session / agent | label | calls | all-in | cache_read | peak ctx | CEST span | compactions |')
w('|---|---|---|---|---:|---:|---:|---:|---|---:|')
def label(f):
    if f['sid'] == COORD: return 'm1e coordinator (in flight)'
    if f['sub']:
        mt = meta.get(f['sid'][len('agent-'):], {})
        lab = mt.get('description', '?')
        if f['parent'] == COORD: lab = 'this diagnosis delegate (in flight): ' + lab
        return lab[:60]
    if f['kind'] == 'engine':
        return 'engine job (headless sdk-cli, # Task Direction brief)'
    return 'main seat session'
for p, a in sorted(FS.items(), key=lambda kv: -kv[1]['t']):
    f = files[p]
    w(f"| {f['seat']} | {f['kind']} | {f['sid'][:14]} | {label(f)} | {a['n']} | {M(a['t'])} | {M(a['cr'])} | {K(a['peak'])} | {cest(f['first'])}-{cest(f['last'])} | {f['bt']} |")
w('')

# delegates
w('## 6. Delegates\n')
subs = [r for r in today if fi(r)['kind'] == 'sub']
def dtype(r):
    return meta.get(fi(r)['sid'][len('agent-'):], {}).get('agentType', '?')
w('| seat | by model | calls | all-in |'); w('|---|---|---:|---:|')
for k, a in sorted(agg(subs, lambda r: (fi(r)['seat'], r['model'])).items()): w(f"| {k[0]} | {k[1]} | {a['n']} | {M(a['t'])} |")
w('')
w('| seat | by agent type | files | calls | all-in |'); w('|---|---|---:|---:|---:|')
for k, a in sorted(agg(subs, lambda r: (fi(r)['seat'], dtype(r))).items()):
    nf = len({r['path'] for r in subs if (fi(r)['seat'], dtype(r)) == k})
    w(f"| {k[0]} | {k[1]} | {nf} | {a['n']} | {M(a['t'])} |")
w('')
w('Top five delegates by tokens (all seats; engine jobs listed after):\n')
w('| seat | description | type | model(s) | calls | all-in | peak ctx | briefs/messages in file | resumed |')
w('|---|---|---|---|---:|---:|---:|---:|---|')
SF = agg(subs, lambda r: r['path'])
for p, a in sorted(SF.items(), key=lambda kv: -kv[1]['t'])[:5]:
    f = files[p]; aid = f['sid'][len('agent-'):]; mt = meta.get(aid, {})
    models = ','.join(sorted({r['model'] for r in subs if r['path'] == p}))
    nst = len(f['starters']); coord = sum(1 for s in f['starters'] if s[1] == 'coordinator_msg')
    res = 'yes' if (resumed_ids.get(aid) or coord or nst > 1 and any(s[1] in ('coordinator_msg',) for s in f['starters'])) else 'no'
    w(f"| {f['seat']} | {mt.get('description','?')[:60]} | {mt.get('agentType','?')} | {models} | {a['n']} | {M(a['t'])} | {K(a['peak'])} | {nst} ({coord} coordinator) | {res} (SendMessage x{resumed_ids.get(aid,0)}) |")
EF = agg([r for r in today if fi(r)['kind'] == 'engine'], lambda r: r['path'])
for p, a in sorted(EF.items(), key=lambda kv: -kv[1]['t']):
    f = files[p]
    w(f"| {f['seat']} | engine job {f['sid'][:8]} | sdk-cli | {','.join(sorted({r['model'] for r in today if r['path']==p}))} | {a['n']} | {M(a['t'])} | {K(a['peak'])} | {len(f['starters'])} | no |")
w('')

# large reads
w('## 7. Large tool results in main sessions (>20,000 chars)\n')
est = []
for b in bigreads:
    f = files[b['path']]
    nb = min([x for x in f['boundaries'] if x > b['pos']] or [len(f['ordinals'])])
    later = sum(1 for ts in f['ordinals'][b['pos']:nb] if ts >= DAY0)
    if later == 0 and b['ts'] < DAY0: continue
    est.append(dict(b, later=later, cost=b['chars'] / 4 * later, seat=f['seat'], sid=f['sid']))
w('| seat | large results | chars | est. re-read tokens | share of seat day |'); w('|---|---:|---:|---:|---:|')
for s in SEATS + ['other']:
    ee = [x for x in est if x['seat'] == s]
    if not ee: continue
    T = seat_tot.get(s, {'t': 1})['t']
    w(f"| {s} | {len(ee)} | {sum(x['chars'] for x in ee):,} | {M(sum(x['cost'] for x in ee))} | {sum(x['cost'] for x in ee)/T:.1%} |")
w('')
w('| seat | session | UTC date, CEST time | tool | command or file | chars | later calls | est. re-read |'); w('|---|---|---|---|---|---:|---:|---:|')
for x in sorted(est, key=lambda x: -x['cost'])[:5]:
    tgt = x['target'].replace('|', '\\|')[:80]
    w(f"| {x['seat']} | {x['sid'][:8]} | {x['ts'][5:10]} {cest(x['ts'])} | {x['tool']} | `{tgt}` | {x['chars']:,} | {x['later']} | {M(x['cost'])} |")
w('')

# cross-check windows
w('## 8. Cross-check windows (all-in, Claude transcripts only)\n')
def win(a, b):
    return agg([r for r in calls.values() if a <= r['ts'] < b and fi(r)['seat'] in SEATS], lambda r: fi(r)['seat'])
iso = lambda dt: dt.strftime('%Y-%m-%dT%H:%M:%S')
wins = [('09-15 CEST day so far', DAY0, '9999'),
        ('rolling 24 h to now', iso(NOW - datetime.timedelta(hours=24)), '9999'),
        ('09-14 CEST full day', '2026-09-13T22:00:00', DAY0),
        ('09-14 00:00-14:00 CEST', '2026-09-13T22:00:00', '2026-09-14T12:00:00')]
w('| window | m1b | m1c | m1e | total | total without cache_read | cache_read at 0.1 weight |'); w('|---|---:|---:|---:|---:|---:|---:|')
for name, a, b in wins:
    W = win(a, b); tot = sum(x['t'] for x in W.values()); crt = sum(x['cr'] for x in W.values())
    w(f"| {name} | " + ' | '.join(M(W[s]['t']) if s in W else '-' for s in SEATS) + f" | {M(tot)} | {M(tot-crt)} | {M(tot-crt+0.1*crt)} |")
w('')
w('Note: the 09-14 rows only include transcript files modified since 2026-09-13T22:00Z.\n')
w('Fence-style reading (UTC day from 2026-09-15T00:00Z; top-level session files only, no subagents/ files; engine job sessions excluded), cumulative:\n')
def fence(r, s): f = fi(r); return f['seat'] == s and f['kind'] == 'main'
w('| up to (UTC) | CEST | m1b | m1c | m1e | 3 seats | m1e calls | m1e mean ctx |'); w('|---|---|---:|---:|---:|---:|---:|---:|')
for hh in ('06:00', '07:00', '08:00', '09:00', '09:45', '10:00', '11:00'):
    lim = '2026-09-15T' + hh
    v = {s: [r for r in calls.values() if '2026-09-15T00:00' <= r['ts'] < lim and fence(r, s)] for s in SEATS}
    e = v['m1e']
    w(f"| {hh} | {int(hh[:2])+2:02d}{hh[2:]} | " + ' | '.join(M(sum(allin(r) for r in v[s])) for s in SEATS) + f" | {M(sum(allin(r) for s in SEATS for r in v[s]))} | {len(e)} | {K(sum(ctx(r) for r in e)/max(len(e),1))} |")
w('')

# other dirs
w('## 9. Other directories with 09-15 activity (not in the seat totals)\n')
w('| directory | session | calls | all-in | turn starters |'); w('|---|---|---:|---:|---|')
for p, a in sorted(agg([r for r in today if fi(r)['seat'] == 'other'], lambda r: r['path']).items(), key=lambda kv: -kv[1]['t']):
    f = files[p]
    w(f"| {f['dir']} | {f['sid'][:8]} | {a['n']} | {M(a['t'])} | {len(f['starters'])} starters |")
w('')
w('## 10. Method and caveats\n')
w(METHOD)
def sl(prefix, cause):
    return sum(a['t'] for k, a in SL.items() if k[0] == cause and k[1].startswith(prefix))
def turns(cause): return sum(v for (s_, sub_, k), v in starters.items() if k == cause and not sub_ and s_ in SEATS)
MECH = {
  'notification': lambda a: f"a harness task notification wakes an idle main session: background Bash completions {M(sl('Background command','notification'))}, Monitor events {M(sl('Monitor','notification'))}, Agent finished {M(sl('Agent','notification'))}; {a['n']/max(turns('notification'),1):.1f} calls per wake at {K(a['ctx']/a['n'])} mean context",
  'stop_hook': lambda a: f"the metasystem stop gate blocks the stop (\"Stop blocked; needs your decision [and supervision repair]; Do not stop. Run this command\"), so each attempted stop buys {a['n']/max(turns('stop_hook'),1):.1f} more calls at {K(a['ctx']/a['n'])} mean context ({turns('stop_hook')} refusals)",
  'peer': lambda a: f"cross-session messages wake the receiving seat: from M1c {M(sl('from M1c','peer'))}, from M1e {M(sl('from M1e','peer'))}, from M1b {M(sl('from M1b','peer'))}; {a['n']/max(turns('peer'),1):.1f} calls per message at {K(a['ctx']/a['n'])}",
  'delegates (subagents)': lambda a: f"subagent files at {K(a['ctx']/a['n'])} mean context",
  'human': lambda a: 'typed prompts and local commands',
}
F = ['## Findings\n', '| seat | all-in (CEST day to generation) | cache_read share |', '|---|---:|---:|']
for s_ in SEATS: F.append(f"| {s_} | {M(seat_tot[s_]['t'])} | {seat_tot[s_]['cr']/seat_tot[s_]['t']:.1%} |")
F.append(f"| three seats | **{M(grand)}** | |\n")
F.append('Top three causes across the three seats (turn-starter attribution, table 2a):\n')
for i, (k, a) in enumerate(sorted(AC.items(), key=lambda kv: -kv[1]['t'])[:3], 1):
    F.append(f"{i}. **{k}**: {M(a['t'])}, {a['t']/grand:.1%} of the day, {a['n']} calls. Mechanism: {MECH.get(k, lambda a: '')(a)}.")
F.append('')
F.append(f"The multiplier under all three: main interactive sessions carry {M(MAIN_T)} ({MAIN_T/grand:.1%}), and {BIG_T/MAIN_T:.1%} of that comes from calls at 400k context or more. Every wake-up, whatever its cause, re-reads a 550-650k context.\n")
F.append('Attribution caveat: if a notification, peer or human message delivered mid-turn (queued_command attachment) starts its own segment (table 2c), the order becomes ' + ', '.join(f"{k} {M(a['t'])} ({a['t']/grand:.1%})" for k, a in sorted(AA.items(), key=lambda kv: -kv[1]['t'])[:3]) + '. Stop-hook refusals then fall to third, because many of their calls follow a mid-turn delivery.\n')
LS = max(((p_, a) for p_, a in FS.items()), key=lambda kv: kv[1]['t']); LD = max(((p_, a) for p_, a in SF.items()), key=lambda kv: kv[1]['t'])
F.append(f"Largest session: {files[LS[0]]['seat']} main {files[LS[0]]['sid'][:8]}, {LS[1]['n']} calls, {M(LS[1]['t'])}, peak {K(LS[1]['peak'])}, {files[LS[0]]['bt']} compactions today. Largest delegate: {files[LD[0]]['seat']} \"{meta.get(files[LD[0]]['sid'][6:],{}).get('description','?')}\" ({meta.get(files[LD[0]]['sid'][6:],{}).get('agentType','?')}), {LD[1]['n']} calls, {M(LD[1]['t'])}, peak {K(LD[1]['peak'])}, resumed.\n")
F.append(f"Large reads (>20k chars) are not a top cause: {len([x for x in est if x['seat'] in SEATS])} results, about {M(sum(x['cost'] for x in est if x['seat'] in SEATS))} estimated re-read tokens across seats.\n")
F.append("Cross-check with the alert: the spend fence ledger for m1e (observed 10:45Z) reads seat dayTokens 264,430,431 and day tokens 322,528,167. This report's fence-style reading of m1e (UTC day, top-level session files only) gives 264.4M up to 11:00Z, a match. The fence therefore counts only m1e's own main transcripts from 00:00Z (02:00 CEST) plus 58.1M of engine job records of any runtime on this machine: 322.5M - 264.4M now, 301.8M - 243.7M at the alert. It leaves out m1b and m1c transcripts, every subagents/ file, and the 22:00-00:00Z slice. That is why the Claude total across the three seats (above) is about 2.9 times the alert. The fence-style m1e reading crossed 243.7M between 09:00Z and 09:45Z (about 11:30 CEST), so the '13:45 CEST' in the handoff notes is about two hours late. Both count cache_read at full weight; cache reads are 96-99% of every seat's figure.\n")
L[L.index('<!--FINDINGS-->\n')] = '\n'.join(F)
open(OUT, 'w').write('\n'.join(L))
print('\n'.join(F))

# ---------- compact stdout ----------
print('CAUSES', [(k, a['n'], M(a['t']), f"{a['t']/grand:.1%}", K(a['ctx']/a['n'])) for k, a in sorted(AC.items(), key=lambda kv: -kv[1]['t'])])
print('UNCLASS', unclassified.most_common(5))
print('META found', len(meta), 'launch causes', len(agent_launch_cause), 'bigreads', len(est), 'resumed', dict(resumed_ids))
print('files', collections.Counter((f['seat'], f['kind']) for f in files.values() if f['first']))
