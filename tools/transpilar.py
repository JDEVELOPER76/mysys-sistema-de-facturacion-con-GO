#!/usr/bin/env python3
"""Transpilador Jinja2 → Go html/template para las plantillas de MySYS."""
import re
import sys
import glob
import os

# ─────────────────────────────────────────────────────────────────────────────
# Utilidades de análisis (conscientes de comillas y paréntesis)
# ─────────────────────────────────────────────────────────────────────────────

def buscar_top(s, sub, desde=0):
    """Busca sub a nivel de anidamiento 0 y fuera de comillas."""
    prof = 0
    comilla = None
    i = desde
    while i <= len(s) - len(sub):
        c = s[i]
        if comilla:
            if c == comilla:
                comilla = None
            i += 1
            continue
        if c in "'\"":
            comilla = c
        elif c in "([{":
            prof += 1
        elif c in ")]}":
            prof -= 1
        if prof == 0 and s.startswith(sub, i):
            return i
        i += 1
    return -1

def dividir_top(s, sep):
    """Divide por sep a nivel 0 (soporta múltiples)."""
    partes = []
    prof = 0
    comilla = None
    inicio = 0
    i = 0
    while i <= len(s) - len(sep):
        c = s[i]
        if comilla:
            if c == comilla:
                comilla = None
            i += 1
            continue
        if c in "'\"":
            comilla = c
        elif c in "([{":
            prof += 1
        elif c in ")]}":
            prof -= 1
        if prof == 0 and s.startswith(sep, i):
            partes.append(s[inicio:i])
            i += len(sep)
            inicio = i
            continue
        i += 1
    partes.append(s[inicio:])
    return partes

def paren_cierre(s, ini):
    prof = 0
    comilla = None
    for i in range(ini, len(s)):
        c = s[i]
        if comilla:
            if c == comilla:
                comilla = None
            continue
        if c in "'\"":
            comilla = c
        elif c == '(':
            prof += 1
        elif c == ')':
            prof -= 1
            if prof == 0:
                return i
    return -1

# ─────────────────────────────────────────────────────────────────────────────
# Conversión de expresiones
# ─────────────────────────────────────────────────────────────────────────────

class Ctx:
    def __init__(self):
        self.vars = []      # pila de variables de plantilla (range/set)
        self.prof = 0       # profundidad de range (para $. vs .)

    def prefijo(self, nombre):
        if nombre in self.vars:
            return "$" + nombre
        return "$." + nombre if self.prof > 0 else "." + nombre

def convertir_cadena(lit):
    """'texto' → \"texto\" (Go no admite strings con comillas simples)."""
    if len(lit) >= 2 and lit[0] == "'" and lit[-1] == "'":
        inner = lit[1:-1].replace('"', '\\"')
        return '"' + inner + '"'
    return lit

def convertir_valor(v, ctx):
    v = v.strip()
    if not v:
        return v

    # loop.index (1-based en Jinja) → (inc $i); loop.first/last
    if v == "loop.index":
        return "(inc $i)"
    if v == "loop.index0":
        return "$i"
    if v == "loop.first":
        return "(eqx $i 0)"

    # url_for('static', path='x') → "/static/x"
    m = re.match(r"""^url_for\(\s*['"]static['"]\s*,\s*(?:path\s*=\s*)?['"]([^'"]*)['"]\s*\)$""", v)
    if m:
        return '"/static/' + m.group(1) + '"'

    # request.query_params.get('x') → variable de contexto directa
    m = re.match(r"""^request\.query_params\.get\(\s*['"]([^'"]*)['"]\s*\)$""", v)
    if m:
        return ctx.prefijo(m.group(1))

    # Cadenas
    if (v.startswith("'") and v.endswith("'")) or (v.startswith('"') and v.endswith('"')):
        return convertir_cadena(v)

    # Números y constantes
    if re.match(r'^-?\d+(\.\d+)?$', v):
        return v
    if v in ("True", "true"):
        return "true"
    if v in ("False", "false"):
        return "false"
    if v in ("None", "none", "nil"):
        return "nil"

    # Paréntesis envolventes
    if v.startswith("(") and paren_cierre(v, 0) == len(v) - 1:
        return "(" + convertir_expr(v[1:-1], ctx) + ")"

    # Métodos: x.lower() / x.upper() / x.title()
    m = re.match(r"^([\w\.\[\]]+)\.(lower|upper|title)\(\)$", v)
    if m:
        return "(" + m.group(2) + " " + convertir_valor(m.group(1), ctx) + ")"

    # x.strftime('fmt') → usar campo _dt si existe (caso fecha_completa)
    m = re.match(r"^([\w\.]+)\.strftime\(\s*'([^']*)'\s*\)$", v)
    if m:
        return convertir_valor(m.group(1), ctx)  # el handler da texto ya usable

    # x.Format("layout") (método Go sobre time.Time)
    m = re.match(r"^([\w\.]+)\.Format\(\s*\"([^\"]*)\"\s*\)$", v)
    if m:
        return "(" + convertir_valor(m.group(1), ctx) + '.Format "' + m.group(2) + '")'

    # Índices y slices: x[0], x[:2], x[1:], x[1:3]
    # (index sobre string devuelve byte en Go: usar at(), que es consciente del tipo)
    m = re.match(r"^([\w\.]+)\[(-?\d*)\]$", v)
    if m:
        return "(at " + convertir_valor(m.group(1), ctx) + " " + m.group(2) + ")"
    m = re.match(r"^([\w\.]+)\[(-?\d*):(-?\d*)\]$", v)
    if m:
        base = convertir_valor(m.group(1), ctx)
        a, b = m.group(2), m.group(3)
        args = []
        if a != "":
            args.append(a)
        else:
            args.append("0")
        if b != "":
            args.append(b)
        return "(slice " + base + " " + " ".join(args) + ")"

    # Ruta de variable: nombre.sub.campo
    if re.match(r"^[A-Za-z_]\w*(\.[A-Za-z_]\w*)*$", v):
        return ctx.prefijo(v.split(".")[0]) + v[len(v.split(".")[0]):]

    # Cualquier otra cosa (no debería ocurrir): devolver tal cual
    return v

def aplicar_filtro(base, filtro, ctx):
    filtro = filtro.strip()
    # nombre y argumento (|f:arg, |f(arg) o |f)
    m = re.match(r"^(\w+)\s*:\s*(.+)$", filtro, re.S)
    m2 = re.match(r"^(\w+)\((.*)\)$", filtro, re.S)
    if m:
        nombre, arg = m.group(1), m.group(2)
    elif m2:
        nombre, arg = m2.group(1), m2.group(2)
    else:
        nombre, arg = filtro, None

    if nombre in ("upper", "lower", "title", "miles", "tojson"):
        return "(" + nombre + " " + base + ")"
    if nombre == "length":
        return "(len " + base + ")"
    if nombre == "default":
        return "(or " + base + " " + convertir_valor(arg, ctx) + ")"
    if nombre == "round":
        if arg:
            return "(round " + base + " " + arg.strip() + ")"
        return "(round " + base + ")"
    if nombre == "format":
        # "FMT"|format(x) → (printf "FMT" x)
        return "(printf " + convertir_cadena(base.strip()) + " " + convertir_valor(arg, ctx) + ")"
    if nombre == "slice":
        arg = arg.strip().strip('"').strip("'")
        a, _, b = arg.partition(":")
        args = []
        args.append(a if a else "0")
        if b:
            args.append(b)
        return "(slice " + base + " " + " ".join(args) + ")"
    if nombre in ("int", "float", "string", "str"):
        fn = {"int": "toInt", "float": "toFloat", "string": "str", "str": "str"}[nombre]
        return "(" + fn + " " + base + ")"
    if nombre in ("safe",):
        return "(safeHTML " + base + ")"
    if nombre in ("e", "escape"):
        return base
    # Filtro desconocido: dejar como llamada por si existe en el FuncMap
    return "(" + nombre + " " + base + ")"

def convertir_expr(e, ctx):
    e = e.strip()
    if not e:
        return e

    # Ternario NO se maneja aquí (lo expande el bloque {{ }})

    # or (valor, igual semántica en Go)
    partes = dividir_top(e, " or ")
    if len(partes) > 1:
        conv = [convertir_expr(p, ctx) for p in partes]
        r = conv[-1]
        for p in reversed(conv[:-1]):
            r = "(or " + p + " " + r + ")"
        return r

    # and
    partes = dividir_top(e, " and ")
    if len(partes) > 1:
        conv = [convertir_expr(p, ctx) for p in partes]
        r = conv[-1]
        for p in reversed(conv[:-1]):
            r = "(and " + p + " " + r + ")"
        return r

    # not
    if e.startswith("not "):
        return "(not " + convertir_expr(e[4:], ctx) + ")"

    # is none / is not none / is defined / is not defined
    m = re.match(r"^(.+?)\s+is\s+not\s+none$", e, re.S)
    if m:
        return convertir_expr(m.group(1), ctx)
    m = re.match(r"^(.+?)\s+is\s+none$", e, re.S)
    if m:
        return "(not " + convertir_expr(m.group(1), ctx) + ")"
    m = re.match(r"^(.+?)\s+is\s+not\s+defined$", e, re.S)
    if m:
        return "(not " + convertir_expr(m.group(1), ctx) + ")"
    m = re.match(r"^(.+?)\s+is\s+defined$", e, re.S)
    if m:
        return convertir_expr(m.group(1), ctx)

    # Comparaciones (con funciones que coercionan tipos: eqx, gtx...)
    for op, fn in [(" == ", "eqx"), (" != ", "nex"), (" <= ", "lex"),
                   (" >= ", "gex"), (" < ", "ltx"), (" > ", "gtx")]:
        idx = buscar_top(e, op)
        if idx >= 0:
            a = convertir_expr(e[:idx], ctx)
            b = convertir_expr(e[idx + len(op):], ctx)
            return "(" + fn + " " + a + " " + b + ")"

    # Pipeline de filtros
    idx = buscar_top(e, "|")
    if idx >= 0:
        base = convertir_valor(e[:idx], ctx)
        filtros = [f.strip() for f in dividir_top(e[idx:], "|") if f.strip()]
        r = base
        for f in filtros:
            r = aplicar_filtro(r, f, ctx)
        return r

    return convertir_valor(e, ctx)

def convertir_ternario(inner, ctx):
    """A if C else B (paréntesis y filtros opcionales) → if/else/end."""
    base = inner
    filtros = ""
    idx = buscar_top(base, "|")
    if idx >= 0:
        filtros = base[idx:]
        base = base[:idx].strip()
    if base.startswith("(") and paren_cierre(base, 0) == len(base) - 1:
        base = base[1:-1].strip()
    idx_if = buscar_top(base, " if ")
    if idx_if < 0:
        return None
    resto = base[idx_if + 4:]
    idx_else = buscar_top(resto, " else ")
    if idx_else < 0:
        return None
    a = base[:idx_if].strip()
    cond = resto[:idx_else].strip()
    b = resto[idx_else + 6:].strip()

    def con_filtros(expr):
        conv = convertir_expr(expr, ctx)
        if not filtros:
            return conv
        r = conv
        for f in [f.strip() for f in dividir_top(filtros, "|") if f.strip()]:
            r = aplicar_filtro(r, f, ctx)
        return r

    return ("{{ if " + convertir_expr(cond, ctx) + " }}{{ " + con_filtros(a) +
            " }}{{ else }}{{ " + con_filtros(b) + " }}{{ end }}")

# ─────────────────────────────────────────────────────────────────────────────
# Conversión de bloques
# ─────────────────────────────────────────────────────────────────────────────

re_bloque = re.compile(r"\{\{.*?\}\}|\{%.*?%\}|\{#.*?#\}", re.S)

def convertir_bloque(b, ctx):
    # Comentarios {# ... #} → {{/* ... */}}
    if b.startswith("{#"):
        cuerpo = b[2:-2].strip()
        return "{{/* " + cuerpo.replace("*/", "* /") + " */}}"

    if b.startswith("{{"):
        inner = b[2:-2].strip()
        tern = convertir_ternario(inner, ctx)
        if tern is not None:
            return tern
        return "{{ " + convertir_expr(inner, ctx) + " }}"

    # {% ... %}
    inner = b[2:-2].strip()

    m = re.match(r"^if\s+(.+)$", inner, re.S)
    if m:
        return "{{ if " + convertir_expr(m.group(1), ctx) + " }}"
    m = re.match(r"^elif\s+(.+)$", inner, re.S)
    if m:
        return "{{ else if " + convertir_expr(m.group(1), ctx) + " }}"
    if inner == "else":
        return "{{ else }}"
    if inner == "endif":
        return "{{ end }}"

    m = re.match(r"^for\s+(\w+)\s+in\s+(.+)$", inner, re.S)
    if m:
        var, expr = m.group(1), m.group(2)
        ctx.vars.append(var)
        ctx.prof += 1
        # Siempre con índice ($i) para soportar loop.index
        return "{{ range $i, $" + var + " := " + convertir_expr(expr, ctx_sin(ctx, var)) + " }}"
    if inner == "endfor":
        if ctx.vars:
            ctx.vars.pop()
        ctx.prof = max(0, ctx.prof - 1)
        return "{{ end }}"

    m = re.match(r"^set\s+(\w+)\s*=\s*(.+)$", inner, re.S)
    if m:
        var, expr = m.group(1), m.group(2)
        ctx.vars.append(var)
        return "{{ $" + var + " := " + convertir_expr(expr, ctx) + " }}"

    m = re.match(r'^include\s+"([^"]+)"$', inner)
    if m:
        return "{{ template \"" + m.group(1) + "\" . }}"

    # Desconocido: comentario para no romper
    return "{{/* TODO tag no traducido: " + inner.replace("*/", "* /") + " */}}"

def ctx_sin(ctx, var):
    """Contexto temporal sin la variable del propio range (para su expresión)."""
    copia = Ctx()
    copia.vars = [v for v in ctx.vars if v != var]
    copia.prof = ctx.prof - 1
    return copia

def transpilar(src):
    ctx = Ctx()
    return re_bloque.sub(lambda m: convertir_bloque(m.group(0), ctx), src)

# ─────────────────────────────────────────────────────────────────────────────

def main():
    carpeta = sys.argv[1] if len(sys.argv) > 1 else "templates"
    respaldo = carpeta + "_jinja_original"
    os.makedirs(respaldo, exist_ok=True)
    for ruta in sorted(glob.glob(os.path.join(carpeta, "*.html"))):
        nombre = os.path.basename(ruta)
        src = open(ruta).read()
        # Respaldo del original Jinja (solo si no existe)
        dst = os.path.join(respaldo, nombre)
        if not os.path.exists(dst):
            open(dst, "w").write(src)
        out = transpilar(src)
        open(ruta, "w").write(out)
        print(f"  {nombre}: {len(src)} → {len(out)} bytes")
    print("Transpilación completa. Originales en", respaldo)

if __name__ == "__main__":
    main()
