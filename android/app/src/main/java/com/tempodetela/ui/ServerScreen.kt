package com.tempodetela.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.tempodetela.BuildConfig
import com.tempodetela.data.ServerConfig

@Composable
fun ServerScreen(state: UiState, config: ServerConfig?, onUnpair: () -> Unit, modifier: Modifier = Modifier) {
    var confirm by remember { mutableStateOf(false) }
    Column(
        modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Card(Modifier.fillMaxWidth()) {
            Column(Modifier.padding(16.dp)) {
                Text("Servidor", style = MaterialTheme.typography.titleMedium, modifier = Modifier.padding(bottom = 8.dp))
                config?.orderedUrls()?.forEachIndexed { i, url ->
                    InfoRow(if (i == 0) "Em uso" else "Alternativo", url)
                }
                state.status?.let { InfoRow("Versão", it.version) }
            }
        }
        state.status?.let { s ->
            Card(Modifier.fillMaxWidth()) {
                Column(Modifier.padding(16.dp)) {
                    Text("DNS", style = MaterialTheme.typography.titleMedium)
                    Text(
                        "Desde o último início do servidor",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(bottom = 8.dp),
                    )
                    InfoRow("Consultas", "%,d".format(s.dns.queries))
                    InfoRow("Do cache", "%,d".format(s.dns.cacheHits))
                    InfoRow("Bloqueadas", "%,d".format(s.dns.blocked))
                    InfoRow("Falhas no upstream", "%,d".format(s.dns.failures))
                    if (s.dns.lastError.isNotBlank()) InfoRow("Último erro", s.dns.lastError)
                }
            }
            Card(Modifier.fillMaxWidth()) {
                Column(Modifier.padding(16.dp)) {
                    Text("Descoberta na rede", style = MaterialTheme.typography.titleMedium, modifier = Modifier.padding(bottom = 8.dp))
                    InfoRow("Interface", s.lan.iface.ifBlank { "—" })
                    InfoRow("ARP", s.lan.arp)
                    InfoRow("DHCP", s.lan.dhcp)
                    InfoRow("mDNS", s.lan.mdns)
                }
            }
        }
        OutlinedButton(onClick = { confirm = true }, modifier = Modifier.fillMaxWidth()) { Text("Desparear este celular") }
        Text(
            "Tempo de Tela ${BuildConfig.VERSION_NAME}",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
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
                }) { Text("Desparear") }
            },
            dismissButton = { TextButton(onClick = { confirm = false }) { Text("Cancelar") } },
        )
    }
}
