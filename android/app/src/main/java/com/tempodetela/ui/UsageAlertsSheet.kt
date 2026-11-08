@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.tempodetela.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Place
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.tempodetela.data.AppCatalog
import com.tempodetela.data.Device
import com.tempodetela.data.Person
import com.tempodetela.data.UsageAlert
import java.text.Normalizer

/**
 * Avisos de uso: "me avise quando <app | categoria | qualquer uso> for usado em <aparelho | pessoa>".
 * [initialDeviceId] abre o formulário já com o aparelho escolhido (atalho da folha do aparelho).
 */
@Composable
fun UsageAlertsSheet(
    alerts: List<UsageAlert>,
    devices: List<Device>,
    persons: List<Person>,
    catalog: AppCatalog,
    telegramOn: Boolean,
    initialDeviceId: Long? = null,
    onDismiss: () -> Unit,
    onCreate: (appId: String?, category: String?, deviceId: Long?, personId: Long?) -> Unit,
    onDelete: (UsageAlert) -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var creating by rememberSaveable { mutableStateOf(initialDeviceId != null || alerts.isEmpty()) }
    var deviceId by rememberSaveable { mutableStateOf(initialDeviceId) }
    var personId by rememberSaveable { mutableStateOf<Long?>(null) }
    // "" = qualquer uso; "cat:<id>" = categoria; "app:<id>" = app.
    var what by rememberSaveable { mutableStateOf("") }
    var query by rememberSaveable { mutableStateOf("") }
    val shownDevices = devices.filter { !it.hidden || it.id == initialDeviceId }

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet) {
        Column(Modifier.fillMaxWidth()) {
            Column(
                Modifier.weight(1f, fill = false).verticalScroll(rememberScrollState()).padding(horizontal = 20.dp).padding(bottom = 12.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                Text("Avisos de uso", style = MaterialTheme.typography.titleLarge)
                Text(
                    "Uma notificação cada vez que alguém começa a usar o que você escolher. " +
                        if (telegramOn) "Chega em 1 a 3 minutos, pelo Telegram." else "Chega com até ~20 min de atraso.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )

                if (alerts.isNotEmpty()) {
                    AppCard(Modifier.fillMaxWidth(), padding = 0.dp) {
                        alerts.forEachIndexed { i, a ->
                            if (i > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                            Row(Modifier.fillMaxWidth().padding(start = 16.dp, top = 6.dp, bottom = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                                Column(Modifier.weight(1f)) {
                                    Text(whatLabel(a, catalog), style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.Medium)
                                    Text(whereLabel(a, devices, persons), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                                IconButton(onClick = { onDelete(a) }) { Icon(Icons.Outlined.Delete, "Apagar aviso") }
                            }
                        }
                    }
                }

                if (!creating) {
                    OutlinedButton(onClick = { creating = true }, modifier = Modifier.fillMaxWidth()) {
                        Icon(Icons.Outlined.Add, null, Modifier.size(18.dp))
                        Text("Novo aviso", Modifier.padding(start = 6.dp))
                    }
                } else {
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                    AlertSectionTitle(Icons.Outlined.Place, "Onde")
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        persons.forEach { p ->
                            val on = personId == p.id
                            FilterChip(
                                colors = brandChipColors(), border = brandChipBorder(on), selected = on,
                                onClick = { personId = p.id; deviceId = null },
                                label = { Text(listOf(p.emoji, p.name).filter { it.isNotBlank() }.joinToString(" ")) },
                            )
                        }
                        shownDevices.forEach { d ->
                            val on = deviceId == d.id
                            FilterChip(
                                colors = brandChipColors(), border = brandChipBorder(on), selected = on,
                                onClick = { deviceId = d.id; personId = null },
                                leadingIcon = { Icon(kindOf(d.effectiveKind)?.icon ?: Icons.Outlined.Apps, null, Modifier.size(18.dp)) },
                                label = { Text(d.displayName) },
                            )
                        }
                    }

                    AlertSectionTitle(Icons.Outlined.Visibility, "O quê")
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        FilterChip(
                            colors = brandChipColors(), border = brandChipBorder(what == ""), selected = what == "",
                            onClick = { what = "" }, label = { Text("Qualquer uso") },
                        )
                        catalog.categories.filter { !it.ignore }.forEach { c ->
                            val on = what == "cat:${c.id}"
                            FilterChip(
                                colors = brandChipColors(), border = brandChipBorder(on), selected = on,
                                onClick = { what = "cat:${c.id}" },
                                leadingIcon = { Icon(categoryIcon(c.id), null, tint = categoryColor(c.id)) },
                                label = { Text(c.label) },
                            )
                        }
                    }
                    OutlinedTextField(
                        value = query, onValueChange = { query = it.take(40) },
                        label = { Text("Ou um app") },
                        placeholder = { Text("Ex.: YouTube, Roblox, Navegador") },
                        leadingIcon = { Icon(Icons.Outlined.Search, null) },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    val term = fold(query.trim())
                    val selectedApp = what.removePrefix("app:").takeIf { what.startsWith("app:") }
                    val matches = catalog.apps.filter { a ->
                        a.id == selectedApp || (term.isNotEmpty() && (fold(a.name).contains(term) || a.id.contains(term)))
                    }.take(12)
                    if (matches.isNotEmpty()) {
                        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            matches.forEach { a ->
                                val on = a.id == selectedApp
                                FilterChip(
                                    colors = brandChipColors(), border = brandChipBorder(on), selected = on,
                                    onClick = { what = "app:${a.id}" },
                                    leadingIcon = { Icon(categoryIcon(a.category), null, tint = categoryColor(a.category)) },
                                    label = { Text(a.name) },
                                )
                            }
                        }
                    } else if (term.isNotEmpty()) {
                        Text(
                            "Nenhum app com esse nome no catálogo. Sites novos aparecem em Servidor › Sites sem categoria.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
            if (creating) {
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                Button(
                    onClick = {
                        onCreate(
                            what.removePrefix("app:").takeIf { what.startsWith("app:") },
                            what.removePrefix("cat:").takeIf { what.startsWith("cat:") },
                            deviceId, personId,
                        )
                        creating = false
                        what = ""
                        query = ""
                    },
                    enabled = deviceId != null || personId != null,
                    contentPadding = PaddingValues(vertical = 14.dp),
                    modifier = Modifier.fillMaxWidth().navigationBarsPadding().padding(horizontal = 20.dp, vertical = 12.dp),
                ) { Text(if (deviceId == null && personId == null) "Escolha onde" else "Criar aviso") }
            }
        }
    }
}

private fun whatLabel(a: UsageAlert, catalog: AppCatalog): String = when {
    a.appId.isNotBlank() -> catalog.apps.firstOrNull { it.id == a.appId }?.name ?: a.appId
    a.category.isNotBlank() -> catalog.categories.firstOrNull { it.id == a.category }?.label ?: a.category
    else -> "Qualquer uso"
}

private fun whereLabel(a: UsageAlert, devices: List<Device>, persons: List<Person>): String =
    a.deviceId?.let { id -> devices.firstOrNull { it.id == id }?.displayName ?: "Aparelho removido" }
        ?: a.personId?.let { id -> persons.firstOrNull { it.id == id }?.name ?: "Pessoa removida" }
        ?: "—"

/** Minúsculas e sem acento, pra busca "musica" achar "Música". */
private fun fold(s: String): String =
    Normalizer.normalize(s.lowercase(), Normalizer.Form.NFD).replace(Regex("\\p{M}"), "")

@Composable
private fun AlertSectionTitle(icon: ImageVector, text: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Icon(icon, null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(20.dp))
        Text(text, style = MaterialTheme.typography.titleMedium)
    }
}
