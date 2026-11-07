@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.tempodetela.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.tempodetela.data.Device
import com.tempodetela.data.Person

private val emojiSuggestions = listOf("👧", "👦", "🧒", "👩", "👨", "🧑", "👵", "👴", "🏠", "🦄", "⚽", "🎮", "⭐", "🐱")

@Composable
fun PersonsScreen(
    state: UiState,
    onRefresh: () -> Unit,
    onOpen: (Person) -> Unit,
    onUnpair: () -> Unit,
    modifier: Modifier = Modifier,
) {
    PullToRefreshBox(isRefreshing = state.loading, onRefresh = onRefresh, modifier = modifier.fillMaxSize()) {
        LazyColumn(Modifier.fillMaxSize()) {
            state.error?.let { msg -> item { ErrorCard(msg, state.unauthorized, onRefresh, onUnpair) } }
            if (state.loaded && state.persons.isEmpty()) {
                item {
                    Text(
                        "Cadastre quem mora na casa e vincule os aparelhos de cada um. Para aparelhos compartilhados, como a TV da sala, use o tipo Casa.",
                        Modifier.padding(24.dp),
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            items(state.persons, key = { it.id }) { person ->
                ListItem(
                    modifier = Modifier.clickable { onOpen(person) },
                    leadingContent = { PersonAvatar(person) },
                    headlineContent = { Text(person.name, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                    supportingContent = {
                        val n = person.deviceIds.size
                        Column {
                            Text("${personKindLabel(person.kind)} · $n ${if (n == 1) "aparelho" else "aparelhos"}")
                            if (!person.monitoring) {
                                Text("Monitoramento pausado", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
                            }
                        }
                    },
                )
            }
            item { Spacer(Modifier.height(96.dp)) }
        }
    }
}

/** person == null: cadastro de uma pessoa nova. */
@Composable
fun PersonSheet(
    person: Person?,
    devices: List<Device>,
    onDismiss: () -> Unit,
    onSave: (name: String, emoji: String, kind: String, monitoring: Boolean) -> Unit,
    onDelete: () -> Unit,
    onAssign: (deviceId: Long, personId: Long?) -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val key = person?.id ?: 0L
    var name by rememberSaveable(key) { mutableStateOf(person?.name ?: "") }
    var emoji by rememberSaveable(key) { mutableStateOf(person?.emoji ?: "") }
    var kind by rememberSaveable(key) { mutableStateOf(person?.kind ?: "child") }
    var monitoring by rememberSaveable(key) { mutableStateOf(person?.monitoring ?: true) }
    var confirmDelete by remember { mutableStateOf(false) }
    var addingDevice by remember { mutableStateOf(false) }

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet) {
        Column(
            Modifier
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp)
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(if (person == null) "Nova pessoa" else person.name, style = MaterialTheme.typography.titleLarge)
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                OutlinedTextField(
                    value = emoji, onValueChange = { if (it.length <= 8) emoji = it },
                    label = { Text("Emoji") },
                    singleLine = true,
                    textStyle = MaterialTheme.typography.titleLarge,
                    modifier = Modifier.size(width = 88.dp, height = 64.dp),
                )
                OutlinedTextField(
                    value = name, onValueChange = { if (it.length <= 64) name = it },
                    label = { Text("Nome") },
                    singleLine = true,
                    modifier = Modifier.weight(1f),
                )
            }
            FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                emojiSuggestions.forEach { e ->
                    TextButton(onClick = { emoji = e }, modifier = Modifier.size(44.dp)) { Text(e, fontSize = 20.sp) }
                }
            }
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                personKinds.forEachIndexed { i, (id, label) ->
                    SegmentedButton(
                        selected = kind == id,
                        onClick = {
                            kind = id
                            if (id == "shared" && emoji.isBlank()) emoji = "🏠"
                        },
                        shape = SegmentedButtonDefaults.itemShape(i, personKinds.size),
                    ) { Text(label) }
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Registrar o uso", style = MaterialTheme.typography.bodyLarge)
                    Text(
                        if (monitoring) "As consultas dos aparelhos desta pessoa entram nos relatórios."
                        else "Pausado: as consultas dos aparelhos desta pessoa não são gravadas.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                Switch(checked = monitoring, onCheckedChange = { monitoring = it })
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                Button(onClick = { onSave(name, emoji, kind, monitoring) }, enabled = name.isNotBlank()) {
                    Text(if (person == null) "Cadastrar" else "Salvar")
                }
            }

            if (person != null) {
                HorizontalDivider()
                Text("Aparelhos", style = MaterialTheme.typography.titleSmall)
                val mine = devices.filter { it.personId == person.id }
                if (mine.isEmpty()) {
                    Text("Nenhum aparelho vinculado.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                mine.forEach { d ->
                    ListItem(
                        leadingContent = { Icon(kindIcon(d.effectiveKind), null) },
                        headlineContent = { Text(d.displayName) },
                        supportingContent = { Text(relativeTime(d.lastSeen)) },
                        trailingContent = {
                            IconButton(onClick = { onAssign(d.id, null) }) { Icon(Icons.Outlined.Close, "Desvincular") }
                        },
                    )
                }
                OutlinedButton(onClick = { addingDevice = true }) {
                    Icon(Icons.Outlined.Add, null)
                    Text("Vincular aparelho", Modifier.padding(start = 8.dp))
                }
                HorizontalDivider()
                TextButton(onClick = { confirmDelete = true }) {
                    Text("Remover ${person.name}", color = MaterialTheme.colorScheme.error)
                }
            }
        }
    }

    if (person != null && addingDevice) {
        val free = devices.filter { !it.hidden && it.personId != person.id }
            .sortedWith(compareBy<Device> { it.personId != null }.thenByDescending { it.lastSeen })
        AlertDialog(
            onDismissRequest = { addingDevice = false },
            title = { Text("Vincular a ${person.name}") },
            text = {
                LazyColumn(Modifier.heightIn(max = 420.dp)) {
                    items(free, key = { it.id }) { d ->
                        ListItem(
                            modifier = Modifier.clickable {
                                onAssign(d.id, person.id)
                                addingDevice = false
                            },
                            leadingContent = { Icon(kindIcon(d.effectiveKind), null) },
                            headlineContent = { Text(d.displayName, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                            supportingContent = {
                                Text(if (d.personId != null) "Já tem dono · ${relativeTime(d.lastSeen)}" else relativeTime(d.lastSeen))
                            },
                        )
                    }
                }
            },
            confirmButton = { TextButton(onClick = { addingDevice = false }) { Text("Fechar") } },
        )
    }

    if (person != null && confirmDelete) {
        AlertDialog(
            onDismissRequest = { confirmDelete = false },
            title = { Text("Remover ${person.name}?") },
            text = { Text("Os aparelhos ficam sem dono. As consultas já gravadas continuam nos aparelhos.") },
            confirmButton = {
                TextButton(onClick = {
                    confirmDelete = false
                    onDelete()
                }) { Text("Remover", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text("Cancelar") } },
        )
    }
}
