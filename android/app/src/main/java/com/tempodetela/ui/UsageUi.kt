package com.tempodetela.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material.icons.outlined.AutoAwesome
import androidx.compose.material.icons.outlined.Chat
import androidx.compose.material.icons.outlined.Groups
import androidx.compose.material.icons.outlined.MusicNote
import androidx.compose.material.icons.outlined.OndemandVideo
import androidx.compose.material.icons.outlined.School
import androidx.compose.material.icons.outlined.ShoppingBag
import androidx.compose.material.icons.outlined.SportsEsports
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.tempodetela.data.AppUsage
import com.tempodetela.data.DayUsage
import com.tempodetela.data.SessionView
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale

private val categoryColors = mapOf(
    "video" to Color(0xFFE5484D),
    "social" to Color(0xFFD6409F),
    "games" to Color(0xFF8E4EC6),
    "messages" to Color(0xFF30A46C),
    "music" to Color(0xFFF76B15),
    "ai" to Color(0xFF12A594),
    "study" to Color(0xFF3B6FD8),
    "shopping" to Color(0xFFFFB224),
    "other" to Color(0xFF8B8D98),
)

fun categoryColor(id: String): Color = categoryColors[id] ?: Color(0xFF8B8D98)

fun categoryIcon(id: String): ImageVector = when (id) {
    "video" -> Icons.Outlined.OndemandVideo
    "social" -> Icons.Outlined.Groups
    "games" -> Icons.Outlined.SportsEsports
    "messages" -> Icons.Outlined.Chat
    "music" -> Icons.Outlined.MusicNote
    "ai" -> Icons.Outlined.AutoAwesome
    "study" -> Icons.Outlined.School
    "shopping" -> Icons.Outlined.ShoppingBag
    else -> Icons.Outlined.Apps
}

/** "2h 15min", "45min", "menos de 1min". */
fun formatDuration(sec: Long): String {
    val min = (sec + 30) / 60
    return when {
        sec <= 0 -> "0min"
        min < 1 -> "menos de 1min"
        min < 60 -> "${min}min"
        min % 60 == 0L -> "${min / 60}h"
        else -> "${min / 60}h ${min % 60}min"
    }
}

private val zone: ZoneId get() = ZoneId.systemDefault()
private val hm = DateTimeFormatter.ofPattern("HH:mm")
private val ptBR = Locale("pt", "BR")

fun clock(epochSec: Long): String = if (epochSec <= 0) "—" else Instant.ofEpochSecond(epochSec).atZone(zone).format(hm)

fun dayLabel(date: LocalDate, today: LocalDate = LocalDate.now()): String = when (date) {
    today -> "Hoje"
    today.minusDays(1) -> "Ontem"
    else -> date.format(DateTimeFormatter.ofPattern("EEE, d 'de' MMM", ptBR)).replaceFirstChar { it.uppercase() }
}

fun weekdayShort(date: LocalDate): String =
    date.format(DateTimeFormatter.ofPattern("EEE", ptBR)).take(3).replaceFirstChar { it.uppercase() }

@Composable
fun CategoryBadgeIcon(category: String, size: Int = 36) {
    val color = categoryColor(category)
    Box(
        Modifier.size(size.dp).clip(RoundedCornerShape((size * 0.3).dp)).background(color.copy(alpha = .14f)),
        contentAlignment = Alignment.Center,
    ) { Icon(categoryIcon(category), null, tint = color, modifier = Modifier.size((size * 0.55).dp)) }
}

/** Linha de app: ícone da categoria, nome, barra proporcional ao maior e duração. */
@Composable
fun AppUsageRow(app: AppUsage, maxSec: Long, modifier: Modifier = Modifier, sessions: Boolean = false) {
    Row(modifier.fillMaxWidth().padding(vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
        CategoryBadgeIcon(app.category)
        Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(app.name, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                Text(formatDuration(app.sec), style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.SemiBold)
            }
            Spacer(Modifier.height(6.dp))
            Box(Modifier.fillMaxWidth().height(6.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceContainerHigh)) {
                val fraction = if (maxSec > 0) (app.sec.toFloat() / maxSec).coerceIn(0.03f, 1f) else 0f
                Box(Modifier.fillMaxWidth(fraction).fillMaxHeight().clip(CircleShape).background(categoryColor(app.category)))
            }
            if (sessions && app.sessions > 0) {
                Text(
                    "${app.sessions} ${if (app.sessions == 1) "sessão" else "sessões"}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 4.dp),
                )
            }
        }
    }
}

/** Barras por dia da semana; a do dia selecionado fica destacada. Tocar numa barra seleciona o dia. */
@Composable
fun DayBars(daily: List<DayUsage>, selected: LocalDate?, onSelect: (LocalDate) -> Unit) {
    val maxSec = daily.maxOfOrNull { it.sec }?.coerceAtLeast(60) ?: 60
    Row(Modifier.fillMaxWidth().height(160.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        daily.forEach { d ->
            val date = LocalDate.parse(d.date)
            val isSel = date == selected
            val primary = MaterialTheme.colorScheme.primary
            Column(
                Modifier
                    .weight(1f)
                    .fillMaxHeight()
                    .clip(RoundedCornerShape(8.dp))
                    .clickable { onSelect(date) },
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(
                    if (d.sec > 0) shortDuration(d.sec) else "",
                    fontSize = 10.sp,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                )
                Box(Modifier.weight(1f).fillMaxWidth().padding(top = 4.dp), contentAlignment = Alignment.BottomCenter) {
                    Box(
                        Modifier
                            .fillMaxWidth()
                            .fillMaxHeight((d.sec.toFloat() / maxSec).coerceIn(0.02f, 1f))
                            .clip(RoundedCornerShape(topStart = 6.dp, topEnd = 6.dp, bottomStart = 2.dp, bottomEnd = 2.dp))
                            .background(if (isSel) primary else primary.copy(alpha = .35f)),
                    )
                }
                Text(
                    weekdayShort(date),
                    style = MaterialTheme.typography.labelSmall,
                    fontWeight = if (isSel) FontWeight.Bold else FontWeight.Normal,
                    color = if (isSel) primary else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 6.dp),
                )
            }
        }
    }
}

/** "2h15", "45m": cabe em cima da barra. */
fun shortDuration(sec: Long): String {
    val min = (sec + 30) / 60
    return if (min < 60) "${min}m" else "${min / 60}h" + if (min % 60 > 0) "%02d".format(min % 60) else ""
}

/** Linha do tempo do dia (0h–24h): uma faixa por app, com as sessões coloridas. */
@Composable
fun Timeline(sessions: List<SessionView>, dayStart: Long, maxApps: Int = 6) {
    val byApp = sessions.groupBy { it.appId }.entries
        .sortedByDescending { (_, list) -> list.sumOf { it.end - it.start } }
        .take(maxApps)
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        byApp.forEach { (_, list) ->
            val first = list.first()
            Column {
                Text(first.appName, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                BoxWithConstraints(
                    Modifier.fillMaxWidth().padding(top = 4.dp).height(14.dp).clip(RoundedCornerShape(4.dp))
                        .background(MaterialTheme.colorScheme.surfaceContainerHigh),
                ) {
                    val w = maxWidth
                    list.forEach { s ->
                        val startF = ((s.start - dayStart).toFloat() / 86400f).coerceIn(0f, 1f)
                        val endF = ((s.end - dayStart).toFloat() / 86400f).coerceIn(0f, 1f)
                        Box(
                            Modifier.offset(x = w * startF).width((w * (endF - startF)).coerceAtLeast(2.dp)).fillMaxHeight()
                                .background(categoryColor(s.category)),
                        )
                    }
                }
            }
        }
        Row(Modifier.fillMaxWidth()) {
            listOf("0h", "6h", "12h", "18h", "24h").forEachIndexed { i, label ->
                Text(
                    label,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.weight(1f),
                    textAlign = when (i) {
                        0 -> androidx.compose.ui.text.style.TextAlign.Start
                        4 -> androidx.compose.ui.text.style.TextAlign.End
                        else -> androidx.compose.ui.text.style.TextAlign.Center
                    },
                )
            }
        }
    }
}

/** Mapa de calor: dias da semana (seg–dom) × horas, cor pela quantidade de minutos. */
@Composable
fun Heatmap(grid: List<List<Int>>) {
    val maxMin = grid.flatten().maxOrNull()?.coerceAtLeast(1) ?: 1
    val labels = listOf("Seg", "Ter", "Qua", "Qui", "Sex", "Sáb", "Dom")
    val color = MaterialTheme.colorScheme.primary
    val empty = MaterialTheme.colorScheme.surfaceContainerHigh
    Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
        grid.forEachIndexed { day, hours ->
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(labels.getOrElse(day) { "" }, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.width(30.dp))
                Row(Modifier.weight(1f), horizontalArrangement = Arrangement.spacedBy(2.dp)) {
                    hours.forEach { m ->
                        Box(
                            Modifier.weight(1f).height(14.dp).clip(RoundedCornerShape(2.dp))
                                .background(if (m <= 0) empty else color.copy(alpha = (0.18f + 0.82f * m / maxMin).coerceAtMost(1f))),
                        )
                    }
                }
            }
        }
        Row(Modifier.padding(start = 30.dp)) {
            listOf("0h", "6h", "12h", "18h").forEach {
                Text(it, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.weight(1f))
            }
        }
    }
}

