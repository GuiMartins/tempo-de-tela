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
) {
    /** Nome automático: o que a rede diz do aparelho. */
    val detectedName: String
        get() = listOf(mdnsName, hostname, vendor).firstOrNull { it.isNotBlank() } ?: ""

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
)

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
)

@Serializable
data class ApiError(val error: String = "")
