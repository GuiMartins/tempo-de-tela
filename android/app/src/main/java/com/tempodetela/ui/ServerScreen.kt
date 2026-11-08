package com.tempodetela.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CloudDone
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.tempodetela.BuildConfig
import com.tempodetela.data.ServerConfig

@Composable
fun ServerScreen(state: UiState, config: ServerConfig?, onUnpair: () -> Unit, modifier: Modifier = Modifier) {
    var confirm by remember { mutableStateOf(false) }
    val status = LocalStatus.current
    val c = MaterialTheme.colorScheme
    val connected = state.status != null && state.error == null
    Column(
        modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        AppCard(Modifier.fillMaxWidth()) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                Box(
                    Modifier.size(48.dp).clip(CircleShape).background(if (connected) status.okContainer else c.errorContainer),
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(
                        if (connected) Icons.Outlined.CloudDone else Icons.Outlined.CloudOff, null,
                        tint = if (connected) status.ok else c.error,
                    )
                }
                Column(Modifier.weight(1f)) {
                    Text(if (connected) "Conectado ao servidor" else "Sem conexão", style = MaterialTheme.typography.titleMedium)
                    Text(
                        config?.orderedUrls()?.firstOrNull()?.removePrefix("http://") ?: "—",
                        style = MaterialTheme.typography.bodyMedium,
                        color = c.onSurfaceVariant,
                    )
                }
                state.status?.version?.takeIf { it.isNotBlank() }?.let { StatusBadge(it) }
            }
            config?.orderedUrls()?.drop(1)?.takeIf { it.isNotEmpty() }?.let { others ->
                HorizontalDivider(Modifier.padding(vertical = 12.dp), color = c.outlineVariant)
                Text("Endereço alternativo", style = MaterialTheme.typography.labelMedium, color = c.onSurfaceVariant)
                others.forEach { Text(it.removePrefix("http://"), style = MaterialTheme.typography.bodyMedium) }
                Text(
                    "Usado quando o principal não responde (fora de casa, pela Tailscale).",
                    style = MaterialTheme.typography.bodySmall,
                    color = c.onSurfaceVariant,
                    modifier = Modifier.padding(top = 4.dp),
                )
            }
        }

        state.status?.let { s ->
            SectionHeader("DNS", trailing = "desde que o servidor subiu")
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                StatTile("Consultas", "%,d".format(s.dns.queries).replace(',', '.'), Modifier.weight(1f))
                val rate = if (s.dns.queries > 0) s.dns.cacheHits * 100 / s.dns.queries else 0
                StatTile("Do cache", "$rate%", Modifier.weight(1f), Tone.Ok)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                StatTile("Bloqueadas", "%,d".format(s.dns.blocked).replace(',', '.'), Modifier.weight(1f))
                StatTile("Falhas", "%,d".format(s.dns.failures).replace(',', '.'), Modifier.weight(1f),
                    if (s.dns.failures > 0) Tone.Warn else Tone.Neutral)
            }
            if (s.dns.lastError.isNotBlank()) {
                Text("Último erro do upstream: ${s.dns.lastError}", style = MaterialTheme.typography.bodySmall, color = c.onSurfaceVariant)
            }

            SectionHeader("Descoberta na rede", trailing = s.lan.iface.ifBlank { null })
            AppCard(Modifier.fillMaxWidth(), padding = 0.dp) {
                listOf("Varredura ARP" to s.lan.arp, "DHCP passivo" to s.lan.dhcp, "mDNS" to s.lan.mdns).forEachIndexed { i, (name, value) ->
                    if (i > 0) HorizontalDivider(color = c.outlineVariant)
                    val color = when (value) {
                        "ativo" -> status.ok
                        "desativado", "iniciando", "" -> c.onSurfaceVariant
                        else -> c.error
                    }
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 14.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(name, Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge)
                        Box(Modifier.size(8.dp).clip(CircleShape).background(color))
                        Text(value.ifBlank { "—" }, color = color, style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(start = 8.dp))
                    }
                }
            }
        }

        OutlinedButton(
            onClick = { confirm = true },
            colors = ButtonDefaults.outlinedButtonColors(contentColor = c.error),
            modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
        ) { Text("Desparear este celular") }
        Text(
            "Tempo de Tela ${BuildConfig.VERSION_NAME}",
            style = MaterialTheme.typography.bodySmall,
            color = c.onSurfaceVariant,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth().padding(bottom = 24.dp),
        )
    }
    if (confirm) {
        AlertDialog(
            onDismissRequest = { confirm = false },
            title = { Text("Desparear?") },
            text = { Text("O app esquece o servidor. Para voltar, gere outro QR code no painel. O token antigo continua valendo até ser revogado lá.") },
            confirmButton = {
                TextButton(onClick = {
                    confirm = false
                    onUnpair()
                }) { Text("Desparear", color = c.error) }
            },
            dismissButton = { TextButton(onClick = { confirm = false }) { Text("Cancelar") } },
        )
    }
}

