package com.tempodetela.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class Device(
    val id: Long,
    val mac: String = "",
    val hostname: String = "",
    @SerialName("mdns_name") val mdnsName: String = "",
    val vendor: String = "",
    @SerialName("os_guess") val osGuess: String = "",
    @SerialName("kind_guess") val kindGuess: String = "",
    @SerialName("random_mac") val randomMac: Boolean = false,
    val name: String = "",
    val kind: String = "",
    val ipv4: String = "",
    val ipv6: String = "",
    @SerialName("first_seen") val firstSeen: Long = 0,
    @SerialName("last_seen") val lastSeen: Long = 0,
    @SerialName("last_dns") val lastDns: Long = 0,
    val hidden: Boolean = false,
    @SerialName("person_id") val personId: Long? = null,
    /** Nome automático calculado pelo servidor (hostname, DNS reverso, sites acessados, fabricante). */
    val label: String = "",
    val brand: String = "",
    @SerialName("ptr_name") val ptrName: String = "",
    @SerialName("dns_label") val dnsLabel: String = "",
    /** Este servidor e seus containers. */
    val system: Boolean = false,
    /** Cliente pela Tailscale. */
    val remote: Boolean = false,
) {
    /** Nome automático: o que a rede diz do aparelho. */
    val detectedName: String
        get() = label.ifBlank { listOf(mdnsName, hostname, vendor).firstOrNull { it.isNotBlank() } ?: "" }

    /** Servidor e acesso remoto não são aparelhos da casa. */
    val infra: Boolean get() = system || remote

    val displayName: String
        get() = name.ifBlank { detectedName.ifBlank { ipv4.ifBlank { ipv6.ifBlank { "Aparelho $id" } } } }

    val effectiveKind: String get() = kind.ifBlank { kindGuess }
}

@Serializable
data class Person(
    val id: Long,
    val name: String,
    val emoji: String = "",
    val kind: String = "child",
    val monitoring: Boolean = true,
    @SerialName("created_at") val createdAt: Long = 0,
    @SerialName("device_ids") val deviceIds: List<Long> = emptyList(),
    /** Internet pausada até (unix); -1 = sem prazo; 0 = liberada. */
    @SerialName("paused_until") val pausedUntil: Long = 0,
) {
    fun isPaused(now: Long = System.currentTimeMillis() / 1000) = pausedUntil == -1L || pausedUntil > now
}

@Serializable
data class DnsStatus(
    val addr: String = "",
    val queries: Long = 0,
    @SerialName("cache_hits") val cacheHits: Long = 0,
    val blocked: Long = 0,
    val failures: Long = 0,
    @SerialName("last_error") val lastError: String = "",
)

@Serializable
data class LanStatus(
    @SerialName("interface") val iface: String = "",
    val arp: String = "",
    val dhcp: String = "",
    val mdns: String = "",
)

@Serializable
data class ServerStatus(
    val version: String = "",
    val dns: DnsStatus = DnsStatus(),
    @SerialName("dropped_logs") val droppedLogs: Long = 0,
    val lan: LanStatus = LanStatus(),
    val telegram: TelegramStatus = TelegramStatus(),
)

/** Bot do Telegram que manda os alertas; [chats] > 0 = já chegam por lá (o app não notifica em dobro). */
@Serializable
data class TelegramStatus(val bot: String = "", val link: String = "", val chats: Int = 0) {
    val active: Boolean get() = bot.isNotBlank() && chats > 0
}

@Serializable
data class ApiError(val error: String = "")
