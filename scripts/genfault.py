#!/usr/bin/env python3
r"""Gera um decorador faultRepo (erro na chamada failAt; transação
envenenada após poisonAt) a partir de `type Repository interface` de um
arquivo de domínio.

Uso (na raiz do repositório):
  scripts/genfault.py backend/internal/modules/<m>/domain/<arquivo>.go domain \
      backend/internal/modules/<m>/application/faultrepo_test.go application_test \
      github.com/yurythx/projeto-nexus/internal/modules/<m>/domain
"""
import re, sys
src, alias, out, pkg, dompath = sys.argv[1:6]
text = open(src).read()
m = re.search(r'type Repository interface \{(.*?)\n\}', text, re.S)
body = m.group(1)
# junta linhas quebradas
lines = []
cur = ''
for raw in body.split('\n'):
    l = raw.strip()
    if not l or l.startswith('//'):
        continue
    cur += (' ' if cur else '') + l
    if cur.count('(') == cur.count(')'):
        lines.append(cur); cur = ''
builtins = {'UUID'}
def qual(t):
    return re.sub(r'(?<![\w.])([A-Z]\w*)', lambda mm: alias + '.' + mm.group(1), t)
def split_top(s):
    parts, depth, cur = [], 0, ''
    for ch in s:
        if ch in '([{': depth += 1
        if ch in ')]}': depth -= 1
        if ch == ',' and depth == 0:
            parts.append(cur.strip()); cur = ''
        else:
            cur += ch
    if cur.strip(): parts.append(cur.strip())
    return parts
methods = []
for l in lines:
    mm = re.match(r'(\w+)\((.*?)\)\s*(.*)$', l)
    name, params, rets = mm.groups()
    ps = split_top(params)
    names, typed = [], []
    pending = []
    for p in ps:
        toks = p.split(None, 1)
        if len(toks) == 1:
            pending.append(toks[0])
        else:
            for n in pending:
                names.append(n); typed.append((n, toks[1]))
            pending = []
            names.append(toks[0]); typed.append((toks[0], toks[1]))
    rets = rets.strip()
    if rets.startswith('('):
        rlist = split_top(rets[1:-1])
    elif rets:
        rlist = [rets]
    else:
        rlist = []
    methods.append((name, typed, names, rlist))

o = []
o.append(f'package {pkg}\n')
o.append('// Código gerado por scripts/genfault.py a partir da interface Repository do\n// domínio. Não edite à mão: regenere se a interface mudar.\n')
IMPORTS_PLACEHOLDER = len(o)
o.append('')
o.append('''var errBoom = errors.New("falha simulada no repositório")

// faultRepo envolve o repositório real: na chamada failAt devolve erro; depois
// da chamada poisonAt "envenena" a transação (a instrução SQL seguinte — do
// repositório, do outbox ou da auditoria — falha). noTx marca veneno em
// leitura fora de transação, onde não há instrução seguinte na mesma conexão.
type faultRepo struct {
	inner                   innerRepo
	calls, failAt, poisonAt int
	trace                   []string
	noTx                    bool
}

func (fr *faultRepo) hook(name string) error {
	fr.calls++
	fr.trace = append(fr.trace, name)
	if fr.calls == fr.failAt {
		return errBoom
	}
	return nil
}

func (fr *faultRepo) post(ctx context.Context, db database.DBTX) {
	if fr.calls != fr.poisonAt {
		return
	}
	if _, inTx := db.(pgx.Tx); !inTx {
		fr.noTx = true
		return
	}
	_, _ = db.Exec(ctx, `SELECT 1/0`)
}
''')
for name, typed, names, rlist in methods:
    params = ', '.join(f'{n} {qual(t)}' for n, t in typed)
    q = [qual(r) for r in rlist]
    retsig = '' if not q else (q[0] if len(q) == 1 else '(' + ', '.join(q) + ')')
    zeros = []
    for i, r in enumerate(q[:-1]):
        zeros.append(f'z{i}')
    o.append(f'func (fr *faultRepo) {name}({params}) {retsig} {{')
    o.append(f'\tif err := fr.hook("{name}"); err != nil {{')
    for i, r in enumerate(q[:-1]):
        o.append(f'\t\tvar z{i} {r}')
    o.append('\t\treturn ' + ', '.join(zeros + ['err']))
    o.append('\t}')
    dbn = 'db' if 'db' in names else 'nil'
    o.append(f'\tdefer fr.post(ctx, {dbn})')
    o.append(f'\treturn fr.inner.{name}(' + ', '.join(names) + ')')
    o.append('}\n')
code = '\n'.join(o[IMPORTS_PLACEHOLDER+1:])
std = ['"context"', '"errors"']
if 'time.' in code: std.append('"time"')
third = []
if 'uuid.' in code: third.append('"github.com/google/uuid"')
third.append('"github.com/jackc/pgx/v5"')
own = []
if 'pagination.' in code: own.append('"github.com/yurythx/projeto-nexus/internal/domain/pagination"')
if 'auth.' in code: own.append('"github.com/yurythx/projeto-nexus/internal/platform/auth"')
if 'publico.' in code: own.append('"github.com/yurythx/projeto-nexus/internal/platform/publico"')
own.append('"' + dompath + '"')
own.append('"github.com/yurythx/projeto-nexus/internal/platform/database"')
imp = 'import (\n' + '\n'.join('\t'+x for x in std) + '\n\n' + '\n'.join('\t'+x for x in third) + '\n\n' + '\n'.join('\t'+x for x in sorted(own)) + '\n)\n\ntype innerRepo = ' + alias + '.Repository\n'
o[IMPORTS_PLACEHOLDER] = imp
open(out, 'w').write('\n'.join(o))
print(f'{len(methods)} métodos')
