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
import com.tempodetela.data.Person
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
) {
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
                actions = { IconButton(onClick = onEdit) { Icon(Icons.Outlined.Edit, "Editar") } },
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
                DayContent(s, date)
            }
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
private fun TotalHeader(total: Long, subtitle: String) {
    AppCard(Modifier.fillMaxWidth(), padding = 20.dp) {
        Text("Tempo de tela", style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(formatDuration(total), fontSize = 36.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 2.dp))
        Text(subtitle, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
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
private fun DayContent(s: UsageSummary, date: LocalDate) {
    TotalHeader(
        s.totalSec,
        if (s.totalSec > 0) "Das ${clock(s.firstActivity)} às ${clock(s.lastActivity)}" else "Sem uso registrado",
    )
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
    TotalHeader(s.totalSec, "Média de ${formatDuration(s.totalSec / activeDays)} por dia com uso")
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
