@file:OptIn(ExperimentalMaterial3Api::class)

package com.tempodetela.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.automirrored.outlined.KeyboardArrowLeft
import androidx.compose.material.icons.automirrored.outlined.KeyboardArrowRight
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.PauseCircle
import androidx.compose.material.icons.outlined.Wifi
import androidx.compose.material.icons.outlined.Tune
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.foundation.background
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.tempodetela.data.AlertEvent
import com.tempodetela.data.AppCatalog
import com.tempodetela.data.LimitUse
import com.tempodetela.data.Person
import com.tempodetela.data.PersonRules
import com.tempodetela.data.UsageSummary
import com.tempodetela.data.userMessage
import java.time.LocalDate
import java.time.ZoneId

/** Tela da pessoa: dia (apps, categorias, linha do tempo) e semana (barras e mapa de calor). */
@Composable
fun PersonDetailScreen(
    person: Person,
    load: suspend (LocalDate, Int) -> Result<UsageSummary>,
    onBack: () -> Unit,
    onEdit: () -> Unit,
    loadRules: suspend () -> Result<PersonRules> = { Result.success(PersonRules()) },
    catalog: AppCatalog = AppCatalog(),
    onSaveRules: (PersonRules, () -> Unit) -> Unit = { _, _ -> },
    events: List<AlertEvent> = emptyList(),
    blockStatus: String = "",
    onPause: (Int) -> Unit = {},
) {
    var choosingPause by remember { mutableStateOf(false) }
    var rules by remember { mutableStateOf<PersonRules?>(null) }
    var editingRules by remember { mutableStateOf(false) }
    var rulesVersion by remember { mutableIntStateOf(0) }
    LaunchedEffect(person.id, rulesVersion) { loadRules().onSuccess { rules = it } }
    BackHandler(onBack = onBack)
    var week by rememberSaveable { mutableStateOf(false) }
    var dateText by rememberSaveable { mutableStateOf(LocalDate.now().toString()) }
    val date = LocalDate.parse(dateText)
    var summary by remember { mutableStateOf<UsageSummary?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var reload by remember { mutableIntStateOf(0) }

    LaunchedEffect(person.id, dateText, week, reload) {
        error = null
        load(date, if (week) 7 else 1)
            .onSuccess { summary = it }
            .onFailure { error = it.userMessage() }
    }

    if (choosingPause) {
        PauseDialog(person.name, onDismiss = { choosingPause = false }) { minutes ->
            choosingPause = false
            onPause(minutes)
        }
    }
    if (editingRules) {
        RulesSheet(person.name, rules ?: PersonRules(), catalog, onDismiss = { editingRules = false }) { new ->
            onSaveRules(new) { rulesVersion++ }
            editingRules = false
        }
    }
    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Outlined.ArrowBack, "Voltar") } },
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PersonAvatar(person, 32)
                        Text(person.name)
                    }
                },
                actions = {
                    IconButton(onClick = { editingRules = true }) { Icon(Icons.Outlined.Tune, "Regras") }
                    IconButton(onClick = onEdit) { Icon(Icons.Outlined.Edit, "Editar") }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
            )
        },
    ) { padding ->
        Column(
            Modifier
                .padding(padding)
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                listOf(false to "Dia", true to "Semana").forEachIndexed { i, (isWeek, label) ->
                    SegmentedButton(
                        selected = week == isWeek,
                        onClick = { week = isWeek },
                        shape = SegmentedButtonDefaults.itemShape(i, 2),
                        colors = SegmentedButtonDefaults.colors(
                            activeContainerColor = MaterialTheme.colorScheme.primaryContainer,
                            activeContentColor = MaterialTheme.colorScheme.onPrimaryContainer,
                        ),
                    ) { Text(label) }
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                IconButton(onClick = { dateText = date.minusDays(if (week) 7 else 1).toString() }) {
                    Icon(Icons.AutoMirrored.Outlined.KeyboardArrowLeft, "Anterior")
                }
                Text(
                    if (week) "${date.minusDays(6).dayOfMonth}/${date.minusDays(6).monthValue} a ${date.dayOfMonth}/${date.monthValue}" else dayLabel(date),
                    style = MaterialTheme.typography.titleMedium,
                    modifier = Modifier.weight(1f),
                    textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                )
                IconButton(
                    onClick = { dateText = minOf(date.plusDays(if (week) 7 else 1), LocalDate.now()).toString() },
                    enabled = date < LocalDate.now(),
                ) { Icon(Icons.AutoMirrored.Outlined.KeyboardArrowRight, "Próximo") }
            }

            PauseCard(person, blockStatus, onPauseClick = { choosingPause = true }, onResume = { onPause(0) })
            error?.let { ErrorCard(it, false, { reload++ }, {}) }
            val s = summary
            if (s == null) {
                Box(Modifier.fillMaxWidth().padding(48.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            } else if (week) {
                WeekContent(s, date) { picked ->
                    dateText = picked.toString()
                    week = false
                }
            } else {
                DayContent(s, date, rules, catalog)
            }
            val mine = events.filter { it.personId == person.id }
            if (mine.isNotEmpty()) AlertsCard(mine)
            Text(
                "Estimado pelas consultas DNS. Uso fora do Wi-Fi de casa não aparece.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(vertical = 16.dp),
            )
        }
    }
}

@Composable
private fun TotalHeader(total: Long, subtitle: String, previous: Long = 0, previousLabel: String = "") {
    AppCard(Modifier.fillMaxWidth(), padding = 20.dp) {
        Text("Tempo de tela", style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Text(formatDuration(total), fontSize = 36.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 2.dp))
            comparison(total, previous)?.let { (text, more) ->
                Box(Modifier.padding(bottom = 8.dp)) { StatusBadge("$text $previousLabel", if (more) Tone.Warn else Tone.Ok) }
            }
        }
        Text(subtitle, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

/** "+25%"/"−10%" em relação ao período anterior; null sem base pra comparar. */
private fun comparison(total: Long, previous: Long): Pair<String, Boolean>? {
    if (previous < 10 * 60 || total == 0L) return null
    val pct = ((total - previous) * 100 / previous).toInt()
    if (pct == 0) return "igual" to false
    return (if (pct > 0) "+$pct%" else "−${-pct}%") to (pct > 0)
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun CategoryStack(s: UsageSummary) {
    if (s.categories.isEmpty()) return
    val total = s.categories.sumOf { it.sec }.coerceAtLeast(1)
    AppCard(Modifier.fillMaxWidth()) {
        Text("Por categoria", style = MaterialTheme.typography.titleSmall)
        Row(Modifier.fillMaxWidth().padding(top = 12.dp).height(12.dp).clip(CircleShape)) {
            s.categories.forEach { c ->
                Box(Modifier.weight(c.sec.toFloat() / total).height(12.dp).background(categoryColor(c.id)))
            }
        }
        FlowRow(Modifier.padding(top = 12.dp), horizontalArrangement = Arrangement.spacedBy(14.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            s.categories.forEach { c ->
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Box(Modifier.size(10.dp).clip(CircleShape).background(categoryColor(c.id)))
                    Text("${c.label} · ${formatDuration(c.sec)}", style = MaterialTheme.typography.bodySmall)
                }
            }
        }
    }
}

@Composable
private fun AppsCard(s: UsageSummary, title: String) {
    AppCard(Modifier.fillMaxWidth()) {
        Text(title, style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(bottom = 4.dp))
        if (s.apps.isEmpty()) {
            Text("Nenhum app reconhecido nesse período.", color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.bodyMedium)
        }
        val maxSec = s.apps.maxOfOrNull { it.sec } ?: 1
        s.apps.take(12).forEach { AppUsageRow(it, maxSec, sessions = true) }
    }
}

@Composable
private fun DayContent(s: UsageSummary, date: LocalDate, rules: PersonRules?, catalog: AppCatalog) {
    TotalHeader(
        s.totalSec,
        if (s.totalSec > 0) "Das ${clock(s.firstActivity)} às ${clock(s.lastActivity)}" else "Sem uso registrado",
        s.previousTotalSec, if (date == LocalDate.now()) "que ontem até esta hora" else "que no dia anterior",
    )
    val limits = rules?.limits.orEmpty()
    if (limits.isNotEmpty()) {
        AppCard(Modifier.fillMaxWidth()) {
            Text("Limites do dia", style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(bottom = 6.dp))
            limits["total"]?.let { LimitBar(LimitUse("total", "Total", s.totalSec, it * 60L)) }
            catalog.categories.filter { it.id in limits }.forEach { c ->
                LimitBar(LimitUse(c.id, c.label, s.categories.firstOrNull { it.id == c.id }?.sec ?: 0, limits.getValue(c.id) * 60L))
            }
        }
    }
    CategoryStack(s)
    if (s.sessions.isNotEmpty()) {
        AppCard(Modifier.fillMaxWidth()) {
            Text("Linha do tempo", style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(bottom = 12.dp))
            Timeline(s.sessions, date.atStartOfDay(ZoneId.systemDefault()).toEpochSecond())
        }
    }
    AppsCard(s, "Apps")
}

@Composable
private fun WeekContent(s: UsageSummary, end: LocalDate, onPickDay: (LocalDate) -> Unit) {
    val activeDays = s.daily.count { it.sec > 0 }.coerceAtLeast(1)
    TotalHeader(
        s.totalSec, "Média de ${formatDuration(s.totalSec / activeDays)} por dia com uso", s.previousTotalSec,
        if (end == LocalDate.now()) "que na semana anterior até agora" else "que na semana anterior",
    )
    AppCard(Modifier.fillMaxWidth()) {
        Text("Por dia", style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(bottom = 12.dp))
        DayBars(s.daily, end, onPickDay)
        Text("Toque num dia para ver o detalhe.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 8.dp))
    }
    CategoryStack(s)
    if (s.heatmap.isNotEmpty()) {
        AppCard(Modifier.fillMaxWidth()) {
            Text("Horários de uso", style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(bottom = 12.dp))
            Heatmap(s.heatmap)
        }
    }
    AppsCard(s, "Apps da semana")
    Spacer(Modifier.height(8.dp))
}

@Composable
private fun PauseCard(person: Person, blockStatus: String, onPauseClick: () -> Unit, onResume: () -> Unit) {
    val paused = person.isPaused()
    val c = MaterialTheme.colorScheme
    androidx.compose.material3.Surface(
        shape = CardShape,
        color = if (blockStatus.isNotBlank()) c.errorContainer else c.surfaceContainerLowest,
        border = androidx.compose.foundation.BorderStroke(1.dp, if (blockStatus.isNotBlank()) c.error.copy(alpha = .3f) else c.outlineVariant),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Row(Modifier.padding(16.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(
                if (paused) Icons.Outlined.PauseCircle else Icons.Outlined.Wifi, null,
                tint = if (blockStatus.isNotBlank()) c.error else c.primary,
            )
            Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
                Text(
                    blockStatus.ifBlank { "Internet liberada" },
                    style = MaterialTheme.typography.titleSmall,
                    color = if (blockStatus.isNotBlank()) c.onErrorContainer else c.onSurface,
                )
                if (blockStatus.isBlank()) {
                    Text("Pause na hora quando precisar.", style = MaterialTheme.typography.bodySmall, color = c.onSurfaceVariant)
                }
            }
            if (paused) {
                androidx.compose.material3.Button(onClick = onResume) { Text("Liberar") }
            } else {
                androidx.compose.material3.OutlinedButton(onClick = onPauseClick) { Text("Pausar") }
            }
        }
    }
}

@Composable
private fun PauseDialog(name: String, onDismiss: () -> Unit, onPick: (Int) -> Unit) {
    val now = java.time.LocalDateTime.now()
    var morning = now.toLocalDate().plusDays(1).atTime(7, 0)
    if (now.hour < 7) morning = now.toLocalDate().atTime(7, 0)
    val untilMorning = java.time.Duration.between(now, morning).toMinutes().toInt().coerceAtLeast(1)
    val options = listOf(
        30 to "30 minutos",
        60 to "1 hora",
        120 to "2 horas",
        untilMorning to "Até amanhã às 7h",
        -1 to "Sem prazo (até liberar)",
    )
    androidx.compose.material3.AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Pausar a internet de $name") },
        text = {
            Column {
                Text(
                    "Os aparelhos de $name ficam sem internet pelo Wi-Fi de casa (serviços do sistema continuam funcionando).",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(bottom = 8.dp),
                )
                options.forEach { (minutes, label) ->
                    androidx.compose.material3.TextButton(onClick = { onPick(minutes) }, modifier = Modifier.fillMaxWidth()) {
                        Text(label, modifier = Modifier.fillMaxWidth())
                    }
                }
            }
        },
        confirmButton = {},
        dismissButton = { androidx.compose.material3.TextButton(onClick = onDismiss) { Text("Cancelar") } },
    )
}
