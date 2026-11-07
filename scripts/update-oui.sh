#!/usr/bin/env bash
# Regenera a base de fabricantes (OUI/MA-L da IEEE) embutida no backend.
set -euo pipefail
cd "$(dirname "$0")/.."
curl -sfL -A "Mozilla/5.0 (tempo-de-tela)" https://standards-oui.ieee.org/oui/oui.csv |
python3 -c '
import csv, sys
rows = {}
for r in csv.reader(sys.stdin):
    if len(r) < 3 or r[0] != "MA-L" or len(r[1]) != 6:
        continue
    name = " ".join(r[2].split())
    rows[r[1].upper()] = name
for k in sorted(rows):
    print(f"{k}\t{rows[k]}")
' | gzip -9n > backend/internal/oui/oui.tsv.gz
echo "ok: $(gzip -dc backend/internal/oui/oui.tsv.gz | wc -l) fabricantes"
