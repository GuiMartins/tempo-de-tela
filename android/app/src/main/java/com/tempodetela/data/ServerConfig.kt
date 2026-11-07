package com.tempodetela.data

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/** Endereços do servidor (LAN, Tailscale…) e o token de pareamento. */
data class ServerConfig(val urls: List<String>, val token: String, val preferred: String? = null) {
    /** O último endereço que respondeu vem primeiro. */
    fun orderedUrls(): List<String> =
        if (preferred != null && preferred in urls) listOf(preferred) + (urls - preferred) else urls
}

@Serializable
private data class PairPayload(val v: Int = 1, val urls: List<String> = emptyList(), val token: String = "")

object Pairing {
    const val DEFAULT_PORT = 8994
    private val json = Json { ignoreUnknownKeys = true }

    /** Lê o conteúdo do QR code do painel: {"v":1,"urls":[...],"token":"tdt_..."}. */
    fun parseQr(text: String): ServerConfig {
        val payload = try {
            json.decodeFromString(PairPayload.serializer(), text.trim())
        } catch (e: Exception) {
            throw IllegalArgumentException("Este QR code não é de pareamento do Tempo de Tela.")
        }
        val urls = payload.urls.mapNotNull { normalizeUrl(it) }.distinct()
        if (urls.isEmpty() || !payload.token.startsWith("tdt_")) {
            throw IllegalArgumentException("Este QR code não é de pareamento do Tempo de Tela.")
        }
        return ServerConfig(urls, payload.token)
    }

    /** Pareamento digitado: aceita "192.168.0.10", "192.168.0.10:8994" ou a URL inteira. */
    fun manual(address: String, token: String): ServerConfig {
        val url = normalizeUrl(address) ?: throw IllegalArgumentException("Endereço inválido.")
        val t = token.trim()
        if (!t.startsWith("tdt_")) throw IllegalArgumentException("O token começa com tdt_.")
        return ServerConfig(listOf(url), t)
    }

    fun normalizeUrl(raw: String): String? {
        var s = raw.trim().trimEnd('/')
        if (s.isEmpty()) return null
        if (!s.startsWith("http://") && !s.startsWith("https://")) s = "http://$s"
        val url = s.toHttpUrlOrNull() ?: return null
        val hasPort = Regex("^https?://(\\[[^]]+]|[^/:]+):\\d+").containsMatchIn(s)
        val port = if (hasPort) url.port else DEFAULT_PORT
        return url.newBuilder().port(port).encodedPath("/").build().toString().trimEnd('/')
    }
}
