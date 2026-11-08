@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.tempodetela.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Search
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
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
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
import com.tempodetela.data.AppCatalog
import com.tempodetela.data.Unclassified

/** Sites fora do catálogo: dizer o que cada um é faz o tempo dele passar a contar. */
@Composable
fun ClassifyScreen(
    items: List<Unclassified>,
    catalog: AppCatalog,
    onBack: () -> Unit,
    onClassify: (site: String, appId: String?, newName: String?, category: String?) -> Unit,
) {
    BackHandler(onBack = onBack)
    var open by remember { mutableStateOf<Unclassified?>(null) }
    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Outlined.ArrowBack, "Voltar") } },
                title = { Text("Sites sem categoria") },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
            )
        },
    ) { padding ->
        LazyColumn(
            Modifier.padding(padding).fillMaxSize(),
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            item {
                Text(
                    "Os mais acessados nos últimos 7 dias. Classifique os que são apps ou sites que alguém usa; serviço de fundo vai em “Ignorar”.",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
            if (items.isEmpty()) {
                item {
                    AppCard(Modifier.fillMaxWidth()) { Text("Tudo classificado.", style = MaterialTheme.typography.titleSmall) }
                }
            }
            items(items, key = { it.site }) { u ->
                AppCard(Modifier.fillMaxWidth(), onClick = { open = u }) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text(u.site, style = MaterialTheme.typography.titleSmall)
                            if (u.examples.isNotEmpty()) {
                                Text(
                                    u.examples.joinToString(", "),
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                            }
                        }
                        Column(horizontalAlignment = Alignment.End) {
                            Text("${u.count}×", style = MaterialTheme.typography.labelLarge)
                            Text(
                                "${u.devices} ${if (u.devices == 1) "aparelho" else "aparelhos"}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                }
            }
        }
    }
    open?.let { u ->
        ClassifySheet(u, catalog, onDismiss = { open = null }) { appId, name, category ->
            onClassify(u.site, appId, name, category)
            open = null
        }
    }
}

@Composable
private fun ClassifySheet(
    item: Unclassified,
    catalog: AppCatalog,
    onDismiss: () -> Unit,
    onDone: (appId: String?, newName: String?, category: String?) -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var newApp by rememberSaveable { mutableStateOf(false) }
    var query by rememberSaveable { mutableStateOf("") }
    var name by rememberSaveable { mutableStateOf(suggestName(item.site)) }
    val categories = catalog.categories.filter { !it.ignore }
    var category by rememberSaveable { mutableStateOf(categories.firstOrNull()?.id ?: "other") }
    val labels = catalog.categories.associate { it.id to it.label }

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet) {
        Column(
            Modifier.verticalScroll(rememberScrollState()).navigationBarsPadding().padding(horizontal = 20.dp).padding(bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(item.site, style = MaterialTheme.typography.titleLarge)
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                listOf(false to "App do catálogo", true to "App novo").forEachIndexed { i, (isNew, label) ->
                    SegmentedButton(
                        selected = newApp == isNew,
                        onClick = { newApp = isNew },
                        shape = SegmentedButtonDefaults.itemShape(i, 2),
                        colors = SegmentedButtonDefaults.colors(
                            activeContainerColor = MaterialTheme.colorScheme.primaryContainer,
                            activeContentColor = MaterialTheme.colorScheme.onPrimaryContainer,
                        ),
                    ) { Text(label) }
                }
            }
            if (!newApp) {
                OutlinedTextField(
                    value = query, onValueChange = { query = it },
                    placeholder = { Text("Buscar app") },
                    leadingIcon = { Icon(Icons.Outlined.Search, null) },
                    singleLine = true,
                    shape = RoundedCornerShape(14.dp),
                    modifier = Modifier.fillMaxWidth(),
                )
                val apps = catalog.apps
                    .filter { catalog.categories.firstOrNull { c -> c.id == it.category }?.ignore != true }
                    .filter { query.isBlank() || it.name.contains(query.trim(), ignoreCase = true) }
                    .sortedBy { it.name.lowercase() }
                LazyColumn(Modifier.heightIn(max = 340.dp)) {
                    items(apps, key = { it.id }) { app ->
                        Row(
                            Modifier.fillMaxWidth().clickable { onDone(app.id, null, null) }.padding(vertical = 8.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                        ) {
                            CategoryBadgeIcon(app.category, 32)
                            Column {
                                Text(app.name, style = MaterialTheme.typography.bodyLarge)
                                Text(labels[app.category] ?: app.category, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                        }
                    }
                }
            } else {
                OutlinedTextField(
                    value = name, onValueChange = { if (it.length <= 64) name = it },
                    label = { Text("Nome do app ou site") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Text("Categoria", style = MaterialTheme.typography.titleSmall)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    categories.forEach { c ->
                        FilterChip(
                            colors = brandChipColors(), border = brandChipBorder(category == c.id),
                            selected = category == c.id,
                            onClick = { category = c.id },
                            leadingIcon = { Icon(categoryIcon(c.id), null, tint = categoryColor(c.id)) },
                            label = { Text(c.label) },
                        )
                    }
                }
                Button(onClick = { onDone(null, name.trim(), category) }, enabled = name.isNotBlank(), modifier = Modifier.fillMaxWidth()) {
                    Text("Criar e classificar")
                }
            }
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            OutlinedButton(onClick = { onDone("system-ignored", null, null) }, modifier = Modifier.fillMaxWidth()) {
                Text("Ignorar: é serviço de fundo, não tempo de tela")
            }
        }
    }
}

/** "escola.exemplo.com.br" → "Exemplo"; "tiktokv.com" → "Tiktokv". */
private fun suggestName(site: String): String {
    val first = site.substringBefore('.')
    return first.replaceFirstChar { it.uppercase() }
}
