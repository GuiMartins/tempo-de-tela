@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.tempodetela.ui

import android.app.TimePickerDialog
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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Bedtime
import androidx.compose.material.icons.outlined.HourglassBottom
import androidx.compose.material.icons.outlined.Remove
import androidx.compose.material.icons.outlined.School
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalIconButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.tempodetela.data.AppCatalog
import com.tempodetela.data.PersonRules
import com.tempodetela.data.SchoolHours
import com.tempodetela.data.TimeWindow

private const val STEP = 15

/** Regras da pessoa: limite por dia (total e por categoria), hora de dormir e horário de aula. */
@Composable
fun RulesSheet(
    personName: String,
    initial: PersonRules,
    catalog: AppCatalog,
    onDismiss: () -> Unit,
    onSave: (PersonRules) -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val limits = remember { mutableStateMapOf<String, Int>().apply { putAll(initial.limits) } }
    var bedtimeOn by remember { mutableStateOf(initial.bedtime != null) }
    var bedtime by remember { mutableStateOf(initial.bedtime ?: TimeWindow("22:00", "06:00")) }
    var schoolOn by remember { mutableStateOf(initial.school != null) }
    var school by remember { mutableStateOf(initial.school ?: SchoolHours()) }
    val categories = listOf("total" to "Total do dia") +
        catalog.categories.filter { !it.ignore }.map { it.id to it.label }

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet) {
        Column(Modifier.fillMaxWidth()) {
            Column(
                Modifier.weight(1f, fill = false).verticalScroll(rememberScrollState()).padding(horizontal = 20.dp).padding(bottom = 12.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                Text("Regras de $personName", style = MaterialTheme.typography.titleLarge)
                Text(
                    "Quando uma regra é quebrada, o app avisa com uma notificação. Ainda não bloqueia nada.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )

                SectionTitle(Icons.Outlined.HourglassBottom, "Limite por dia")
                categories.forEach { (id, label) ->
                    val minutes = limits[id] ?: 0
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        if (id == "total") CategoryBadgeIcon("other", 32) else CategoryBadgeIcon(id, 32)
                        Text(label, Modifier.weight(1f).padding(horizontal = 12.dp), style = MaterialTheme.typography.bodyLarge)
                        FilledTonalIconButton(onClick = { limits[id] = (minutes - STEP).coerceAtLeast(0) }, enabled = minutes > 0, modifier = Modifier.size(36.dp)) {
                            Icon(Icons.Outlined.Remove, "Menos")
                        }
                        Text(
                            if (minutes == 0) "sem limite" else formatDuration(minutes * 60L),
                            Modifier.width(86.dp),
                            textAlign = TextAlign.Center,
                            style = MaterialTheme.typography.labelLarge,
                            fontWeight = if (minutes > 0) FontWeight.SemiBold else FontWeight.Normal,
                            color = if (minutes > 0) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        FilledTonalIconButton(onClick = { limits[id] = (minutes + if (minutes == 0) 60 else STEP).coerceAtMost(24 * 60) }, modifier = Modifier.size(36.dp)) {
                            Icon(Icons.Outlined.Add, "Mais")
                        }
                    }
                }

                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) { SectionTitle(Icons.Outlined.Bedtime, "Hora de dormir") }
                    Switch(checked = bedtimeOn, onCheckedChange = { bedtimeOn = it })
                }
                if (bedtimeOn) {
                    Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        TimeButton("Das", bedtime.start, Modifier.weight(1f)) { bedtime = bedtime.copy(start = it) }
                        TimeButton("Até", bedtime.end, Modifier.weight(1f)) { bedtime = bedtime.copy(end = it) }
                    }
                }

                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) { SectionTitle(Icons.Outlined.School, "Horário de aula") }
                    Switch(checked = schoolOn, onCheckedChange = { schoolOn = it })
                }
                if (schoolOn) {
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        listOf("Seg", "Ter", "Qua", "Qui", "Sex", "Sáb", "Dom").forEachIndexed { i, label ->
                            val day = i + 1
                            val on = day in school.days
                            FilterChip(
                                colors = brandChipColors(), border = brandChipBorder(on),
                                selected = on,
                                onClick = { school = school.copy(days = if (on) school.days - day else (school.days + day).sorted()) },
                                label = { Text(label) },
                            )
                        }
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        TimeButton("Das", school.start, Modifier.weight(1f)) { school = school.copy(start = it) }
                        TimeButton("Até", school.end, Modifier.weight(1f)) { school = school.copy(end = it) }
                    }
                    Text(
                        "Apps de estudo (Classroom, Khan Academy, Duolingo…) não contam como uso na aula.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Button(
                onClick = {
                    onSave(
                        PersonRules(
                            limits = limits.filterValues { it > 0 },
                            bedtime = if (bedtimeOn) bedtime else null,
                            school = if (schoolOn && school.days.isNotEmpty()) school else null,
                        ),
                    )
                },
                contentPadding = PaddingValues(vertical = 14.dp),
                modifier = Modifier.fillMaxWidth().navigationBarsPadding().padding(horizontal = 20.dp, vertical = 12.dp),
            ) { Text("Salvar regras") }
        }
    }
}

@Composable
private fun SectionTitle(icon: ImageVector, text: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Icon(icon, null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(20.dp))
        Text(text, style = MaterialTheme.typography.titleMedium)
    }
}

@Composable
private fun TimeButton(label: String, value: String, modifier: Modifier = Modifier, onPick: (String) -> Unit) {
    val context = LocalContext.current
    val (h, m) = value.split(":").map { it.toIntOrNull() ?: 0 }.let { it.getOrElse(0) { 0 } to it.getOrElse(1) { 0 } }
    OutlinedButton(
        onClick = {
            TimePickerDialog(context, { _, hour, minute -> onPick("%02d:%02d".format(hour, minute)) }, h, m, true).show()
        },
        modifier = modifier,
    ) { Text("$label $value") }
}
