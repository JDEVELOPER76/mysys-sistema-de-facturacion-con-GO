#!/usr/bin/env python3
"""Ajustes manuales post-transpilación (no traducibles automáticamente)."""

# 1. admin_estadistica.html: cap de pct con minN (Go no reasigna vars entre scopes)
p = 'templates/admin_estadistica.html'
s = open(p).read()
viejo = """{{ $pct := ((or $empleado.porcentaje 0)) }}
                                            {{ if (gtx $pct 100) }}{{ $pct := 100 }}{{ end }}"""
nuevo = """{{ $pct := (minN (or $empleado.porcentaje 0) 100.0) }}"""
assert viejo in s, "patron pct no encontrado"
s = s.replace(viejo, nuevo)
open(p, 'w').write(s)

# 2. admin_ventas.html: guardia para fecha_completa_dt (puede faltar si la fecha no parsea)
p = 'templates/admin_ventas.html'
s = open(p).read()
viejo = '{{ ($venta.fecha_completa_dt.Format "02/01/2006 15:04") }}'
nuevo = '{{ if $venta.fecha_completa_dt }}{{ ($venta.fecha_completa_dt.Format "02/01/2006 15:04") }}{{ else }}{{ $venta.fecha_completa }}{{ end }}'
assert viejo in s, "patron fecha no encontrado"
s = s.replace(viejo, nuevo)
open(p, 'w').write(s)
print("Post-ediciones OK")
