package com.tempodetela.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.tempodetela.TempoApp
import com.tempodetela.data.AlertEvent
import com.tempodetela.data.AppCatalog
import com.tempodetela.data.PersonRules
import com.tempodetela.data.AppUpdate
import com.tempodetela.data.PersonToday
import com.tempodetela.data.Unclassified
import com.tempodetela.data.UsageAlert
import com.tempodetela.data.UsageSummary
import com.tempodetela.data.Device
import com.tempodetela.data.Person
import com.tempodetela.data.ServerConfig
import com.tempodetela.data.ServerStatus
import com.tempodetela.data.buildApi
import com.tempodetela.data.isUnauthorized
import com.tempodetela.data.userMessage
import com.tempodetela.update.InstallState
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import java.time.LocalDate

data class UiState(
    val devices: List<Device> = emptyList(),
    val persons: List<Person> = emptyList(),
    val today: List<PersonToday> = emptyList(),
    val unclassified: List<Unclassified> = emptyList(),
    val catalog: AppCatalog = AppCatalog(),
    val events: List<AlertEvent> = emptyList(),
    val usageAlerts: List<UsageAlert> = emptyList(),
    val status: ServerStatus? = null,
    val loading: Boolean = false,
    val loaded: Boolean = false,
    val error: String? = null,
    val unauthorized: Boolean = false,
)

class AppViewModel(app: Application) : AndroidViewModel(app) {
    private val tempo = app as TempoApp
    private val api get() = tempo.api

    val config: StateFlow<ServerConfig?> = tempo.serverStore.config

    val updater = tempo.updater
    val update: StateFlow<AppUpdate?> = updater.available
    val installState: StateFlow<InstallState> = updater.state

    private val _checkingUpdate = MutableStateFlow(false)
    val checkingUpdate: StateFlow<Boolean> = _checkingUpdate

    fun installUpdate() {
        val u = update.value ?: return
        viewModelScope.launch { updater.install(u) }
    }

    /** Consulta o GitHub; [force] ignora o intervalo de 6 h (botão da aba Servidor). */
    fun checkUpdate(force: Boolean = false) {
        viewModelScope.launch {
            _checkingUpdate.value = force
            val found = updater.check(force)
            _checkingUpdate.value = false
            if (force && found == null) _messages.tryEmit("Você já está na versão mais nova")
        }
    }

    private val _state = MutableStateFlow(UiState())
    val state: StateFlow<UiState> = _state

    private val _messages = MutableSharedFlow<String>(extraBufferCapacity = 4)
    val messages: SharedFlow<String> = _messages

    init {
        viewModelScope.launch {
            config.map { it?.token }.distinctUntilChanged().collect { token ->
                _state.value = UiState()
                if (token != null) refresh()
            }
        }
    }

    fun refresh() {
        viewModelScope.launch {
            _state.update { it.copy(loading = true) }
            try {
                load()
            } catch (e: Exception) {
                _state.update { it.copy(error = e.userMessage(), unauthorized = e.isUnauthorized) }
            } finally {
                _state.update { it.copy(loading = false) }
            }
        }
    }

    private suspend fun load() = coroutineScope {
        val devices = async { api.devices() }
        val persons = async { api.persons() }
        val status = async { api.status() }
        // Servidor antigo (sem catálogo) não deve derrubar o resto.
        val today = async { runCatching { api.usageToday() }.getOrDefault(emptyList()) }
        val unclassified = async { runCatching { api.unclassified() }.getOrDefault(emptyList()) }
        val catalog = async { runCatching { api.catalog() }.getOrDefault(AppCatalog()) }
        val events = async { runCatching { api.events(limit = 20) }.getOrDefault(emptyList()) }
        val usageAlerts = async { runCatching { api.usageAlerts() }.getOrDefault(emptyList()) }
        _state.update {
            it.copy(
                devices = devices.await(), persons = persons.await(), status = status.await(),
                today = today.await(), unclassified = unclassified.await(), catalog = catalog.await(),
                events = events.await().reversed(), usageAlerts = usageAlerts.await(),
                loaded = true, error = null, unauthorized = false,
            )
        }
    }

    suspend fun personUsage(id: Long, date: LocalDate, days: Int): Result<UsageSummary> =
        runCatching { api.personUsage(id, date.toString(), days) }

    suspend fun rules(personId: Long): Result<PersonRules> = runCatching { api.rules(personId) }

    /** minutes: >0 pausa por esse tempo, -1 sem prazo, 0 libera. */
    fun pause(person: Person, minutes: Int) = mutate(
        when (minutes) {
            0 -> "Internet de ${person.name} liberada"
            else -> "Internet de ${person.name} pausada"
        },
    ) { api.pause(person.id, buildJsonObject { put("minutes", minutes) }) }

    fun saveRules(personId: Long, rules: PersonRules, onSaved: () -> Unit = {}) = mutate("Regras salvas") {
        api.saveRules(personId, rules)
        onSaved()
    }

    suspend fun deviceUsage(id: Long, date: LocalDate, days: Int): Result<UsageSummary> =
        runCatching { api.deviceUsage(id, date.toString(), days) }

    /** Cria um aviso de uso; appId e category nulos = qualquer uso. */
    fun createUsageAlert(appId: String?, category: String?, deviceId: Long?, personId: Long?) = mutate("Aviso criado") {
        api.createUsageAlert(buildJsonObject {
            appId?.let { put("app_id", it) }
            category?.let { put("category", it) }
            deviceId?.let { put("device_id", it) }
            personId?.let { put("person_id", it) }
        })
    }

    fun deleteUsageAlert(alert: UsageAlert) = mutate("Aviso apagado") {
        val resp = api.deleteUsageAlert(alert.id)
        if (!resp.isSuccessful) throw retrofit2.HttpException(resp)
    }

    /** Classifica um site: em um app do catálogo ou num app novo. */
    fun classify(site: String, appId: String?, newAppName: String?, category: String?) = mutate("Classificado") {
        api.createRule(buildJsonObject {
            put("domain", site)
            if (appId != null) {
                put("app_id", appId)
            } else {
                put("new_app", buildJsonObject {
                    put("name", newAppName ?: site)
                    put("category", category ?: "other")
                })
            }
        })
    }

    /** Executa a alteração, recarrega os dados e avisa o erro, se houver. */
    private fun mutate(success: String? = null, block: suspend () -> Unit) {
        viewModelScope.launch {
            try {
                block()
                load()
                success?.let { _messages.tryEmit(it) }
            } catch (e: Exception) {
                _messages.tryEmit(e.userMessage())
                if (e.isUnauthorized) _state.update { it.copy(unauthorized = true) }
            }
        }
    }

    /** Testa os endereços do QR e guarda o pareamento se algum responder. Devolve o erro ou null. */
    suspend fun pair(candidate: ServerConfig): String? {
        var working: String? = null
        val testApi = buildApi(config = { candidate }, onWorking = { working = it })
        return try {
            testApi.status()
            tempo.serverStore.save(candidate.copy(preferred = working ?: candidate.urls.first()))
            null
        } catch (e: Exception) {
            if (e.isUnauthorized) "O servidor recusou o token. Gere outro QR code no painel." else e.userMessage()
        }
    }

    fun unpair() = tempo.serverStore.clear()

    fun saveDevice(device: Device, name: String, kind: String, personId: Long?) = mutate("Aparelho salvo") {
        if (name.trim() != device.name || kind != device.kind) {
            api.patchDevice(device.id, buildJsonObject {
                put("name", name.trim())
                put("kind", kind)
            })
        }
        if (personId != device.personId) assignNow(device.id, personId)
    }

    fun setHidden(device: Device, hidden: Boolean) =
        mutate(if (hidden) "Aparelho ocultado" else "Aparelho visível de novo") {
            api.patchDevice(device.id, buildJsonObject { put("hidden", hidden) })
        }

    fun assign(deviceId: Long, personId: Long?) = mutate { assignNow(deviceId, personId) }

    private suspend fun assignNow(deviceId: Long, personId: Long?) {
        api.assignDevice(deviceId, JsonObject(mapOf("person_id" to (personId?.let { JsonPrimitive(it) } ?: JsonNull))))
    }

    fun savePerson(
        id: Long?, name: String, emoji: String, kind: String, monitoring: Boolean,
        onSaved: (Person) -> Unit = {},
    ) {
        viewModelScope.launch {
            try {
                val body = buildJsonObject {
                    put("name", name.trim())
                    put("emoji", emoji.trim())
                    put("kind", kind)
                    put("monitoring", monitoring)
                }
                val saved = if (id == null) api.createPerson(body) else api.patchPerson(id, body)
                load()
                _messages.tryEmit(if (id == null) "${saved.name} cadastrada" else "Salvo")
                onSaved(saved)
            } catch (e: Exception) {
                _messages.tryEmit(e.userMessage())
            }
        }
    }

    fun deletePerson(person: Person) = mutate("${person.name} removida") {
        val resp = api.deletePerson(person.id)
        if (!resp.isSuccessful) throw retrofit2.HttpException(resp)
    }
}
