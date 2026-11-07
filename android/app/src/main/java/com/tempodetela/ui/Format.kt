package com.tempodetela.ui

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Computer
import androidx.compose.material.icons.outlined.Devices
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material.icons.outlined.Router
import androidx.compose.material.icons.outlined.Sensors
import androidx.compose.material.icons.outlined.Speaker
import androidx.compose.material.icons.outlined.SportsEsports
import androidx.compose.material.icons.outlined.TabletAndroid
import androidx.compose.material.icons.outlined.Tv
import androidx.compose.ui.graphics.vector.ImageVector
import com.tempodetela.data.Device
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.format.DateTimeFormatter

data class Kind(val id: String, val label: String, val icon: ImageVector)

val deviceKinds = listOf(
    Kind("phone", "Celular", Icons.Outlined.PhoneAndroid),
    Kind("tablet", "Tablet", Icons.Outlined.TabletAndroid),
    Kind("computer", "Computador", Icons.Outlined.Computer),
    Kind("tv", "TV", Icons.Outlined.Tv),
    Kind("console", "Videogame", Icons.Outlined.SportsEsports),
    Kind("speaker", "Caixa de som", Icons.Outlined.Speaker),
    Kind("iot", "Casa inteligente", Icons.Outlined.Sensors),
    Kind("router", "Roteador", Icons.Outlined.Router),
    Kind("server", "Servidor", Icons.Outlined.Dns),
    Kind("other", "Outro", Icons.Outlined.Devices),
)

fun kindOf(id: String): Kind? = deviceKinds.firstOrNull { it.id == id }

fun kindIcon(id: String): ImageVector = kindOf(id)?.icon ?: Icons.Outlined.Devices

val personKinds = listOf("child" to "Criança", "adult" to "Adulto", "shared" to "Casa")

fun personKindLabel(id: String) = personKinds.firstOrNull { it.first == id }?.second ?: id

const val ONLINE_WINDOW_SEC = 5 * 60

fun nowSec() = System.currentTimeMillis() / 1000

fun Device.isOnline(now: Long = nowSec()) = lastSeen > 0 && now - lastSeen < ONLINE_WINDOW_SEC

private val zone: ZoneId get() = ZoneId.systemDefault()
private val hourFmt = DateTimeFormatter.ofPattern("HH:mm")
private val dateFmt = DateTimeFormatter.ofPattern("dd/MM HH:mm")
private val fullFmt = DateTimeFormatter.ofPattern("dd/MM/yyyy HH:mm")

/** "agora", "há 5 min", "há 3 h", "ontem 14:20", "02/11 09:10". */
fun relativeTime(epochSec: Long, now: Long = nowSec()): String {
    if (epochSec <= 0) return "nunca"
    val diff = now - epochSec
    if (diff < 60) return "agora"
    if (diff < 3600) return "há ${diff / 60} min"
    val time = Instant.ofEpochSecond(epochSec).atZone(zone)
    val today = LocalDate.now(zone)
    return when (time.toLocalDate()) {
        today -> if (diff < 6 * 3600) "há ${diff / 3600} h" else "hoje ${time.format(hourFmt)}"
        today.minusDays(1) -> "ontem ${time.format(hourFmt)}"
        else -> time.format(dateFmt)
    }
}

fun fullTime(epochSec: Long): String =
    if (epochSec <= 0) "—" else Instant.ofEpochSecond(epochSec).atZone(zone).format(fullFmt)
