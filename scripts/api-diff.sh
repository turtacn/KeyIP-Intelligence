#!/usr/bin/env bash
# =============================================================================
#  KeyIP-Intelligence API Diff Tool
# =============================================================================
#  Compares two versions of the OpenAPI specification and produces a Markdown
#  report of added, removed, and modified endpoints and schemas.
#
#  Usage:
#    ./scripts/api-diff.sh old.yaml new.yaml
#    ./scripts/api-diff.sh path/to/v1.yaml path/to/v2.yaml
#    ./scripts/api-diff.sh --output report.md old.yaml new.yaml
#    ./scripts/api-diff.sh --help
#
#  Requirements:
#    - python3  (stdlib only; no external packages required)
# =============================================================================

set -euo pipefail

usage() {
    sed -n '3,15p' "$0"
    exit 0
}

OUTPUT_FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --help) usage ;;
        --output)
            shift
            OUTPUT_FILE="$1"
            ;;
        -*)
            echo "Unknown option: $1" >&2
            usage
            ;;
        *)
            if [[ -z "${OLD_FILE:-}" ]]; then
                OLD_FILE="$1"
            elif [[ -z "${NEW_FILE:-}" ]]; then
                NEW_FILE="$1"
            else
                echo "Error: Too many positional arguments." >&2
                usage
            fi
            ;;
    esac
    shift
done

if [[ -z "${OLD_FILE:-}" ]] || [[ -z "${NEW_FILE:-}" ]]; then
    echo "Error: Two file paths are required." >&2
    usage
fi

for f in "$OLD_FILE" "$NEW_FILE"; do
    if [[ ! -f "$f" ]]; then
        echo "Error: File not found: $f" >&2
        exit 1
    fi
done

# ---------------------------------------------------------------------------
# Run diff via Python (stdlib only -- json + mini YAML parser)
# ---------------------------------------------------------------------------

python3 - "$OLD_FILE" "$NEW_FILE" <<'PYEOF'
import sys
import json

# ===========================================================================
# Minimal YAML parser (handles the full OpenAPI YAML subset)
# ===========================================================================

def yaml_load(path):
    with open(path, 'r', encoding='utf-8') as f:
        lines = f.readlines()

    # Remove BOM and trailing newlines
    lines = [l.rstrip('\n\r') for l in lines]
    if lines and lines[0].startswith('﻿'):
        lines[0] = lines[0][1:]

    def parse_scalar(s):
        """Convert a YAML scalar string to a Python value."""
        s = s.strip()
        if not s or s in ('~', 'null'):
            return None
        if s.lower() in ('true', 'yes', 'on'):
            return True
        if s.lower() in ('false', 'no', 'off'):
            return False

        # Flow sequence: [a, b, c]
        if s.startswith('[') and s.endswith(']'):
            inner = s[1:-1].strip()
            if not inner:
                return []
            items = []
            depth = 0
            buf = ''
            in_sq = in_dq = False
            for ch in inner:
                if ch == "'" and not in_dq:
                    in_sq = not in_sq
                elif ch == '"' and not in_sq:
                    in_dq = not in_dq
                elif ch in '([{' and not in_sq and not in_dq:
                    depth += 1
                elif ch in ')]}' and not in_sq and not in_dq:
                    depth -= 1
                elif ch == ',' and depth == 0 and not in_sq and not in_dq:
                    items.append(parse_scalar(buf))
                    buf = ''
                    continue
                buf += ch
            if buf.strip():
                items.append(parse_scalar(buf))
            return items

        # Flow mapping: {k: v, k2: v2}
        if s.startswith('{') and s.endswith('}'):
            inner = s[1:-1].strip()
            if not inner:
                return {}
            result = {}
            depth = 0
            buf = ''
            in_sq = in_dq = False
            for ch in inner:
                if ch == "'" and not in_dq:
                    in_sq = not in_sq
                elif ch == '"' and not in_sq:
                    in_dq = not in_dq
                elif ch in '([{' and not in_sq and not in_dq:
                    depth += 1
                elif ch in ')]}' and not in_sq and not in_dq:
                    depth -= 1
                elif ch == ',' and depth == 0 and not in_sq and not in_dq:
                    k, v = _parse_flow_pair(buf)
                    if k is not None:
                        result[k] = v
                    buf = ''
                    continue
                buf += ch
            if buf.strip():
                k, v = _parse_flow_pair(buf)
                if k is not None:
                    result[k] = v
            return result

        # Quoted strings
        if len(s) >= 2:
            if s[0] == "'" and s[-1] == "'":
                return s[1:-1]
            if s[0] == '"' and s[-1] == '"':
                try:
                    return json.loads(s)
                except json.JSONDecodeError:
                    return s[1:-1]

        # Numbers
        try:
            if '.' in s or 'e' in s.lower():
                return float(s)
            return int(s)
        except (ValueError, TypeError):
            pass
        return s

    def _parse_flow_pair(s):
        """Parse a key: value pair in flow mapping context."""
        s = s.strip()
        colon_pos = -1
        depth = 0
        in_sq = in_dq = False
        for i, ch in enumerate(s):
            if ch == "'" and not in_dq:
                in_sq = not in_sq
            elif ch == '"' and not in_sq:
                in_dq = not in_dq
            elif ch in '([{' and not in_sq and not in_dq:
                depth += 1
            elif ch in ')]}' and not in_sq and not in_dq:
                depth -= 1
            elif ch == ':' and depth == 0 and not in_sq and not in_dq:
                colon_pos = i
                break
        if colon_pos >= 0:
            k = parse_scalar(s[:colon_pos])
            v = parse_scalar(s[colon_pos + 1:])
            return k, v
        return None, None

    def get_indent(line):
        return len(line) - len(line.lstrip())

    def strip_comment(line):
        """Remove trailing # comment from a line, respecting quotes."""
        in_sq = in_dq = False
        for i, ch in enumerate(line):
            if ch == "'" and not in_dq:
                in_sq = not in_sq
            elif ch == '"' and not in_sq:
                in_dq = not in_dq
            elif ch == '#' and not in_sq and not in_dq:
                return line[:i].rstrip()
        return line.rstrip()

    def collect_block(lines, idx, indent, style):
        """Collect lines for a block scalar (| or >)."""
        collected = []
        i = idx
        while i < len(lines):
            line = lines[i]
            stripped = line.lstrip()
            if not stripped or stripped.startswith('#'):
                if i > idx and get_indent(line) > indent:
                    collected.append('')
                    i += 1
                    continue
                if not stripped:
                    collected.append('')
                    i += 1
                    if get_indent(line) <= indent:
                        break
                    continue
                i += 1
                continue
            if get_indent(line) > indent:
                collected.append(stripped)
                i += 1
            else:
                break
        if style == '|':
            result = '\n'.join(collected)
            if result and not result.endswith('\n'):
                result += '\n'
        else:
            result = ' '.join(collected)
        return result, i

    def parse_seq(lines, idx, indent):
        """Parse a block sequence starting at lines[idx]."""
        items = []
        i = idx
        while i < len(lines):
            line = lines[i]
            if not line.strip() or line.strip().startswith('#'):
                i += 1
                continue
            cur = get_indent(line)
            if cur < indent:
                break
            if cur > indent:
                i += 1
                continue
            sl = line.lstrip()
            if not sl.startswith('- '):
                break
            item_text = sl[2:]
            cont_indent = cur + 2 + len(item_text) - len(item_text.lstrip())
            stripped_item = item_text.strip()

            if stripped_item in ('|', '>'):
                val, i = collect_block(lines, i + 1, cont_indent or cur + 2, stripped_item)
                items.append(val)
            elif not stripped_item:
                peek = i + 1
                if peek < len(lines):
                    nl = lines[peek]
                    ni = get_indent(nl)
                    ns = nl.lstrip()
                    if ni > cur:
                        if ns.startswith('- '):
                            items.append(parse_seq(lines, peek, ni))
                            si = peek
                            while si < len(lines):
                                sl2 = lines[si].rstrip('\n\r')
                                if not sl2.strip() or sl2.strip().startswith('#'):
                                    si += 1
                                    continue
                                if get_indent(sl2) < ni:
                                    break
                                if get_indent(sl2) == ni and not sl2.lstrip().startswith('- '):
                                    break
                                si += 1
                            i = si - 1
                        else:
                            sub, i = parse_map(lines, peek, ni)
                            items.append(sub)
                            i -= 1
                    else:
                        items.append(None)
                        i += 1
                        continue
                else:
                    items.append(None)
            else:
                items.append(parse_scalar(stripped_item))
            i += 1
        return items

    def parse_map(lines, idx, indent):
        """Parse a block mapping starting at lines[idx]."""
        result = {}
        i = idx
        while i < len(lines):
            line = lines[i]
            if not line.strip():
                i += 1
                continue
            cur = get_indent(line)
            if cur < indent:
                break
            sl = strip_comment(line)
            if not sl.strip():
                i += 1
                continue
            if sl.lstrip().startswith('- ') and cur == indent:
                break

            content = sl[cur:]

            # Find the colon separator
            colon = -1
            depth = 0
            in_sq = in_dq = False
            for ci, ch in enumerate(content):
                if ch == "'" and not in_dq:
                    in_sq = not in_sq
                elif ch == '"' and not in_sq:
                    in_dq = not in_dq
                elif ch in '([{' and not in_sq and not in_dq:
                    depth += 1
                elif ch in ')]}' and not in_sq and not in_dq:
                    depth -= 1
                elif ch == ':' and depth == 0 and not in_sq and not in_dq:
                    colon = ci
                    break

            if colon < 0:
                i += 1
                continue

            key_str = content[:colon].rstrip()
            val_str = content[colon + 1:].strip()
            key = parse_scalar(key_str)

            if val_str in ('|', '>'):
                val, i = collect_block(lines, i + 1, cur + 2, val_str)
            elif val_str == '':
                # Skip comment and blank lines to find first child
                peek = i + 1
                while peek < len(lines):
                    _nl = lines[peek].rstrip('\n\r')
                    _ns = _nl.strip()
                    if _ns and not _ns.startswith('#'):
                        break
                    peek += 1
                if peek < len(lines):
                    nl = lines[peek]
                    ni = get_indent(nl)
                    ns = nl.strip()
                    if ns and not ns.startswith('#') and ni > cur:
                        if ns.startswith('- '):
                            val = parse_seq(lines, peek, ni)
                            si = peek
                            while si < len(lines):
                                sl2 = lines[si].rstrip('\n\r')
                                if not sl2.strip() or sl2.strip().startswith('#'):
                                    si += 1
                                    continue
                                if get_indent(sl2) < ni:
                                    break
                                if get_indent(sl2) == ni and not sl2.lstrip().startswith('- '):
                                    break
                                si += 1
                            i = si - 1
                        else:
                            val, i = parse_map(lines, peek, ni)
                            i -= 1
                    else:
                        val = None
                else:
                    val = None
            else:
                val = parse_scalar(val_str)

            result[key] = val
            i += 1
        return result, i

    top = 0
    for line in lines:
        if line.strip() and not line.strip().startswith('#'):
            top = get_indent(line)
            break
    doc, _ = parse_map(lines, 0, top)
    return doc


# ===========================================================================
# Diff Logic
# ===========================================================================

def version(doc):
    info = doc.get("info", {}) or {}
    return info.get("version", "unknown")


def list_endpoints(doc):
    eps = {}
    paths = doc.get("paths", {}) or {}
    for path, methods in paths.items():
        if methods is None:
            continue
        for method in ("get", "post", "put", "patch", "delete", "head", "options"):
            spec = methods.get(method)
            if spec is None:
                continue
            key = f"{method.upper()} {path}"
            eps[key] = {
                "path": path,
                "method": method.upper(),
                "tags": spec.get("tags", []),
                "summary": spec.get("summary", ""),
                "operationId": spec.get("operationId", ""),
            }
    return eps


def list_schemas(doc):
    schemas = {}
    comp = doc.get("components", {}) or {}
    defs = comp.get("schemas", {}) or {}
    for name, spec in defs.items():
        if not isinstance(spec, dict):
            continue
        schemas[name] = {
            "type": spec.get("type", "object"),
            "required": spec.get("required", []),
            "properties": list((spec.get("properties", {}) or {}).keys()),
        }
    return schemas


old_path = sys.argv[1]
new_path = sys.argv[2]

old = yaml_load(old_path)
new = yaml_load(new_path)

old_ver = version(old)
new_ver = version(new)
old_eps = list_endpoints(old)
new_eps = list_endpoints(new)
old_schemas = list_schemas(old)
new_schemas = list_schemas(new)

old_keys = set(old_eps.keys())
new_keys = set(new_eps.keys())

added_eps = sorted(new_keys - old_keys)
removed_eps = sorted(old_keys - new_keys)
common_eps = sorted(old_keys & new_keys)

modified_eps = []
for key in common_eps:
    o, n = old_eps[key], new_eps[key]
    changes = []
    if o["summary"] != n["summary"]:
        changes.append(f'summary changed: "{o["summary"]}" -> "{n["summary"]}"')
    if o["tags"] != n["tags"]:
        changes.append(f"tags changed: {o['tags']} -> {n['tags']}")
    if o["operationId"] != n["operationId"]:
        changes.append(f"operationId changed: {o['operationId']} -> {n['operationId']}")
    if changes:
        modified_eps.append((key, changes))

old_schema_keys = set(old_schemas.keys())
new_schema_keys = set(new_schemas.keys())
added_schemas = sorted(new_schema_keys - old_schema_keys)
removed_schemas = sorted(old_schema_keys - new_schema_keys)
common_schemas = sorted(old_schema_keys & new_schema_keys)

modified_schemas = []
for name in common_schemas:
    o, n = old_schemas[name], new_schemas[name]
    changes = []
    if o["type"] != n["type"]:
        changes.append(f"type: {o['type']} -> {n['type']}")
    old_props, new_props = set(o["properties"]), set(n["properties"])
    added_props = new_props - old_props
    removed_props = old_props - new_props
    if added_props:
        changes.append(f"added properties: {sorted(added_props)}")
    if removed_props:
        changes.append(f"removed properties: {sorted(removed_props)}")
    if o["required"] != n["required"]:
        changes.append(f"required fields changed: {o['required']} -> {n['required']}")
    if changes:
        modified_schemas.append((name, changes))


def count_by_tag(eps_dict):
    counts = {}
    for info in eps_dict.values():
        for tag in info["tags"]:
            counts[tag] = counts.get(tag, 0) + 1
    return counts


old_tag_counts = count_by_tag(old_eps)
new_tag_counts = count_by_tag(new_eps)

# Build Markdown output
output = []
output.append(f"# API Diff: {old_ver} -> {new_ver}")
output.append("")
output.append(f"| Metric | Old ({old_ver}) | New ({new_ver}) |")
output.append("|---|---|---|")
output.append(f"| Total endpoints | {len(old_eps)} | {len(new_eps)} |")
output.append(f"| Total schemas | {len(old_schemas)} | {len(new_schemas)} |")
output.append(f"| Endpoints added | — | {len(added_eps)} |")
output.append(f"| Endpoints removed | {len(removed_eps)} | — |")
output.append(f"| Endpoints modified | — | {len(modified_eps)} |")
output.append(f"| Schemas added | — | {len(added_schemas)} |")
output.append(f"| Schemas removed | {len(removed_schemas)} | — |")
output.append(f"| Schemas modified | — | {len(modified_schemas)} |")
output.append("")

output.append("## Endpoints by Tag")
output.append("")
output.append("| Tag | Old | New |")
output.append("|---|---|---|")
all_tags = sorted(set(list(old_tag_counts.keys()) + list(new_tag_counts.keys())))
for tag in all_tags:
    o_cnt = old_tag_counts.get(tag, 0)
    n_cnt = new_tag_counts.get(tag, 0)
    if n_cnt > o_cnt:
        delta = f"+{n_cnt - o_cnt}"
    elif n_cnt < o_cnt:
        delta = str(n_cnt - o_cnt)
    else:
        delta = "—"
    output.append(f"| {tag} | {o_cnt} | {n_cnt} ({delta}) |")
output.append("")

if added_eps:
    output.append("## Added Endpoints")
    output.append("")
    for key in added_eps:
        info = new_eps[key]
        tags = ", ".join(info["tags"]) if info["tags"] else "_none_"
        summary = info["summary"] or "_no summary_"
        output.append(f"- **{key}** — {summary}")
        output.append(f"  - Tags: {tags}")
    output.append("")

if removed_eps:
    output.append("## Removed Endpoints")
    output.append("")
    for key in removed_eps:
        info = old_eps[key]
        tags = ", ".join(info["tags"]) if info["tags"] else "_none_"
        summary = info["summary"] or "_no summary_"
        output.append(f"- ~~**{key}**~~ — {summary}")
        output.append(f"  - Tags: {tags}")
    output.append("")

if modified_eps:
    output.append("## Modified Endpoints")
    output.append("")
    for key, changes in modified_eps:
        output.append(f"- **{key}**")
        for c in changes:
            output.append(f"  - {c}")
    output.append("")

if added_schemas:
    output.append("## Added Schemas")
    output.append("")
    for name in added_schemas:
        s = new_schemas[name]
        props = ", ".join(s["properties"]) if s["properties"] else "_none_"
        output.append(f"- **{name}** (`{s['type']}`) — properties: {props}")
    output.append("")

if removed_schemas:
    output.append("## Removed Schemas")
    output.append("")
    for name in removed_schemas:
        s = old_schemas[name]
        props = ", ".join(s["properties"]) if s["properties"] else "_none_"
        output.append(f"- ~~**{name}**~~ (`{s['type']}`) — properties: {props}")
    output.append("")

if modified_schemas:
    output.append("## Modified Schemas")
    output.append("")
    for name, changes in modified_schemas:
        output.append(f"- **{name}**")
        for c in changes:
            output.append(f"  - {c}")
    output.append("")

sys.stdout.write("\n".join(output) + "\n")
PYEOF

# --- Write to output file if requested ----------------------------------------

if [[ -n "${OUTPUT_FILE:-}" ]]; then
    python3 - "$OLD_FILE" "$NEW_FILE" > "$OUTPUT_FILE" <<'PYEOF2'
import sys, json

# Re-use the same parser from the main script
# (Inlined from above; identical logic, just different PYEOF marker)
def yaml_load(path):
    with open(path, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    lines = [l.rstrip('\n\r') for l in lines]
    if lines and lines[0].startswith('﻿'):
        lines[0] = lines[0][1:]

    def parse_scalar(s):
        s = s.strip()
        if not s or s in ('~','null'): return None
        if s.lower() in ('true','yes','on'): return True
        if s.lower() in ('false','no','off'): return False
        if s.startswith('[') and s.endswith(']'):
            inner = s[1:-1].strip()
            if not inner: return []
            items=[]; depth=0; buf=''; sq=dq=False
            for ch in inner:
                if ch=="'" and not dq: sq=not sq
                elif ch=='"' and not sq: dq=not dq
                elif ch in '([{' and not sq and not dq: depth+=1
                elif ch in ')]}' and not sq and not dq: depth-=1
                elif ch==',' and depth==0 and not sq and not dq:
                    items.append(parse_scalar(buf)); buf=''; continue
                buf+=ch
            if buf.strip(): items.append(parse_scalar(buf))
            return items
        if s.startswith('{') and s.endswith('}'):
            inner=s[1:-1].strip()
            if not inner: return {}
            res={}; depth=0; buf=''; sq=dq=False
            for ch in inner:
                if ch=="'" and not dq: sq=not sq
                elif ch=='"' and not sq: dq=not dq
                elif ch in '([{' and not sq and not dq: depth+=1
                elif ch in ')]}' and not sq and not dq: depth-=1
                elif ch==',' and depth==0 and not sq and not dq:
                    cols=-1; d2=0; s2=d2_=False
                    for ci,ch2 in enumerate(buf):
                        if ch2=="'" and not d2_: s2=not s2
                        elif ch2=='"' and not s2: d2_=not d2_
                        elif ch2 in '([{' and not s2 and not d2_: d2+=1
                        elif ch2 in ')]}' and not s2 and not d2_: d2-=1
                        elif ch2==':' and d2==0 and not s2 and not d2_: cols=ci; break
                    if cols>=0:
                        res[parse_scalar(buf[:cols])]=parse_scalar(buf[cols+1:])
                    buf=''; continue
                buf+=ch
            if buf.strip():
                cols=-1; d2=0; s2=d2_=False
                for ci,ch2 in enumerate(buf):
                    if ch2=="'" and not d2_: s2=not s2
                    elif ch2=='"' and not s2: d2_=not d2_
                    elif ch2 in '([{' and not s2 and not d2_: d2+=1
                    elif ch2 in ')]}' and not s2 and not d2_: d2-=1
                    elif ch2==':' and d2==0 and not s2 and not d2_: cols=ci; break
                if cols>=0:
                    res[parse_scalar(buf[:cols])]=parse_scalar(buf[cols+1:])
            return res
        if len(s)>=2:
            if s[0]=="'" and s[-1]=="'": return s[1:-1]
            if s[0]=='"' and s[-1]=='"':
                try: return json.loads(s)
                except: return s[1:-1]
        try:
            if '.' in s or 'e' in s.lower(): return float(s)
            return int(s)
        except: pass
        return s

    def gi(l): return len(l)-len(l.lstrip())
    def sc(l):
        sq=dq=False
        for i,ch in enumerate(l):
            if ch=="'" and not dq: sq=not sq
            elif ch=='"' and not sq: dq=not dq
            elif ch=='#' and not sq and not dq: return l[:i].rstrip()
        return l.rstrip()

    def cb(lines,idx,indent,style):
        col=[]; i=idx
        while i<len(lines):
            l=lines[i]; st=l.lstrip()
            if not st or st.startswith('#'):
                if i>idx and gi(l)>indent: col.append(''); i+=1; continue
                if not st: col.append(''); i+=1
                if gi(l)<=indent: break
                continue
            i+=1; continue
            if gi(l)>indent: col.append(st); i+=1
            else: break
        if style=='|':
            r='\n'.join(col)
            if r and not r.endswith('\n'): r+='\n'
        else: r=' '.join(col)
        return r,i

    def ps(lines,idx,indent):
        items=[]; i=idx
        while i<len(lines):
            l=lines[i]
            if not l.strip() or l.strip().startswith('#'): i+=1; continue
            c=gi(l)
            if c<indent: break
            if c>indent: i+=1; continue
            sl=l.lstrip()
            if not sl.startswith('- '): break
            it=sl[2:]; si=it.strip()
            if si in ('|','>'):
                v,i=cb(lines,i+1,c+2+len(it)-len(it.lstrip()),si); items.append(v)
            elif not si:
                pk=i+1
                if pk<len(lines):
                    nl=lines[pk]; ni=gi(nl); ns=nl.lstrip()
                    if ni>c:
                        if ns.startswith('- '):
                            items.append(ps(lines,pk,ni))
                            sk=pk
                            while sk<len(lines):
                                sl2=lines[sk].rstrip('\n\r')
                                if not sl2.strip() or sl2.strip().startswith('#'): sk+=1; continue
                                if gi(sl2)<ni: break
                                if gi(sl2)==ni and not sl2.lstrip().startswith('- '): break
                                sk+=1
                            i=sk-1
                        else:
                            sub,i=pm(lines,pk,ni); items.append(sub); i-=1
                    else: items.append(None); i+=1; continue
                else: items.append(None)
            else: items.append(parse_scalar(si))
            i+=1
        return items

    def pm(lines,idx,indent):
        res={}; i=idx
        while i<len(lines):
            l=lines[i]
            if not l.strip(): i+=1; continue
            c=gi(l)
            if c<indent: break
            sl=sc(l)
            if not sl.strip(): i+=1; continue
            if sl.lstrip().startswith('- ') and c==indent: break
            con=sl[c:]
            colon=-1; depth=0; sq=dq=False
            for ci,ch in enumerate(con):
                if ch=="'" and not dq: sq=not sq
                elif ch=='"' and not sq: dq=not dq
                elif ch in '([{' and not sq and not dq: depth+=1
                elif ch in ')]}' and not sq and not dq: depth-=1
                elif ch==':' and depth==0 and not sq and not dq: colon=ci; break
            if colon<0: i+=1; continue
            k=parse_scalar(con[:colon].rstrip())
            vs=con[colon+1:].strip()
            if vs in ('|','>'):
                v,i=cb(lines,i+1,c+2,vs)
            elif vs=='':
                pk=i+1
                if pk<len(lines):
                    nl=lines[pk]; ni=gi(nl); ns=nl.strip()
                    if ns and not ns.startswith('#') and ni>c:
                        if ns.startswith('- '):
                            v=ps(lines,pk,ni)
                            sk=pk
                            while sk<len(lines):
                                sl2=lines[sk].rstrip('\n\r')
                                if not sl2.strip() or sl2.strip().startswith('#'): sk+=1; continue
                                if gi(sl2)<ni: break
                                if gi(sl2)==ni and not sl2.lstrip().startswith('- '): break
                                sk+=1
                            i=sk-1
                        else:
                            v,i=pm(lines,pk,ni); i-=1
                    else: v=None
                else: v=None
            else: v=parse_scalar(vs)
            res[k]=v; i+=1
        return res,i

    top=0
    for l in lines:
        if l.strip() and not l.strip().startswith('#'):
            top=gi(l); break
    doc,_=pm(lines,0,top)
    return doc

# Diff logic
old = yaml_load(sys.argv[1]); new = yaml_load(sys.argv[2])

def v(d): i=d.get("info",{}) or {}; return i.get("version","unknown")
def le(doc):
    eps={}
    for p,ms in (doc.get("paths",{}) or {}).items():
        if ms is None: continue
        for m in ("get","post","put","patch","delete","head","options"):
            s=ms.get(m)
            if s is None: continue
            eps[f"{m.upper()} {p}"]={"path":p,"method":m.upper(),"tags":s.get("tags",[]),"summary":s.get("summary",""),"operationId":s.get("operationId","")}
    return eps
def ls(doc):
    schemas={}
    for n,s in ((doc.get("components",{}) or {}).get("schemas",{}) or {}).items():
        if not isinstance(s,dict): continue
        schemas[n]={"type":s.get("type","object"),"required":s.get("required",[]),"properties":list((s.get("properties",{}) or {}).keys())}
    return schemas

ov=v(old); nv=v(new)
oeps=le(old); neps=le(new)
os=ls(old); ns=ls(new)
ok=set(oeps.keys()); nk=set(neps.keys())

aeps=sorted(nk-ok); reps=sorted(ok-nk); ceps=sorted(ok&nk)
meps=[]
for k in ceps:
    o,n=oeps[k],neps[k]; c=[]
    if o["summary"]!=n["summary"]: c.append(f'summary changed: "{o["summary"]}" -> "{n["summary"]}"')
    if o["tags"]!=n["tags"]: c.append(f"tags changed: {o['tags']} -> {n['tags']}")
    if o["operationId"]!=n["operationId"]: c.append(f"operationId changed: {o['operationId']} -> {n['operationId']}")
    if c: meps.append((k,c))

osk=set(os.keys()); nsk=set(ns.keys())
aschemas=sorted(nsk-osk); rschemas=sorted(osk-nsk); cschemas=sorted(osk&nsk)
mschemas=[]
for n in cschemas:
    o,nn=os[n],ns[n]; c=[]
    if o["type"]!=nn["type"]: c.append(f"type: {o['type']} -> {nn['type']}")
    op,np=set(o["properties"]),set(nn["properties"])
    ap=np-op; rp=op-np
    if ap: c.append(f"added properties: {sorted(ap)}")
    if rp: c.append(f"removed properties: {sorted(rp)}")
    if o["required"]!=nn["required"]: c.append(f"required fields changed: {o['required']} -> {nn['required']}")
    if c: mschemas.append((n,c))

otc={}; for i in oeps.values():
    for t in i["tags"]: otc[t]=otc.get(t,0)+1
ntc={}; for i in neps.values():
    for t in i["tags"]: ntc[t]=ntc.get(t,0)+1

out=[]
out.append(f"# API Diff: {ov} -> {nv}"); out.append("")
out.append(f"| Metric | Old ({ov}) | New ({nv}) |"); out.append("|---|---|---|")
out.append(f"| Total endpoints | {len(oeps)} | {len(neps)} |")
out.append(f"| Total schemas | {len(os)} | {len(ns)} |")
out.append(f"| Endpoints added | — | {len(aeps)} |"); out.append(f"| Endpoints removed | {len(reps)} | — |")
out.append(f"| Endpoints modified | — | {len(meps)} |"); out.append(f"| Schemas added | — | {len(aschemas)} |")
out.append(f"| Schemas removed | {len(rschemas)} | — |"); out.append(f"| Schemas modified | — | {len(mschemas)} |")
out.append(""); out.append("## Endpoints by Tag"); out.append("")
out.append("| Tag | Old | New |"); out.append("|---|---|---|")
for t in sorted(set(list(otc.keys())+list(ntc.keys()))):
    o=otc.get(t,0); n=ntc.get(t,0); d=f"+{n-o}" if n>o else str(n-o) if n<o else "—"
    out.append(f"| {t} | {o} | {n} ({d}) |")
out.append("")
if aeps:
    out.append("## Added Endpoints"); out.append("")
    for k in aeps:
        i=neps[k]; tg=", ".join(i["tags"]) or "_none_"
        out.append(f"- **{k}** — {i['summary']}"); out.append(f"  - Tags: {tg}")
    out.append("")
if reps:
    out.append("## Removed Endpoints"); out.append("")
    for k in reps:
        i=oeps[k]; tg=", ".join(i["tags"]) or "_none_"
        out.append(f"- ~~**{k}**~~ — {i['summary']}"); out.append(f"  - Tags: {tg}")
    out.append("")
if meps:
    out.append("## Modified Endpoints"); out.append("")
    for k,c in meps:
        out.append(f"- **{k}**")
        for x in c: out.append(f"  - {x}")
    out.append("")
if aschemas:
    out.append("## Added Schemas"); out.append("")
    for n in aschemas:
        s=ns[n]; p=", ".join(s["properties"]) or "_none_"
        out.append(f"- **{n}** (`{s['type']}`) — properties: {p}")
    out.append("")
if rschemas:
    out.append("## Removed Schemas"); out.append("")
    for n in rschemas:
        s=os[n]; p=", ".join(s["properties"]) or "_none_"
        out.append(f"- ~~**{n}**~~ (`{s['type']}`) — properties: {p}")
    out.append("")
if mschemas:
    out.append("## Modified Schemas"); out.append("")
    for n,c in mschemas:
        out.append(f"- **{n}**")
        for x in c: out.append(f"  - {x}")
    out.append("")
sys.stdout.write("\n".join(out)+"\n")
PYEOF2
    echo "API diff report written to: $OUTPUT_FILE"
fi
