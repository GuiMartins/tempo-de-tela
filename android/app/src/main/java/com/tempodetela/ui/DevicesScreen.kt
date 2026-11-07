@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.tempodetela.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ExpandLess
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.tempodetela.data.Device
import com.tempodetela.data.Person

private val OnlineGreen = Color(0xFF2E9D57)

@Composable
fun DevicesScreen(
    state: UiState,
    onRefresh: () -> Unit,
    onOpen: (Device) -> Unit,
    onUnpair: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var showHidden by rememberSaveable { mutableStateOf(false) }
    val now = nowSec()
    val order = compareByDescending<Device> { it.isOnline(now) }.thenByDescending { it.lastSeen }
    val visible = state.devices.filter { !it.hidden }
    val hidden = state.devices.filter { it.hidden }.sortedWith(order)
    val unowned = visible.filter { it.personId == null }.sortedWith(order)
    val byPerson = state.persons.map { p -> p to visible.filter { it.personId == p.id }.sortedWith(order) }
        .filter { it.second.isNotEmpty() }

    PullToRefreshBox(isRefreshing = state.loading, onRefresh = onRefresh, modifier = modifier.fillMaxSize()) {
        LazyColumn(Modifier.fillMaxSize()) {
            state.error?.let { msg ->
                item { ErrorCard(msg, state.unauthorized, onRefresh, onUnpair) }
            }
            if (state.loaded) {
                item {
                    val online = visible.count { it.isOnline(now) }
                    val noDns = visible.count { it.lastDns == 0L }
                    Text(
                        buildString {
                            append("${visible.size} aparelhos · $online online")
                            if (noDns > 0) append(" · $noDns sem usar o DNS da casa")
                        },
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                    )
                }
            }
            if (unowned.isNotEmpty()) {
                item { SectionHeader("Sem dono (${unowned.size})") }
                items(unowned, key = { it.id }) { DeviceRow(it, null, now) { onOpen(it) } }
            }
            byPerson.forEach { (person, devices) ->
                item(key = "p${person.id}") {
                    SectionHeader(listOf(person.emoji, person.name).filter { it.isNotBlank() }.joinToString(" "))
                }
                items(devices, key = { it.id }) { DeviceRow(it, person, now) { onOpen(it) } }
            }
            if (hidden.isNotEmpty()) {
                item {
                    TextButton(onClick = { showHidden = !showHidden }, modifier = Modifier.padding(8.dp)) {
                        Icon(if (showHidden) Icons.Outlined.ExpandLess else Icons.Outlined.ExpandMore, null)
                        Text("Ocultos (${hidden.size})", Modifier.padding(start = 4.dp))
                    }
                }
                if (showHidden) items(hidden, key = { it.id }) { DeviceRow(it, null, now) { onOpen(it) } }
            }
            if (state.loaded && state.devices.isEmpty()) {
                item {
                    Text(
                        "Nenhum aparelho ainda. Eles aparecem quando entram na rede ou usam o DNS do servidor.",
                        Modifier.padding(24.dp),
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            item { Spacer(Modifier.navigationBarsPadding().padding(bottom = 16.dp)) }
        }
    }
}

@Composable
private fun DeviceRow(device: Device, owner: Person?, now: Long, onClick: () -> Unit) {
    val online = device.isOnline(now)
    ListItem(
        modifier = Modifier.clickable(onClick = onClick),
        leadingContent = {
            Box(
                Modifier.size(40.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceContainerHighest),
                contentAlignment = Alignment.Center,
            ) { Icon(kindIcon(device.effectiveKind), null) }
        },
        headlineContent = { Text(device.displayName, maxLines = 1, overflow = TextOverflow.Ellipsis) },
        supportingContent = {
            val parts = buildList {
                if (device.name.isNotBlank() && device.detectedName.isNotBlank()) add(device.detectedName)
                kindOf(device.effectiveKind)?.let { if (device.name.isBlank() || device.detectedName.isBlank()) add(it.label) }
                if (device.ipv4.isNotBlank()) add(device.ipv4)
            }
            Column {
                if (parts.isNotEmpty()) Text(parts.joinToString(" · "), maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (device.lastDns == 0L) {
                    Text("Não usa o DNS da casa", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
                }
            }
        },
        trailingContent = {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                if (online) Box(Modifier.size(8.dp).clip(CircleShape).background(OnlineGreen))
                Text(
                    if (online) "online" else relativeTime(device.lastSeen, now),
                    style = MaterialTheme.typography.labelMedium,
                    color = if (online) OnlineGreen else MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        },
    )
}

@Composable
fun DeviceSheet(
    device: Device,
    persons: List<Person>,
    onDismiss: () -> Unit,
    onSave: (name: String, kind: String, personId: Long?) -> Unit,
    onHidden: (Boolean) -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var name by rememberSaveable(device.id) { mutableStateOf(device.name) }
    var kind by rememberSaveable(device.id) { mutableStateOf(device.kind) }
    var personId by rememberSaveable(device.id) { mutableStateOf(device.personId) }

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet) {
        Column(
            Modifier
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp)
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(device.displayName, style = MaterialTheme.typography.titleLarge)
            OutlinedTextField(
                value = name, onValueChange = { if (it.length <= 64) name = it },
                label = { Text("Nome") },
                placeholder = { Text(device.detectedName.ifBlank { "Ex.: Tablet da Helena" }) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )

            Text("Dono", style = MaterialTheme.typography.titleSmall)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                FilterChip(selected = personId == null, onClick = { personId = null }, label = { Text("Sem dono") })
                persons.forEach { p ->
                    FilterChip(
                        selected = personId == p.id,
                        onClick = { personId = p.id },
                        label = { Text(listOf(p.emoji, p.name).filter { it.isNotBlank() }.joinToString(" ")) },
                    )
                }
            }
            if (persons.isEmpty()) {
                Text(
                    "Cadastre as pessoas na aba Pessoas para vincular os aparelhos.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }

            Text("Tipo", style = MaterialTheme.typography.titleSmall)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                val guess = kindOf(device.kindGuess)?.label
                FilterChip(
                    selected = kind.isBlank(),
                    onClick = { kind = "" },
                    label = { Text(if (guess != null) "Automático ($guess)" else "Automático") },
                )
                deviceKinds.forEach { k ->
                    FilterChip(
                        selected = kind == k.id,
                        onClick = { kind = k.id },
                        leadingIcon = { Icon(k.icon, null, Modifier.size(18.dp)) },
                        label = { Text(k.label) },
                    )
                }
            }

            HorizontalDivider()
            Column {
                if (device.detectedName.isNotBlank()) InfoRow("Detectado como", device.detectedName)
                if (device.mac.isNotBlank()) InfoRow("MAC", device.mac + if (device.randomMac) " (aleatório)" else "")
                if (device.ipv4.isNotBlank()) InfoRow("IPv4", device.ipv4)
                if (device.ipv6.isNotBlank()) InfoRow("IPv6", device.ipv6)
                if (device.vendor.isNotBlank()) InfoRow("Fabricante", device.vendor)
                if (device.osGuess.isNotBlank()) InfoRow("Sistema", device.osGuess)
                InfoRow("Apareceu em", fullTime(device.firstSeen))
                InfoRow("Visto na rede", relativeTime(device.lastSeen))
                InfoRow("Última consulta DNS", if (device.lastDns == 0L) "nunca: não usa o DNS da casa" else relativeTime(device.lastDns))
            }
            if (device.randomMac) {
                Text(
                    "Com MAC aleatório, o aparelho pode aparecer como novo de tempos em tempos. Na rede de casa, use o MAC do aparelho nas configurações de Wi-Fi.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }

            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                TextButton(onClick = { onHidden(!device.hidden); onDismiss() }) {
                    Text(if (device.hidden) "Mostrar" else "Ocultar")
                }
                Button(onClick = { onSave(name, kind, personId); onDismiss() }) { Text("Salvar") }
            }
        }
    }
}
