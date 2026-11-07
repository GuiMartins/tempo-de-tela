package com.tempodetela.data

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

/** Guarda o pareamento (endereços + token) criptografado no aparelho. */
class ServerStore(context: Context) {
    private val prefs: SharedPreferences = EncryptedSharedPreferences.create(
        context,
        "servidor",
        MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build(),
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )

    private val _config = MutableStateFlow(load())
    val config: StateFlow<ServerConfig?> = _config

    private fun load(): ServerConfig? {
        val token = prefs.getString("token", null) ?: return null
        val urls = prefs.getString("urls", null)?.split("\n")?.filter { it.isNotBlank() }.orEmpty()
        if (urls.isEmpty()) return null
        return ServerConfig(urls, token, prefs.getString("preferred", null))
    }

    fun save(config: ServerConfig) {
        prefs.edit()
            .putString("urls", config.urls.joinToString("\n"))
            .putString("token", config.token)
            .putString("preferred", config.preferred)
            .apply()
        _config.value = config
    }

    fun setPreferred(url: String) {
        val current = _config.value ?: return
        if (current.preferred == url) return
        save(current.copy(preferred = url))
    }

    fun clear() {
        prefs.edit().clear().apply()
        _config.value = null
    }
}
