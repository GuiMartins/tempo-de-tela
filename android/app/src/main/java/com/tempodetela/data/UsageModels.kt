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
)

@Serializable
data class PersonToday(
    @SerialName("person_id") val personId: Long,
    @SerialName("total_sec") val totalSec: Long = 0,
    @SerialName("last_activity") val lastActivity: Long = 0,
    @SerialName("top_apps") val topApps: List<AppUsage> = emptyList(),
)
