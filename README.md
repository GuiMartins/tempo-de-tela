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
- `internal/collectors`: ARP, DHCPv4 passivo e mDNS na LAN (Linux) pra identificar aparelhos pelo MAC.
- `internal/auth`: senha do painel (PBKDF2), sessões e tokens de pareamento da API.
- `internal/web`: painel (aparelhos, configurações, pareamento por QR code, senha) e API `/api/v1`.

## Instalar no CasaOS / ZimaOS

O app usa a rede do host: DNS na porta 53 e painel na 8994. Pare antes qualquer outro DNS
da máquina (Pi-hole, AdGuard Home). Com o [`casaos/docker-compose.yml`](casaos/docker-compose.yml):

```bash
casaos-cli app-management install -f casaos/docker-compose.yml
```

No primeiro acesso a `http://<servidor>:8994`, o painel pede pra definir a senha. Para
apagar a senha esquecida:

```bash
docker exec tempo-de-tela /tempo-de-tela -reset-password
```

Depois, no DHCP do roteador, coloque o IP do servidor como DNS (o painel tem as instruções).
Se o servidor cair, a casa fica sem DNS: volte o DNS do roteador para automático.

## Rodar localmente

```bash
cd backend
go run ./cmd/tempo-de-tela
```

O DNS usa `127.0.0.1:1053` (UDP e TCP) e o painel fica em <http://127.0.0.1:8994>.
As consultas vão para o SQLite em `./data` de forma assíncrona. As opções `-dns`, `-http`,
`-data`, `-lan` e `-lan-interface` (ou `TDT_DNS`, `TDT_HTTP`, `TDT_DATA`, `TDT_LAN=1`,
`TDT_LAN_INTERFACE`) mudam esses padrões.

No Linux, `-lan` ativa a varredura ARP, a escuta passiva de DHCPv4 e a descoberta mDNS via
IPv4. ARP e DHCP passivo precisam de `CAP_NET_RAW`. A interface vem da rota IPv4 padrão,
ou pode ser selecionada com `-lan-interface`. Em redes maiores que /24, a varredura ativa
fica no /24 do servidor. O painel exibe o estado de cada coletor.

As consultas de um IP com associação recente entram no histórico do MAC confirmado.
Consultas anteriores à confirmação, de IPs sem associação ou via IPv6 ficam no histórico
por IP.

## API

Autenticação por `Authorization: Bearer <token>`, com o token gerado no pareamento do
painel (o QR code leva `{"v":1,"urls":[...],"token":"tdt_..."}`). A sessão do painel
também vale.

- `GET /api/v1/status`: versão, contadores do DNS e estado dos coletores.
- `GET /api/v1/devices` (`?filter=all` inclui os ocultos), `GET /api/v1/devices/{id}`.
- `PATCH /api/v1/devices/{id}`: `{"name", "kind", "hidden"}`, só os campos enviados.
- `PUT /api/v1/devices/{id}/person`: `{"person_id": 3}` ou `{"person_id": null}`. O primeiro
  vínculo de um aparelho vale desde que ele apareceu na rede; as trocas valem a partir de agora.
- `GET|POST /api/v1/persons`, `GET|PATCH|DELETE /api/v1/persons/{id}`: `{"name", "emoji",
  "kind": "child|adult|shared", "monitoring"}`. Com `monitoring` falso, as consultas dos
  aparelhos da pessoa não são gravadas.
- `GET /healthz`: sem autenticação, para o healthcheck.

## Testes

```bash
cd backend && go test ./...
```

## Licença

MIT
