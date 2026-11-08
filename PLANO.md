# Plano: app de controle parental / tempo de tela para o ZimaOS

Nome: **`tempo-de-tela`**.

## Status (08/11/2026)

**Todas as fases do plano no ar (1 a 6).** Backend no zima como DNS da casa (no lugar do pihole), com coletores
ARP/DHCP/mDNS, nomes automáticos dos aparelhos (DHCP, DNS reverso do roteador e da Tailscale,
mDNS, assinatura pelos domínios), painel web com senha e API. App Android (Releases no GitHub,
atualiza sozinho) com pareamento por QR, pessoas, aparelhos e vínculos.

Fase 4: catálogo embutido domínio → app → categoria (`backend/internal/catalog/catalog.yaml`),
classificação dos sites desconhecidos pelo app (regras e apps do usuário no banco), sessões
estimadas por aparelho e app e, no app, a aba Hoje (tempo de cada pessoa e top apps) e a tela da
pessoa (dia: total, categorias, linha do tempo, apps; semana: barras por dia, mapa de calor).

Fase 5: regras por pessoa (limite diário por categoria e total, hora de dormir, horário de
aula), alertas avaliados no servidor a cada 5 min (limite, hora de dormir, aula, app novo,
aparelho novo) e notificações no app pelo WorkManager; aba Hoje com limites e "agora: app";
tela da pessoa com comparação com o dia/semana anterior e as regras.

Fase 6: bloqueio no DNS por pessoa (motor em memória, decidido em toda consulta): pausar a
internet (prazo ou sem prazo), bloquear na hora de dormir, bloquear vídeo/jogos/redes na aula,
categorias sempre bloqueadas e categoria que passou do limite (o total bloqueia tudo). Serviços do
sistema ficam liberados. No app: pausar/liberar na tela da pessoa e as opções nas regras.

Fase 2 (08/11/2026): no roteador (Xiaomi AX6000, firmware 1.0.122), o DNS2 do DHCP (`8.8.8.8`)
foi removido e o IPv6 foi desligado (o firmware não deixa desligar só o anúncio de DNS IPv6, e os
aparelhos Apple preferiam o DNS IPv6 do roteador). Resultado: 16 de 17 aparelhos vistos na rede
usando o DNS da casa (o que falta é o próprio roteador); o iPhone que tinha zero consultas passou a
ter mais de mil em 6 h. Voltar atrás: DNS2 `8.8.8.8` em 局域网设置 e o interruptor "IPV6网络设置"
em 上网设置.

Avisos de uso + ntfy (08/11/2026): avisos cadastrados no app ("me avise quando <app | categoria |
qualquer uso> for usado em <aparelho | pessoa>"), um alerta cada vez que começa um bloco de uso,
avaliados a cada 1 min. Todos os alertas são publicados num ntfy da casa (container `ntfy`,
porta 8996, tópico aleatório gerado pelo servidor) e chegam no celular em 1 a 3 min; fora de casa
precisa da Tailscale ligada no celular (o ntfy guarda 24 h). Com ntfy, o app para de notificar
sozinho pra não chegar em dobro.

Limites do bloqueio por DNS: conexões já abertas continuam até o app resolver o nome de novo (o
TTL entregue é de no máximo 60 s); dados móveis e DNS IPv6 do roteador escapam; a busca do
Google (google.com) está em "sistema" e não é bloqueada na pausa. Descoberta por SSDP/NetBIOS ficou de fora (os nomes já vêm de outras fontes).
Limitação conhecida: apps de computador que consultam o servidor o tempo todo em segundo plano
(ChatGPT, Claude, WhatsApp Desktop) aparecem com mais tempo do que o uso real.

Voltar ao Pi-hole: `docker stop tempo-de-tela`, `docker update --restart unless-stopped pihole`,
`docker start pihole`.

Alternativa pronta considerada: AdGuard Home (clientes nomeados, serviços bloqueados por aparelho,
agenda, controle parental e busca segura).

## Contexto

Controle parental da casa: descobrir os aparelhos do Wi-Fi, vincular cada um a uma pessoa e mostrar,
por pessoa, quanto tempo de tela, em quais apps/serviços e em que horários. Todos em casa sabem do
monitoramento. Backend no zima com UI web mínima só de configuração; o app Android é onde se cadastra
gente, vincula aparelho e vê as análises.

**Sem dependência de Pi-hole:** o próprio app é o servidor DNS da casa, no mesmo modelo do Pi-hole
(responde as consultas, registra quem perguntou o quê e quando, e pode bloquear).

Terreno levantado:
- Zima: x86_64, ZimaOS 1.7.1, 7,5 GB de RAM (vive no limite), deploy padrão por imagem no GHCR +
  `casaos/docker-compose.yml` + `casaos-cli apply`.
- Roteador: Xiaomi AX6000 (RA72, firmware chinês), sem API, atrás de CGNAT.
- **O DHCP do roteador já entrega o IP do servidor (o zima) como DNS primário** e `8.8.8.8` como
  secundário (o próprio zima recebe isso por DHCP). Ou seja: não precisa o app fazer DHCP.
- O roteador também anuncia **ele mesmo como DNS IPv6**. Aparelho com IPv6
  pode perguntar pro roteador e escapar do app. Resolver na fase 2 (ajustes do roteador).
- O container `pihole` está rodando de novo (subiu há pouco) e ocupa a porta 53. Na hora do deploy ele
  precisa parar.
- Repos de referência: `streamarr` (casaos compose, workflow GHCR amd64+arm64, UI em um HTML só),
  `hardlink-audit` (`network_mode: host`), `casshole` (Android Kotlin + Compose + Material3, `dev.sh`).

## Como os dados chegam

| Fonte | O que dá | Como |
|---|---|---|
| **DNS próprio** (porta 53) | quais apps/serviços cada aparelho usa, quando, por quanto tempo (estimado) | o app responde as consultas (repassa pro upstream, com cache) e grava cliente + domínio + hora |
| **DHCP passivo** | MAC, hostname, fabricante e "impressão digital" do sistema (Android, iOS, Windows, PlayStation…) | escuta os broadcasts de DHCP da rede sem ser o servidor DHCP (o roteador continua sendo) |
| **Varredura da LAN** | lista de aparelhos e quem está online agora | ARP scan da /24 a cada 2 min, mDNS/SSDP/NetBIOS pra nome e tipo, base OUI da IEEE pra fabricante |

Só metadados: domínio e horário. Sem conteúdo, sem interceptar HTTPS.

**Limites (expectativa):**
- Não vê página, vídeo específico, mensagem nem busca, só o serviço (YouTube, TikTok, Roblox…).
- Não vê volume em bytes por aparelho (precisaria de roteador com OpenWrt/NetFlow).
- 4G/dados móveis fora da rede: invisível.
- DNS privado/DoH nos aparelhos escapa. Mitigação padrão de controle parental, embutida no app:
  responder "não existe" pros domínios de DoH conhecidos (`dns.google`, `cloudflare-dns.com`…) e pro
  domínio-sinal `use-application-dns.net`, o que faz navegador e Android voltarem pro DNS da rede. Nos
  celulares das crianças, "DNS privado = desativado".
- MAC aleatório (Android/iOS): na rede de casa, marcar "usar MAC do aparelho" (ou "fixo") nos aparelhos
  monitorados, senão o mesmo celular vira um aparelho novo de tempos em tempos.

**Ponto único de falha:** sendo o DNS da casa, se o app ou o zima cair, a casa fica sem internet.
Mitigações: cache em memória, gravação no banco assíncrona (banco com problema nunca trava resposta de
DNS), `restart: unless-stopped`, healthcheck. Plano de volta em 1 minuto: no roteador, DNS do DHCP de
volta pra automático.

## Como vira "uso por app"

- **Catálogo domínio → app → categoria**, em YAML versionado no repo e editável pelo app Android.
  Ex.: YouTube (`googlevideo.com`, `ytimg.com`), TikTok, Instagram, WhatsApp, Discord, Roblox, Fortnite/Epic,
  Minecraft, PSN, Xbox Live, Steam, Netflix, Spotify, Google Classroom. Categorias: vídeo, rede social, jogos,
  mensagens, estudo, sistema (telemetria/updates, ignorado nas contas).
- Domínio desconhecido aparece numa lista "não classificados"; classificar no app ensina o catálogo
  (mesma ideia do aprendizado de categoria do casshole).
- **Sessões:** consultas do mesmo app no mesmo aparelho viram sessão; intervalo maior que N min fecha a
  sessão (padrão 5 min, vídeo 10 min). Como o app é o próprio DNS, ele controla o TTL das respostas
  (ex.: no máximo 60 s pros domínios do catálogo), o que deixa a duração bem mais precisa que só ler log.
  Ainda é **estimada** e o app mostra isso.
- **Aparelho → pessoa** com data de início, pra quando um aparelho troca de dono. Aparelho compartilhado
  (TV da sala) vai pra pessoa "Casa".

## Análises e insights

Por pessoa:
- Tempo de tela por dia/semana, por app e por categoria; top apps.
- Linha do tempo do dia (barras por app, início e fim de cada sessão).
- Primeira e última atividade online do dia, comparadas com o horário combinado (ex.: "sem tela depois
  das 22h") e alerta de uso de madrugada.
- Uso em horário de aula (horário configurável).
- Mapa de calor hora × dia da semana; dia útil vs fim de semana; esta semana vs a anterior.
- App novo (primeira vez que aparece pra pessoa).
- Limite diário por categoria (ex.: 2 h de jogos) com alerta quando estoura.

Casa:
- Quem está online agora e em quê.
- Aparelho desconhecido entrou na rede.
- Curiosidade: aparelhos IoT mais "falantes" (telemetria de TV, câmera etc.).

Transparência embutida: cada pessoa tem botão "pausar monitoramento", retenção configurável (padrão:
eventos brutos 90 dias, agregados 1 ano) e nada sai do zima.

## Arquitetura

### Backend (zima), um container
- **Go**: o app vira o DNS da casa inteira, então tem que ser leve e não cair. Binário único, ~30–50 MB
  de RAM (o zima vive no limite), build multi-arquitetura simples.
  - `miekg/dns` (servidor e cliente DNS, a base do CoreDNS), upstream configurável (padrão Cloudflare/Google,
    opção de DoH pra o provedor não ver as consultas).
  - `modernc.org/sqlite` (SQLite sem cgo, modo WAL) em `/appdata`.
  - `insomniacslk/dhcp` (ler DHCP passivo), `mdlayher/arp` + `mdlayher/packet` (ARP scan), `grandcat/zeroconf` (mDNS).
  - `net/http` com o `index.html` da UI embutido no binário.
- `network_mode: host` + `cap_add: [NET_RAW, NET_BIND_SERVICE]`. Portas: **53** udp/tcp (DNS) e **8994** (UI + API).
- `TZ=America/Sao_Paulo`, dados em `/DATA/AppData/tempo-de-tela/data`.
- **UI web mínima** (um HTML, como o streamarr): status do DNS e dos coletores, DNS upstream, sub-rede,
  intervalos, retenção, horários (aula/dormir), instruções do roteador e **QR code de pareamento** do app Android.
- **API REST** `/api/v1/…`: pessoas, aparelhos, vínculos, catálogo, resumos, linha do tempo, insights,
  eventos, regras. Autenticação por token gerado no pareamento (o QR leva URL + token).
- Acesso fora de casa pela **Tailscale** que você já usa (`<IP Tailscale do servidor>:8994`). Nada exposto na internet.

### App Android
- **Kotlin + Jetpack Compose + Material3** (stack do casshole), Retrofit/OkHttp + kotlinx.serialization,
  gráficos com Vico, WorkManager, DataStore (URL + token), leitura de QR com ZXing.
- Sem Room no começo: o backend é a fonte da verdade.
- Notificações sem Firebase (repo público, sem serviço externo): WorkManager consulta `/api/v1/events`
  a cada 15 min e gera notificação local.
- Telas: **Início** (online agora, resumo de hoje por pessoa, alertas) · **Pessoas** (CRUD, emoji/foto,
  criança/adulto) · **Pessoa** (dia/semana, apps, timeline, primeira/última atividade, mapa de calor) ·
  **Aparelhos** (descobertos, não vinculados em destaque, renomear, tipo, vincular) · **Apps** (classificar
  domínios desconhecidos) · **Regras** (limites, hora de dormir, pausar) · **Configurações** (servidor, notificações).
- APK gerado pelo GitHub Actions e anexado na Release (tag `v*`). `dev.sh` copiado do casshole.

### Repositório
Público `GuiMartins/tempo-de-tela`, só `main`, commit direto (sem branch, sem PR). Monorepo:
```
backend/           Go: dns/, collectors/ (dhcp, arp, mdns), store/, api/, web/index.html, catalog.yaml
android/           app Compose
casaos/docker-compose.yml   bloco x-casaos, network_mode host, portas 53 e 8994
.github/workflows/docker.yml   adaptado do streamarr, filtra backend/**
.github/workflows/android.yml  build do APK, release em tag
Dockerfile  icon.svg  README.md  LICENSE (MIT)
```
Imagem `ghcr.io/guimartins/tempo-de-tela` (amd64+arm64, pacote público: o CasaOS faz pull na instalação).

## Fases

1. **Backend:** DNS + coletores + SQLite + UI mínima. Deploy no zima: parar o `pihole` e subir o container
   novo (peço seu ok na hora). Como o roteador já aponta pro zima como DNS primário, o app passa a receber
   consultas na hora, **sem mexer no roteador**. Dados começam a acumular enquanto o Android é feito.
   Na UI mínima, uma tela "cobertura" mostra quais aparelhos estão usando o DNS do app e quais não.
2. **Ajustes do roteador** (exigem reiniciar o roteador: fazer num horário combinado, não bloqueia nada):
   - tirar o `8.8.8.8` do DNS secundário do DHCP (até lá, parte das consultas escapa e a cobertura é parcial);
   - DNS IPv6: apontar pro zima ou desligar o anúncio de DNS IPv6. Desligar o IPv6 inteiro quebra a saída
     IPv6 que o metube usa pro YouTube, então é a última opção.
3. **Android base:** pareamento por QR, pessoas, aparelhos, vínculo.
4. **Catálogo + sessões + telas de análise.**
5. **Insights, limites e notificações.**
6. **Ações:** pausar internet, hora de dormir e bloqueio por categoria por pessoa, direto no DNS do app
   (fica fácil por ser o próprio DNS). Opcional: listas de bloqueio de anúncio, como o Pi-hole.

## Verificação

- Ajustes do roteador: depois de mudar, `dig` de um celular mostra o zima respondendo, inclusive por IPv6.
- Backend: testes Go da sessionização, do catálogo e do servidor DNS (respostas, cache, TTL, bloqueio);
  no zima, `dig @<IP do servidor> youtube.com` responde e aparece no log com o IP certo; `/api/v1/devices`
  bate com a lista de clientes do roteador; `docker stats` abaixo de 100 MB; derrubar o banco não derruba o DNS.
- Ponta a ponta: 20 min de YouTube num aparelho vinculado → sessão de ~20 min na timeline da pessoa no app.
- Android: emulador apontando pro zima pela Tailscale; criar pessoa, vincular, ver dados, receber
  notificação de aparelho desconhecido.

## Decisões (fechadas em 07/11/2026)

1. Nome: **`tempo-de-tela`** (repo, imagem e app Android).
2. Coleta: **toggle por pessoa** (coleta completa ligada/desligada no app, vale pra adulto e criança).
3. Fase 6 (pausar, hora de dormir, bloqueio por categoria): **entra, no fim**.
4. Backend em **Go**.
