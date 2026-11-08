package com.tempodetela.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/** Uma versão publicada no GitHub, com o endereço do APK. */
@Serializable
data class AppUpdate(val version: String, val apk: String, val size: Long)

/** As releases do repositório público; o workflow do Android anexa o APK assinado em cada tag v*. */
object GitHubReleases {
    const val LATEST = "https://api.github.com/repos/GuiMartins/tempo-de-tela/releases/latest"

    private val json = Json { ignoreUnknownKeys = true }

    @Serializable
    private data class Release(@SerialName("tag_name") val tag: String, val assets: List<Asset> = emptyList())

    @Serializable
    private data class Asset(val name: String, @SerialName("browser_download_url") val url: String, val size: Long)

    /** A resposta de /releases/latest; null se a release não tem APK. */
    fun parse(text: String): AppUpdate? {
        val release = json.decodeFromString(Release.serializer(), text)
        val apk = release.assets.firstOrNull { it.name.endsWith(".apk") } ?: return null
        return AppUpdate(release.tag.removePrefix("v"), apk.url, apk.size)
    }
}

/** "0.10.0" é mais nova que "0.9.1", número por número. Texto que não é versão nunca é mais novo. */
fun isNewer(candidate: String, installed: String): Boolean {
    val new = versionNumbers(candidate) ?: return false
    val current = versionNumbers(installed) ?: return true
    for (i in 0 until maxOf(new.size, current.size)) {
        val n = new.getOrElse(i) { 0 }
        val c = current.getOrElse(i) { 0 }
        if (n != c) return n > c
    }
    return false
}

private fun versionNumbers(version: String): List<Int>? =
    version.removePrefix("v").substringBefore("-").split('.').map { it.toIntOrNull() ?: return null }.takeIf { it.isNotEmpty() }
