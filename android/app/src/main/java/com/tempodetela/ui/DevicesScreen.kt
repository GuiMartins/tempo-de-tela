@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.tempodetela.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ExpandLess
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material.icons.outlined.VisibilityOff
import androidx.compose.material.icons.outlined.WarningAmber
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.tempodetela.data.Device
import com.tempodetela.data.Person

private enum class DeviceFilter { All, Online, NoDns, Unowned }

@Composable
fun DevicesScreen(
    state: UiState,
    onRefresh: () -> Unit,
    onOpen: (Device) -> Unit,
    onUnpair: () -> Unit,
    modifier: Modifier = Modifier,
    banner: (@Composable () -> Unit)? = null,
) {
    var showHidden by rememberSaveable { mutableStateOf(false) }
    var filter by rememberSaveable { mutableStateOf(DeviceFilter.All) }
    var query by rememberSaveable { mutableStateOf("") }
    val now = nowSec()
    val order = compareByDescending<Device> { it.isOnline(now) }.thenByDescending { it.lastSeen }
    val visible = state.devices.filter { !it.hidden }
    val owners = state.persons.associateBy { it.id }
    val term = query.trim().lowercase()
    val matches: (Device) -> Boolean = { d ->
        val byFilter = when (filter) {
            DeviceFilter.All -> true
            DeviceFilter.Online -> d.isOnline(now)
            DeviceFilter.NoDns -> d.lastDns == 0L
            DeviceFilter.Unowned -> d.personId == null
        }
        byFilter && (term.isEmpty() || listOf(d.displayName, d.detectedName, d.vendor, d.ipv4, d.mac, owners[d.personId]?.name ?: "")
            .any { it.lowercase().contains(term) })
    }
    val shown = visible.filter(matches)
    val hidden = state.devices.filter { it.hidden }.sortedWith(order)
    val unowned = shown.filter { it.personId == null }.sortedWith(order)
    val byPerson = state.persons.map { p -> p to shown.filter { it.personId == p.id }.sortedWith(order) }
        .filter { it.second.isNotEmpty() }
    val toggle = { f: DeviceFilter -> filter = if (filter == f) DeviceFilter.All else f }

    PullToRefreshBox(isRefreshing = state.loading, onRefresh = onRefresh, modifier = modifier.fillMaxSize()) {
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(horizontal = 16.dp, vertical = 4.dp)) {
            banner?.let { b -> item(key = "update") { b() } }
            state.error?.let { msg -> item { ErrorCard(msg, state.unauthorized, onRefresh, onUnpair) } }
            if (state.loaded) {
                item {
                    Row(Modifier.fillMaxWidth().padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        StatTile("Online", visible.count { it.isOnline(now) }.toString(), Modifier.weight(1f), Tone.Ok,
                            selected = filter == DeviceFilter.Online, onClick = { toggle(DeviceFilter.Online) })
                        StatTile("Fora do DNS", visible.count { it.lastDns == 0L }.toString(), Modifier.weight(1f), Tone.Warn,
                            selected = filter == DeviceFilter.NoDns, onClick = { toggle(DeviceFilter.NoDns) })
                        StatTile("Sem dono", visible.count { it.personId == null }.toString(), Modifier.weight(1f),
                            selected = filter == DeviceFilter.Unowned, onClick = { toggle(DeviceFilter.Unowned) })
                    }
                }
                item {
                    OutlinedTextField(
                        value = query, onValueChange = { query = it },
                        placeholder = { Text("Buscar aparelho, IP, fabricante…") },
                        leadingIcon = { Icon(Icons.Outlined.Search, null) },
                        trailingIcon = {
                            if (query.isNotEmpty()) IconButton(onClick = { query = "" }) { Icon(Icons.Outlined.Close, "Limpar") }
                        },
                        singleLine = true,
                        shape = RoundedCornerShape(14.dp),
                        colors = OutlinedTextFieldDefaults.colors(
                            unfocusedContainerColor = MaterialTheme.colorScheme.surfaceContainerLowest,
                            focusedContainerColor = MaterialTheme.colorScheme.surfaceContainerLowest,
                            unfocusedBorderColor = MaterialTheme.colorScheme.outlineVariant,
                        ),
                        modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
                    )
                }
            }
            if (unowned.isNotEmpty()) {
                item(key = "h-unowned") { SectionHeader("Sem dono", trailing = "${unowned.size}") }
                item(key = "c-unowned") { DeviceGroup(unowned, now, onOpen) }
            }
            byPerson.forEach { (person, devices) ->
                item(key = "h${person.id}") {
                    Row(
                        Modifier.fillMaxWidth().padding(start = 4.dp, top = 20.dp, bottom = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        PersonAvatar(person, 28)
                        Text(person.name, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                        if (!person.monitoring) StatusBadge("pausado", Tone.Warn)
                        Text("${devices.size}", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
                item(key = "c${person.id}") { DeviceGroup(devices, now, onOpen) }
            }
            if (state.loaded && shown.isEmpty() && visible.isNotEmpty()) {
                item {
                    Text(
                        "Nenhum aparelho com esse filtro.",
                        Modifier.fillMaxWidth().padding(32.dp),
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        textAlign = TextAlign.Center,
                    )
                }
            }
            if (hidden.isNotEmpty() && filter == DeviceFilter.All && term.isEmpty()) {
                item {
                    TextButton(onClick = { showHidden = !showHidden }, modifier = Modifier.padding(top = 8.dp)) {
                        Icon(if (showHidden) Icons.Outlined.ExpandLess else Icons.Outlined.ExpandMore, null)
                        Text("Ocultos (${hidden.size})", Modifier.padding(start = 4.dp))
                    }
                }
                if (showHidden) item(key = "c-hidden") { DeviceGroup(hidden, now, onOpen) }
            }
            if (state.loaded && state.devices.isEmpty()) {
                item {
                    AppCard(Modifier.fillMaxWidth().padding(top = 16.dp)) {
                        Text("Nenhum aparelho ainda", style = MaterialTheme.typography.titleMedium)
                        Text(
                            "Eles aparecem quando entram na rede ou usam o DNS do servidor.",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 4.dp),
                        )
                    }
                }
            }
            item { Spacer(Modifier.navigationBarsPadding().padding(bottom = 24.dp)) }
        }
    }
}

@Composable
private fun DeviceGroup(devices: List<Device>, now: Long, onOpen: (Device) -> Unit) {
    AppCard(Modifier.fillMaxWidth(), padding = 0.dp) {
        devices.forEachIndexed { i, d ->
            if (i > 0) HorizontalDivider(Modifier.padding(start = 72.dp), color = MaterialTheme.colorScheme.outlineVariant)
            DeviceRow(d, now) { onOpen(d) }
        }
    }
}

@Composable
private fun DeviceRow(device: Device, now: Long, onClick: () -> Unit) {
    val online = device.isOnline(now)
    val status = LocalStatus.current
    CardRow(onClick) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            DeviceAvatar(device.effectiveKind, online)
            Column(Modifier.weight(1f)) {
                Text(device.displayName, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
                val parts = buildList {
                    if (device.name.isNotBlank() && device.detectedName.isNotBlank()) add(device.detectedName)
                    else kindOf(device.effectiveKind)?.let { add(it.label) }
                    if (device.ipv4.isNotBlank()) add(device.ipv4)
                }
                if (parts.isNotEmpty()) {
                    Text(
                        parts.joinToString(" · "),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                }
                if (device.lastDns == 0L) {
                    Box(Modifier.padding(top = 4.dp)) { StatusBadge("fora do DNS", Tone.Warn, Icons.Outlined.WarningAmber) }
                }
            }
            Text(
                if (online) "online" else relativeTime(device.lastSeen, now),
                style = MaterialTheme.typography.labelMedium,
                color = if (online) status.ok else MaterialTheme.colorScheme.onSurfaceVariant,
                fontWeight = if (online) FontWeight.SemiBold else FontWeight.Normal,
            )
        }
    }
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
      Column {
        Column(
            Modifier
                .weight(1f, fill = false)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp)
                .padding(bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                DeviceAvatar(device.effectiveKind, device.isOnline(), 52.dp)
                Column(Modifier.weight(1f)) {
                    Text(device.displayName, style = MaterialTheme.typography.titleLarge, maxLines = 2, overflow = TextOverflow.Ellipsis)
                    Text(
                        if (device.isOnline()) "Online agora" else "Visto ${relativeTime(device.lastSeen)}",
                        style = MaterialTheme.typography.bodyMedium,
                        color = if (device.isOnline()) LocalStatus.current.ok else MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            OutlinedTextField(
                value = name, onValueChange = { if (it.length <= 64) name = it },
                label = { Text("Nome") },
                placeholder = { Text(device.detectedName.ifBlank { "Ex.: Tablet da Helena" }) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )

            Text("Dono", style = MaterialTheme.typography.titleSmall)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                FilterChip(colors = brandChipColors(), border = brandChipBorder(personId == null), selected = personId == null, onClick = { personId = null }, label = { Text("Sem dono") })
                persons.forEach { p ->
                    FilterChip(colors = brandChipColors(), border = brandChipBorder(personId == p.id), selected = personId == p.id,
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
                FilterChip(colors = brandChipColors(), border = brandChipBorder(kind.isBlank()), selected = kind.isBlank(),
                    onClick = { kind = "" },
                    label = { Text(if (guess != null) "Automático ($guess)" else "Automático") },
                )
                deviceKinds.forEach { k ->
                    FilterChip(colors = brandChipColors(), border = brandChipBorder(kind == k.id), selected = kind == k.id,
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

        }
        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
        Row(
            Modifier.fillMaxWidth().navigationBarsPadding().padding(horizontal = 20.dp, vertical = 12.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TextButton(onClick = { onHidden(!device.hidden); onDismiss() }) {
                Icon(if (device.hidden) Icons.Outlined.Visibility else Icons.Outlined.VisibilityOff, null, Modifier.size(18.dp))
                Text(if (device.hidden) "Mostrar" else "Ocultar", Modifier.padding(start = 6.dp))
            }
            Button(onClick = { onSave(name, kind, personId); onDismiss() }, contentPadding = PaddingValues(horizontal = 28.dp, vertical = 12.dp)) {
                Text("Salvar")
            }
        }
      }
    }
}
