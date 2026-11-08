package com.tempodetela.ui

import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.SystemUpdate
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.tempodetela.data.AppUpdate
import com.tempodetela.update.InstallState

/** Aviso de versão nova, com o botão que baixa e instala (ou pede a liberação do Android). */
@Composable
fun UpdateCard(update: AppUpdate, installed: String, state: InstallState, onUpdate: () -> Unit, modifier: Modifier = Modifier) {
    val context = LocalContext.current
    val c = MaterialTheme.colorScheme
    Surface(shape = CardShape, color = c.primaryContainer, modifier = modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Icon(Icons.Outlined.SystemUpdate, null, tint = c.primary)
                Text("Versão ${update.version} disponível", style = MaterialTheme.typography.titleMedium, color = c.onPrimaryContainer)
            }
            val text = when (state) {
                InstallState.Idle -> "Você está na $installed. O pareamento e os dados continuam como estão."
                InstallState.NeedsPermission -> "O Android pede, uma vez só, que você libere o Tempo de Tela para instalar as próprias atualizações."
                is InstallState.Downloading -> "Baixando…"
                InstallState.Installing -> "Instalando. O app fecha sozinho quando terminar; é só abrir de novo."
                is InstallState.Failed -> state.message
            }
            Text(text, color = c.onPrimaryContainer, style = MaterialTheme.typography.bodyMedium)
            if (state is InstallState.Downloading) {
                LinearProgressIndicator(progress = { state.fraction }, modifier = Modifier.fillMaxWidth().padding(top = 4.dp))
            }
            when (state) {
                InstallState.Idle, is InstallState.Failed ->
                    Button(onClick = onUpdate) { Text(if (state is InstallState.Failed) "Tentar de novo" else "Atualizar") }
                InstallState.NeedsPermission ->
                    Button(onClick = {
                        context.startActivity(
                            Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:${context.packageName}")),
                        )
                    }) { Text("Liberar") }
                else -> Unit
            }
        }
    }
}
