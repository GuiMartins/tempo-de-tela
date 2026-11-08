@file:OptIn(ExperimentalMaterial3Api::class)

package com.tempodetela.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowForward
import androidx.compose.material.icons.outlined.Block
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.tempodetela.data.Person
import com.tempodetela.data.PersonToday
import java.time.LocalDate

@Composable
fun TodayScreen(
    state: UiState,
    onRefresh: () -> Unit,
    onOpenPerson: (Person) -> Unit,
    onClassify: () -> Unit,
    onGoToPersons: () -> Unit,
    onUnpair: () -> Unit,
    modifier: Modifier = Modifier,
    banner: (@Composable () -> Unit)? = null,
) {
    val todayBy = state.today.associateBy { it.personId }
    val now = nowSec()
    PullToRefreshBox(isRefreshing = state.loading, onRefresh = onRefresh, modifier = modifier.fillMaxSize()) {
        LazyColumn(
            Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            banner?.let { b -> item(key = "update") { b() } }
            state.error?.let { msg -> item { ErrorCard(msg, state.unauthorized, onRefresh, onUnpair) } }
            item {
                Text(
                    dayLabel(LocalDate.now()) + " · " + LocalDate.now().format(java.time.format.DateTimeFormatter.ofPattern("d/MM")),
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            if (state.loaded && state.persons.isEmpty()) {
                item {
                    AppCard(Modifier.fillMaxWidth(), onClick = onGoToPersons, padding = 20.dp) {
                        Text("Cadastre as pessoas da casa", style = MaterialTheme.typography.titleMedium)
                        Text(
                            "Depois vincule os aparelhos de cada uma na aba Pessoas. O tempo de tela aparece aqui.",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 4.dp),
                        )
                    }
                }
            }
            val ordered = state.persons
                .filter { it.kind != "shared" || (todayBy[it.id]?.totalSec ?: 0) > 0 }
                .sortedByDescending { todayBy[it.id]?.totalSec ?: 0 }
            items(ordered, key = { it.id }) { person ->
                PersonTodayCard(person, todayBy[person.id], state, now) { onOpenPerson(person) }
            }
            if (state.events.isNotEmpty()) {
                item(key = "alerts") { AlertsCard(state.events) }
            }
            if (state.unclassified.isNotEmpty()) {
                item {
                    AppCard(Modifier.fillMaxWidth(), onClick = onClassify) {
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                            CategoryBadgeIcon("other", 40)
                            Column(Modifier.weight(1f)) {
                                Text("${state.unclassified.size} sites sem categoria", style = MaterialTheme.typography.titleSmall)
                                Text(
                                    "Diga o que é cada um e o tempo deles passa a contar.",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                            Icon(Icons.AutoMirrored.Outlined.ArrowForward, null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }
            if (state.loaded) {
                item {
                    Text(
                        "Tempo estimado pelas consultas DNS dos aparelhos de cada pessoa. Não inclui o uso fora do Wi-Fi de casa.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(horizontal = 4.dp, vertical = 8.dp),
                    )
                }
            }
            item { Spacer(Modifier.height(24.dp)) }
        }
    }
}

@Composable
private fun PersonTodayCard(person: Person, today: PersonToday?, state: UiState, now: Long, onClick: () -> Unit) {
    val devices = state.devices.filter { it.personId == person.id }
    val online = devices.any { it.isOnline(now) }
    AppCard(Modifier.fillMaxWidth(), onClick = onClick) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
            PersonAvatar(person, 48)
            Column(Modifier.weight(1f)) {
                Text(person.name, style = MaterialTheme.typography.titleMedium)
                Text(
                    when {
                        devices.isEmpty() -> "Nenhum aparelho vinculado"
                        today == null || today.totalSec == 0L -> "Sem uso hoje"
                        else -> "Última atividade às ${clock(today.lastActivity)}"
                    },
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(formatDuration(today?.totalSec ?: 0), fontSize = 22.sp, fontWeight = FontWeight.Bold)
                when {
                    person.isPaused() -> StatusBadge("internet pausada", Tone.Danger)
                    !person.monitoring -> StatusBadge("sem registro", Tone.Warn)
                    !today?.nowApp.isNullOrBlank() -> StatusBadge("agora: ${today?.nowApp}", Tone.Ok, dot = true)
                    online -> StatusBadge("online", Tone.Ok, dot = true)
                }
            }
        }
        today?.block?.takeIf { it.isNotBlank() }?.let { block ->
            Row(
                Modifier.padding(top = 12.dp).fillMaxWidth().clip(androidx.compose.foundation.shape.RoundedCornerShape(10.dp))
                    .background(MaterialTheme.colorScheme.errorContainer).padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Icon(Icons.Outlined.Block, null, tint = MaterialTheme.colorScheme.error, modifier = Modifier.size(18.dp))
                Text(block, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onErrorContainer)
            }
        }
        val limits = today?.limits.orEmpty()
        if (limits.isNotEmpty()) {
            Column(Modifier.padding(top = 12.dp)) { limits.forEach { LimitBar(it) } }
        }
        val apps = today?.topApps.orEmpty()
        if (apps.isNotEmpty()) {
            HorizontalDivider(Modifier.padding(vertical = 12.dp), color = MaterialTheme.colorScheme.outlineVariant)
            val maxSec = apps.maxOf { it.sec }
            apps.forEach { AppUsageRow(it, maxSec) }
        }
    }
}

