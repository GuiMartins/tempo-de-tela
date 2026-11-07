# tempo-de-tela

App self-hosted de controle parental para o ZimaOS/CasaOS: um DNS local que identifica os
aparelhos da rede de casa, vincula cada um a uma pessoa e mostra tempo de tela por app e horário,
com um app Android para cadastro e visualização. Tudo roda e fica no servidor da casa.

> **Status:** em desenvolvimento, pausado. Veja o [PLANO.md](PLANO.md).

## O que já existe

Em `backend/` (Go):

- `internal/dnsserver`: servidor DNS (UDP/TCP) que repassa pro upstream (IP ou DoH), com cache,
  limite de TTL entregue aos aparelhos, failover entre upstreams e bloqueio de domínios usados
  pra contornar o DNS da rede (DoH conhecidos, domínio-sinal do Firefox, iCloud Private Relay).
- `internal/store`: SQLite (WAL) com o esquema de aparelhos, pessoas, vínculos e consultas.
- `internal/settings`: configuração editável, validada e persistida.
- `internal/oui`: fabricante pelo prefixo do MAC (base IEEE embutida; `scripts/update-oui.sh` regenera).
- `internal/inventory`: regras pra estimar sistema e tipo do aparelho.

## Testes

```bash
cd backend && go test ./...
```

## Licença

MIT
