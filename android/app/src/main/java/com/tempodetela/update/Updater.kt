package com.tempodetela.update

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.os.Build
import android.util.Log
import com.tempodetela.data.AppUpdate
import com.tempodetela.data.GitHubReleases
import com.tempodetela.data.isNewer
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

/** Onde está a instalação de uma versão nova. */
sealed interface InstallState {
    data object Idle : InstallState

    /** O Android só deixa instalar depois de liberar o app como fonte, uma vez. */
    data object NeedsPermission : InstallState

    data class Downloading(val fraction: Float) : InstallState

    data object Installing : InstallState

    data class Failed(val message: String) : InstallState
}

/**
 * Atualiza o app pelas releases do GitHub (mesmo esquema do app Jardim): consulta a última no
 * máximo a cada 6 h e, quando a pessoa toca em "Atualizar", baixa o APK direto para uma sessão do
 * PackageInstaller. A resposta do Android chega no InstallReceiver.
 */
class Updater(private val context: Context) {
    private val prefs = context.getSharedPreferences("atualizacao", Context.MODE_PRIVATE)
    private val json = Json { ignoreUnknownKeys = true }

    val installedVersion: String =
        context.packageManager.getPackageInfo(context.packageName, 0).versionName ?: "0"

    private val _available = MutableStateFlow(cached()?.takeIf { isNewer(it.version, installedVersion) })

    /** A versão nova, se a última consultada for mais nova que a instalada. */
    val available: StateFlow<AppUpdate?> = _available

    private val _state = MutableStateFlow<InstallState>(InstallState.Idle)
    val state: StateFlow<InstallState> = _state

    private val lock = Mutex()

    /** [force] ignora o intervalo de 6 h. Sem rede, fica valendo a última consulta. */
    suspend fun check(force: Boolean = false): AppUpdate? = lock.withLock {
        val last = prefs.getLong("consultado_em", 0)
        if (force || System.currentTimeMillis() - last >= INTERVAL_MS) {
            try {
                val update = withContext(Dispatchers.IO) { GitHubReleases.parse(fetchText(GitHubReleases.LATEST)) }
                prefs.edit()
                    .putString("ultima", update?.let { json.encodeToString(AppUpdate.serializer(), it) })
                    .putLong("consultado_em", System.currentTimeMillis())
                    .apply()
                _available.value = update?.takeIf { isNewer(it.version, installedVersion) }
            } catch (e: Exception) {
                Log.w("Updater", "não deu para consultar a release", e)
            }
        }
        _available.value
    }

    suspend fun install(update: AppUpdate) {
        if (_state.value is InstallState.Downloading || _state.value == InstallState.Installing) return
        if (!context.packageManager.canRequestPackageInstalls()) {
            _state.value = InstallState.NeedsPermission
            return
        }
        _state.value = InstallState.Downloading(0f)
        try {
            withContext(Dispatchers.IO) { downloadIntoSession(update) }
            _state.value = InstallState.Installing
        } catch (e: IOException) {
            Log.w("Updater", "download falhou", e)
            _state.value = InstallState.Failed("Não deu para baixar a versão ${update.version}. Confira a internet e tente de novo.")
        }
    }

    /** O que o InstallReceiver ouviu do Android quando a instalação não foi até o fim. */
    fun failed(status: Int, message: String?) {
        Log.w("Updater", "instalação: status $status, $message")
        _state.value = when (status) {
            // Tocou em "Cancelar" na confirmação: volta ao botão, sem erro.
            PackageInstaller.STATUS_FAILURE_ABORTED -> InstallState.Idle
            PackageInstaller.STATUS_FAILURE_CONFLICT, PackageInstaller.STATUS_FAILURE_INCOMPATIBLE ->
                InstallState.Failed("A versão nova foi assinada com outra chave e não instala por cima desta.")
            PackageInstaller.STATUS_FAILURE_STORAGE -> InstallState.Failed("Falta espaço no celular para instalar a versão nova.")
            else -> InstallState.Failed("O Android não instalou a versão nova${message?.let { " ($it)" } ?: ""}.")
        }
    }

    fun clearError() {
        if (_state.value is InstallState.Failed || _state.value == InstallState.NeedsPermission) {
            _state.value = InstallState.Idle
        }
    }

    private fun cached(): AppUpdate? = prefs.getString("ultima", null)?.let {
        try {
            json.decodeFromString(AppUpdate.serializer(), it)
        } catch (e: Exception) {
            null
        }
    }

    private fun downloadIntoSession(update: AppUpdate) {
        val installer = context.packageManager.packageInstaller
        val params = PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL).apply {
            setAppPackageName(context.packageName)
            // Android 12+: quem instalou a versão atual pode atualizar sem pedir confirmação.
            // Se a versão atual veio de outro lugar (navegador, adb), o Android pergunta assim mesmo.
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                setRequireUserAction(PackageInstaller.SessionParams.USER_ACTION_NOT_REQUIRED)
            }
        }
        val id = installer.createSession(params)
        try {
            installer.openSession(id).use { session ->
                val conn = open(update.apk)
                try {
                    val total = conn.contentLengthLong.takeIf { it > 0 } ?: update.size
                    session.openWrite("tempo-de-tela.apk", 0, total).use { out ->
                        conn.inputStream.use { input ->
                            val buffer = ByteArray(64 * 1024)
                            var read = 0L
                            while (true) {
                                val n = input.read(buffer)
                                if (n < 0) break
                                out.write(buffer, 0, n)
                                read += n
                                _state.value = InstallState.Downloading((read.toFloat() / total).coerceAtMost(1f))
                            }
                        }
                        session.fsync(out)
                    }
                } finally {
                    conn.disconnect()
                }
                // Mutável: o Android põe o status e a tela de confirmação nos extras.
                val reply = PendingIntent.getBroadcast(
                    context,
                    id,
                    Intent(context, InstallReceiver::class.java).setPackage(context.packageName),
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_MUTABLE,
                )
                session.commit(reply.intentSender)
            }
        } catch (e: IOException) {
            installer.abandonSession(id)
            throw e
        }
    }

    private fun open(url: String, accept: String? = null): HttpURLConnection {
        // O GitHub redireciona o download para outro domínio, também https: o HttpURLConnection segue.
        val conn = URL(url).openConnection() as HttpURLConnection
        conn.connectTimeout = 10_000
        conn.readTimeout = 30_000
        accept?.let { conn.setRequestProperty("Accept", it) }
        if (conn.responseCode != HttpURLConnection.HTTP_OK) {
            conn.disconnect()
            throw IOException("HTTP ${conn.responseCode} em $url")
        }
        return conn
    }

    private fun fetchText(url: String): String {
        val conn = open(url, accept = "application/vnd.github+json")
        try {
            return conn.inputStream.bufferedReader().use { it.readText() }
        } finally {
            conn.disconnect()
        }
    }

    private companion object {
        const val INTERVAL_MS = 6 * 60 * 60 * 1000L
    }
}
