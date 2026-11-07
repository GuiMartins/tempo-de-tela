# tempo-de-tela

App self-hosted de controle parental para o ZimaOS/CasaOS: um DNS local que identifica os
aparelhos da rede de casa, vincula cada um a uma pessoa e mostra tempo de tela por app e horário,
com um app Android para cadastro e visualização. Tudo roda e fica no servidor da casa.

> **Status:** fase 1 em andamento. Veja o [PLANO.md](PLANO.md).

## O que já existe

Em `backend/` (Go):

- `internal/dnsserver`: servidor DNS (UDP/TCP) que repassa pro upstream (IP ou DoH), com cache,
  limite de TTL entregue aos aparelhos, failover entre upstreams e bloqueio de domínios usados
  pra contornar o DNS da rede (DoH conhecidos, domínio-sinal do Firefox, iCloud Private Relay).
- `internal/store`: SQLite (WAL) com o esquema de aparelhos, pessoas, vínculos e consultas.
- `internal/settings`: configuração editável, validada e persistida.
- `internal/oui`: fabricante pelo prefixo do MAC (base IEEE embutida; `scripts/update-oui.sh` regenera).
- `internal/inventory`: regras pra estimar sistema e tipo do aparelho.

## Rodar localmente

```bash
cd backend
go run ./cmd/tempo-de-tela
```

O DNS usa `127.0.0.1:1053` (UDP e TCP) e o painel fica em <http://127.0.0.1:8994>.
O painel e `GET /api/v1/devices` mostram a cobertura dos aparelhos. As consultas vão para o
SQLite em `./data` de forma assíncrona. As opções `-dns`, `-http` e `-data` permitem
alterar esses padrões.

No Linux, `-lan` ativa a varredura ARP, a escuta passiva de DHCPv4 e a descoberta mDNS via
IPv4. ARP e DHCP passivo precisam de `CAP_NET_RAW`. A interface vem da rota IPv4 padrão,
ou pode ser selecionada com `-lan-interface`. Em redes maiores que /24, a varredura ativa
fica no /24 do servidor. O painel exibe o estado de cada coletor.

As consultas de um IP com associação recente entram no histórico do MAC confirmado.
Consultas anteriores à confirmação, de IPs sem associação ou via IPv6 ficam no histórico
por IP.

## Testes

```bash
cd backend && go test ./...
```

## Licença

MIT
