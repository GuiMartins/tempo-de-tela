package com.tempodetela.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class CategoryDef(
    val id: String,
    val label: String,
    @SerialName("gap_min") val gapMin: Int = 5,
    val ignore: Boolean = false,
)

@Serializable
data class AppDef(
    val id: String,
    val name: String,
    val category: String,
    val domains: List<String> = emptyList(),
    val custom: Boolean = false,
)

@Serializable
data class AppCatalog(val categories: List<CategoryDef> = emptyList(), val apps: List<AppDef> = emptyList())

@Serializable
data class Unclassified(val site: String, val count: Int = 0, val devices: Int = 0, val examples: List<String> = emptyList())

@Serializable
data class AppUsage(val id: String, val name: String, val category: String, val sec: Long = 0, val sessions: Int = 0)

@Serializable
data class CategoryUsage(val id: String, val label: String, val sec: Long = 0)

@Serializable
data class DayUsage(val date: String, val sec: Long = 0)

@Serializable
data class SessionView(
    @SerialName("app_id") val appId: String,
    @SerialName("app_name") val appName: String,
    val category: String,
    @SerialName("device_id") val deviceId: Long = 0,
    val start: Long,
    val end: Long,
)

@Serializable
data class UsageSummary(
    val from: Long = 0,
    val to: Long = 0,
    val days: Int = 1,
    @SerialName("total_sec") val totalSec: Long = 0,
    @SerialName("first_activity") val firstActivity: Long = 0,
    @SerialName("last_activity") val lastActivity: Long = 0,
    val categories: List<CategoryUsage> = emptyList(),
    val apps: List<AppUsage> = emptyList(),
    val daily: List<DayUsage> = emptyList(),
    val heatmap: List<List<Int>> = emptyList(),
    val sessions: List<SessionView> = emptyList(),
    val estimated: Boolean = true,
    @SerialName("previous_total_sec") val previousTotalSec: Long = 0,
)

@Serializable
data class PersonToday(
    @SerialName("person_id") val personId: Long,
    @SerialName("total_sec") val totalSec: Long = 0,
    @SerialName("last_activity") val lastActivity: Long = 0,
    @SerialName("top_apps") val topApps: List<AppUsage> = emptyList(),
    @SerialName("now_app") val nowApp: String = "",
    val limits: List<LimitUse> = emptyList(),
    /** Bloqueio valendo agora ("Internet pausada até 21:30"); vazio se nenhum. */
    val block: String = "",
    /** Acessos barrados hoje pelas regras (mostra que o bloqueio está pegando). */
    @SerialName("blocked_today") val blockedToday: Int = 0,
)

@Serializable
data class TimeWindow(val start: String = "22:00", val end: String = "06:00", val block: Boolean = false)

@Serializable
data class SchoolHours(
    val days: List<Int> = listOf(1, 2, 3, 4, 5),
    val start: String = "07:00",
    val end: String = "12:30",
    val block: Boolean = false,
)

/** Regras da pessoa: limites em minutos por categoria (ou "total"), hora de dormir, aula. */
@Serializable
data class PersonRules(
    val limits: Map<String, Int> = emptyMap(),
    val bedtime: TimeWindow? = null,
    val school: SchoolHours? = null,
    @SerialName("block_on_limit") val blockOnLimit: Boolean = false,
    @SerialName("block_categories") val blockCategories: List<String> = emptyList(),
)

@Serializable
data class LimitUse(
    val category: String,
    val label: String,
    @SerialName("used_sec") val usedSec: Long = 0,
    @SerialName("limit_sec") val limitSec: Long = 0,
)

/** Aviso de uso: app, categoria ou qualquer uso (os dois vazios), num aparelho ou de uma pessoa. */
@Serializable
data class UsageAlert(
    val id: Long,
    @SerialName("app_id") val appId: String = "",
    val category: String = "",
    @SerialName("device_id") val deviceId: Long? = null,
    @SerialName("person_id") val personId: Long? = null,
    @SerialName("created_at") val createdAt: Long = 0,
)

@Serializable
data class AlertEvent(
    val id: Long,
    val ts: Long,
    val type: String,
    @SerialName("person_id") val personId: Long? = null,
    @SerialName("device_id") val deviceId: Long? = null,
    val title: String,
    val message: String = "",
)
