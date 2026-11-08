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

Cada aparelho ganha um nome automático com o que a rede já sabe, nesta ordem: hostname do
DHCP, DNS reverso do roteador (hostname da concessão) ou do MagicDNS da Tailscale, nome mDNS,
o que os domínios consultados dizem (`bambulab.com` → impressora Bambu Lab, `lgthinq.com` →
eletrodoméstico LG) e, por fim, tipo + fabricante. Nomes que não dizem nada (MAC, UUID,
`Android_XXXX`) são ignorados. As consultas do próprio servidor e dos containers Docker vão para
um registro só ("Este servidor").

As consultas de um IP com associação recente entram no histórico do MAC confirmado.
O IP continua do último MAC por até 24 h depois de a varredura deixar de vê-lo (celular
dormindo não responde ARP), enquanto outro MAC não assumir o IP. Consultas que chegaram antes
da confirmação são juntadas ao aparelho de hora em hora.

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
- `GET /api/v1/catalog`: categorias e apps (embutidos em `backend/internal/catalog/catalog.yaml`
  e os criados pelo usuário).
- `GET /api/v1/catalog/unclassified?days=7`: sites acessados que não estão no catálogo.
- `POST /api/v1/catalog/rules`: `{"domain", "app_id"}` ou `{"domain", "new_app": {"name",
  "category"}}`; `DELETE /api/v1/catalog/rules/{domain}`.
- `GET /api/v1/persons/{id}/usage?date=AAAA-MM-DD&days=1|7` (e `/devices/{id}/usage`): tempo
  de tela estimado, por categoria, por app, por dia, sessões do dia e mapa de calor da semana.
- `GET /api/v1/usage/today`: tempo de hoje de cada pessoa, os 3 apps mais usados, o app em uso
  agora e o progresso dos limites.
- `GET|PUT /api/v1/persons/{id}/rules`: `{"limits": {"games": 120, "total": 240}, "bedtime":
  {"start": "22:00", "end": "06:00"}, "school": {"days": [1,2,3,4,5], "start": "07:00",
  "end": "12:30"}}` (minutos por dia; dias 1 = segunda).
- `POST /api/v1/persons/{id}/pause`: `{"minutes": 60}` pausa a internet da pessoa por 1 h; `-1`
  sem prazo; `0` libera.
- Regras com bloqueio: `"block_on_limit": true` (categoria que passou do limite fica bloqueada até
  o fim do dia; o limite total bloqueia tudo), `"block_categories": ["games"]` (sempre),
  `"bedtime": {…, "block": true}` e `"school": {…, "block": true}` (vídeo, jogos, redes, música e
  compras). O bloqueio é no DNS (responde "não existe"); serviços do sistema continuam liberados.
- `GET /api/v1/events?after=ID`: alertas novos (limite estourado, uso na hora de dormir ou na
  aula, app usado pela primeira vez, aparelho novo na rede), avaliados a cada 5 min. O app
  Android consulta a cada 15 min e mostra como notificação, sem Firebase.
- `GET /healthz`: sem autenticação, para o healthcheck.

O tempo de tela é estimado pelas consultas DNS: consultas do mesmo app no mesmo aparelho formam
uma sessão enquanto o intervalo for menor que o da categoria (vídeo e jogos 10 min, mensagens
4 min, demais 5–8 min); a sessão vai até 1 min depois da última consulta, e consulta isolada
(sincronização em segundo plano) não conta. Apps abertos ao mesmo tempo não somam em dobro.

## App Android

Em `android/` (Kotlin, Jetpack Compose, Material3). Pareia lendo o QR code do painel (ou com
endereço + token digitados) e guarda o token criptografado no aparelho. Tenta os endereços
do servidor em ordem (LAN em casa, Tailscale fora) e lembra o último que respondeu.

- **Aparelhos:** sem dono em destaque, agrupados por pessoa, online agora, quem não usa o DNS
  da casa; renomear, tipo, dono e ocultar.
- **Pessoas:** cadastro (criança, adulto, casa), emoji, pausar o registro, vincular aparelhos.
- **Servidor:** endereço em uso, contadores do DNS, estado dos coletores, desparear.

```bash
cd android
./dev.sh run     # build + instala + abre no aparelho/emulador conectado
./dev.sh test    # testes unitários
```

O app se atualiza sozinho: consulta a última Release do GitHub (no máximo a cada 6 h), mostra
um aviso com "Atualizar" e instala o APK por cima, pedindo uma vez a liberação do Android para
instalar apps desta fonte.

O GitHub Actions builda o APK debug a cada mudança em `android/` e, em tag `v*`, anexa o APK
release assinado na Release (precisa dos secrets `RELEASE_KEYSTORE_BASE64`,
`RELEASE_STORE_PASSWORD`, `RELEASE_KEY_ALIAS` e `RELEASE_KEY_PASSWORD`).

## Testes

```bash
cd backend && go test ./...
```

## Licença

MIT
