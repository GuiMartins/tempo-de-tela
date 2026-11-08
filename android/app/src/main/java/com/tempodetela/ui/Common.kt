package com.tempodetela.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.requiredSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.WarningAmber
import androidx.compose.material3.Button
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.tempodetela.R
import com.tempodetela.data.Person

val CardShape = RoundedCornerShape(18.dp)

/** Cartão no estilo do painel: fundo claro, borda fina, cantos arredondados. */
@Composable
fun AppCard(
    modifier: Modifier = Modifier,
    onClick: (() -> Unit)? = null,
    padding: Dp = 16.dp,
    content: @Composable ColumnScope.() -> Unit,
) {
    val colors = MaterialTheme.colorScheme
    Surface(
        modifier = modifier,
        shape = CardShape,
        color = colors.surfaceContainerLowest,
        border = BorderStroke(1.dp, colors.outlineVariant),
        onClick = onClick ?: {},
        enabled = onClick != null,
    ) {
        Column(Modifier.padding(padding), content = content)
    }
}

/** Logo do app: o mesmo ícone do launcher, em quadrado azul arredondado. */
@Composable
fun BrandLogo(size: Dp = 40.dp) {
    Box(
        Modifier.size(size).clip(RoundedCornerShape(size * 0.28f)).background(Color(0xFF3B6FD8)),
        contentAlignment = Alignment.Center,
    ) {
        // O desenho do launcher ocupa os 66dp centrais de 108dp.
        Image(painterResource(R.drawable.ic_launcher_foreground), null, Modifier.requiredSize(size * (108f / 66f)))
    }
}

@Composable
fun SectionHeader(text: String, modifier: Modifier = Modifier, trailing: String? = null) {
    Row(
        modifier.fillMaxWidth().padding(start = 4.dp, end = 4.dp, top = 20.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(text, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
        trailing?.let { Text(it, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
    }
}

enum class Tone { Neutral, Ok, Warn, Danger }

@Composable
fun StatusBadge(text: String, tone: Tone = Tone.Neutral, icon: ImageVector? = null, dot: Boolean = false) {
    val s = LocalStatus.current
    val c = MaterialTheme.colorScheme
    val (bg, fg) = when (tone) {
        Tone.Ok -> s.okContainer to s.ok
        Tone.Warn -> s.warnContainer to s.warn
        Tone.Danger -> c.errorContainer to c.error
        Tone.Neutral -> c.surfaceContainerHigh to c.onSurfaceVariant
    }
    Row(
        Modifier.clip(CircleShape).background(bg).padding(horizontal = 8.dp, vertical = 3.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        if (dot) Box(Modifier.size(6.dp).clip(CircleShape).background(fg))
        icon?.let { Icon(it, null, Modifier.size(13.dp), tint = fg) }
        Text(text, color = fg, fontSize = 12.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
    }
}

/** Ícone do aparelho num quadrado arredondado, com bolinha verde quando está online. */
@Composable
fun DeviceAvatar(kind: String, online: Boolean, size: Dp = 44.dp) {
    val c = MaterialTheme.colorScheme
    Box(Modifier.size(size + 4.dp)) {
        Box(
            Modifier
                .size(size)
                .clip(RoundedCornerShape(12.dp))
                .background(if (online) c.primaryContainer else c.surfaceContainerHigh),
            contentAlignment = Alignment.Center,
        ) {
            Icon(kindIcon(kind), null, tint = if (online) c.primary else c.onSurfaceVariant, modifier = Modifier.size(size * 0.5f))
        }
        if (online) {
            Box(
                Modifier
                    .align(Alignment.BottomEnd)
                    .size(14.dp)
                    .clip(CircleShape)
                    .background(LocalStatus.current.ok)
                    .border(2.5.dp, c.surfaceContainerLowest, CircleShape),
            )
        }
    }
}

@Composable
fun PersonAvatar(person: Person, size: Int = 44) {
    val c = MaterialTheme.colorScheme
    Box(
        Modifier.size(size.dp).clip(CircleShape).background(c.primaryContainer),
        contentAlignment = Alignment.Center,
    ) {
        if (person.emoji.isNotBlank()) {
            Text(person.emoji, fontSize = (size * 0.48).sp)
        } else {
            Text(person.name.take(1).uppercase(), fontWeight = FontWeight.Bold, color = c.onPrimaryContainer, fontSize = (size * 0.42).sp)
        }
    }
}

/** Número grande com rótulo; tocável quando serve de filtro. */
@Composable
fun StatTile(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
    tone: Tone = Tone.Neutral,
    selected: Boolean = false,
    onClick: (() -> Unit)? = null,
) {
    val s = LocalStatus.current
    val c = MaterialTheme.colorScheme
    val accent = when (tone) {
        Tone.Ok -> s.ok
        Tone.Warn -> s.warn
        Tone.Danger -> c.error
        Tone.Neutral -> c.onSurface
    }
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(16.dp),
        color = if (selected) c.primaryContainer else c.surfaceContainerLowest,
        border = BorderStroke(1.dp, if (selected) c.primary.copy(alpha = .5f) else c.outlineVariant),
        onClick = onClick ?: {},
        enabled = onClick != null,
    ) {
        Column(Modifier.padding(horizontal = 14.dp, vertical = 12.dp)) {
            Text(label, style = MaterialTheme.typography.labelMedium, color = c.onSurfaceVariant, maxLines = 1)
            Text(value, fontSize = 26.sp, fontWeight = FontWeight.Bold, color = accent, modifier = Modifier.padding(top = 2.dp))
        }
    }
}

@Composable
fun ErrorCard(message: String, unauthorized: Boolean, onRetry: () -> Unit, onUnpair: () -> Unit) {
    val c = MaterialTheme.colorScheme
    Surface(shape = CardShape, color = c.errorContainer, modifier = Modifier.fillMaxWidth().padding(top = 12.dp)) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Icon(Icons.Outlined.WarningAmber, null, tint = c.onErrorContainer)
                Text(message, color = c.onErrorContainer)
            }
            if (unauthorized) {
                Button(onClick = onUnpair) { Text("Parear de novo") }
            } else {
                OutlinedButton(onClick = onRetry) { Text("Tentar de novo") }
            }
        }
    }
}

@Composable
fun InfoRow(label: String, value: String, modifier: Modifier = Modifier) {
    Row(modifier.fillMaxWidth().padding(vertical = 7.dp), verticalAlignment = Alignment.Top) {
        Text(label, Modifier.weight(0.42f), color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.bodyMedium)
        Text(value, Modifier.weight(0.58f), style = MaterialTheme.typography.bodyMedium)
    }
}

/** Linha clicável dentro de um cartão (sem o fundo próprio do ListItem). */
@Composable
fun CardRow(onClick: (() -> Unit)?, content: @Composable () -> Unit) {
    Box(
        Modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .padding(horizontal = 14.dp, vertical = 12.dp),
    ) { content() }
}

@Composable
fun brandChipColors() = FilterChipDefaults.filterChipColors(
    selectedContainerColor = MaterialTheme.colorScheme.primaryContainer,
    selectedLabelColor = MaterialTheme.colorScheme.onPrimaryContainer,
    selectedLeadingIconColor = MaterialTheme.colorScheme.primary,
)

@Composable
fun brandChipBorder(selected: Boolean) = FilterChipDefaults.filterChipBorder(
    enabled = true,
    selected = selected,
    borderColor = MaterialTheme.colorScheme.outline,
    selectedBorderColor = MaterialTheme.colorScheme.primary.copy(alpha = .6f),
    selectedBorderWidth = 1.dp,
)
