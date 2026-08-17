import os

FILES = [
    "internal/analytics/analytics.go",
    "internal/analytics/posthog.go",
    "internal/monitoring/api.go",
    "internal/monitoring/initdata.go",
    "internal/payment/mock_yookassa.go",
    "internal/bot/handlers/router/router_demo.go",
    "internal/bot/handlers/router/router.go",
    "internal/bot/handlers/router/router_back.go",
]

def find_matching_paren(s, i):
    depth = 0
    n = len(s)
    in_str = False
    in_rstr = False
    esc = False
    j = i
    while j < n:
        c = s[j]
        if in_str:
            if esc:
                esc = False
            elif c == '\\':
                esc = True
            elif c == '"':
                in_str = False
        elif in_rstr:
            if c == '`':
                in_rstr = False
        else:
            if c == '"':
                in_str = True
            elif c == '`':
                in_rstr = True
            elif c == '(':
                depth += 1
            elif c == ')':
                depth -= 1
                if depth == 0:
                    return j
        j += 1
    return -1

for f in FILES:
    with open(f) as fh:
        src = fh.read()
    positions = []
    i = 0
    while True:
        idx = src.find('log.Printf(', i)
        if idx == -1:
            break
        paren = idx + len('log.Printf')
        end = find_matching_paren(src, paren)
        if end == -1:
            print("NO MATCH in", f, "at", idx)
            break
        positions.append((idx, paren, end))
        i = end + 1
    if not positions:
        print("no log.Printf in", f)
        continue
    out = src
    for (idx, paren, end) in sorted(positions, key=lambda x: x, reverse=True):
        inside = src[paren + 1:end]
        repl = 'slog.Info(fmt.Sprintf(' + inside + '))'
        out = out[:idx] + repl + out[end + 1:]
    with open(f, 'w') as fh:
        fh.write(out)
    print("converted", f, "->", len(positions), "calls")
