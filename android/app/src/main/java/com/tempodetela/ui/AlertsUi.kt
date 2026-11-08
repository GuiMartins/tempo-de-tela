package com.tempodetela.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Bedtime
import androidx.compose.material.icons.outlined.FiberNew
import androidx.compose.material.icons.outlined.HourglassBottom
import androidx.compose.material.icons.outlined.NotificationsActive
import androidx.compose.material.icons.outlined.School
import androidx.compose.material.icons.outlined.Wifi
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import com.tempodetela.data.AlertEvent
import com.tempodetela.data.LimitUse

fun alertIcon(type: String): ImageVector = when (type) {
    "limit" -> Icons.Outlined.HourglassBottom
    "bedtime" -> Icons.Outlined.Bedtime
    "school" -> Icons.Outlined.School
    "new_app" -> Icons.Outlined.FiberNew
    "new_device" -> Icons.Outlined.Wifi
    else -> Icons.Outlined.NotificationsActive
}

@Composable
fun alertColor(type: String): Color = when (type) {
    "limit", "bedtime", "school" -> LocalStatus.current.warn
    else -> MaterialTheme.colorScheme.primary
}

@Composable
fun AlertsCard(events: List<AlertEvent>, modifier: Modifier = Modifier, max: Int = 5) {
    if (events.isEmpty()) return
    AppCard(modifier.fillMaxWidth(), padding = 0.dp) {
        Text("Alertas recentes", style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(start = 16.dp, top = 14.dp, bottom = 4.dp))
        events.take(max).forEachIndexed { i, e ->
            if (i > 0) HorizontalDivider(Modifier.padding(start = 60.dp), color = MaterialTheme.colorScheme.outlineVariant)
            AlertRow(e)
        }
    }
}

@Composable
fun AlertRow(e: AlertEvent) {
    val color = alertColor(e.type)
    Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Box(Modifier.size(32.dp).clip(CircleShape).background(color.copy(alpha = .14f)), contentAlignment = Alignment.Center) {
            Icon(alertIcon(e.type), null, tint = color, modifier = Modifier.size(18.dp))
        }
        Column(Modifier.weight(1f)) {
            Text(e.title, style = MaterialTheme.typography.bodyMedium)
            val detail = listOf(e.message, relativeTime(e.ts)).filter { it.isNotBlank() }.joinToString(" · ")
            Text(detail, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

/** Barra de limite: verde até 80%, laranja até 100%, vermelha passou. */
@Composable
fun LimitBar(limit: LimitUse) {
    val s = LocalStatus.current
    val fraction = if (limit.limitSec > 0) limit.usedSec.toFloat() / limit.limitSec else 0f
    val color = when {
        fraction >= 1f -> MaterialTheme.colorScheme.error
        fraction >= .8f -> s.warn
        else -> s.ok
    }
    Column(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
        Row {
            Text(limit.label, style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
            Text(
                "${formatDuration(limit.usedSec)} de ${formatDuration(limit.limitSec)}",
                style = MaterialTheme.typography.bodySmall,
                color = if (fraction >= 1f) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Box(Modifier.padding(top = 4.dp).fillMaxWidth().height(6.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceContainerHigh)) {
            Box(Modifier.fillMaxWidth(fraction.coerceIn(0.02f, 1f)).fillMaxHeight().clip(CircleShape).background(color))
        }
    }
}
