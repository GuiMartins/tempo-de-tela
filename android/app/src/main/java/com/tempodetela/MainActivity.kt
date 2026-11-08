package com.tempodetela

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tempodetela.alerts.AlertsWorker
import com.tempodetela.ui.AppViewModel
import com.tempodetela.ui.MainScreen
import com.tempodetela.ui.PairScreen
import com.tempodetela.ui.TempoTheme
import com.tempodetela.ui.UpdateCard
import com.tempodetela.update.InstallState

class MainActivity : ComponentActivity() {
    private val vm: AppViewModel by viewModels()
    private val askNotifications = registerForActivityResult(ActivityResultContracts.RequestPermission()) {}

    /** Android 13+: pede a permissão de notificação uma vez, depois do pareamento. */
    private fun maybeAskNotifications() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU || vm.config.value == null) return
        if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) return
        val prefs = getSharedPreferences("alertas", MODE_PRIVATE)
        if (prefs.getBoolean("permissao_pedida", false)) return
        prefs.edit().putBoolean("permissao_pedida", true).apply()
        askNotifications.launch(Manifest.permission.POST_NOTIFICATIONS)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // Só na build de debug: `adb shell am start -n com.tempodetela/.MainActivity --ez verificarAlertas true`.
        if (BuildConfig.DEBUG && intent.getBooleanExtra("verificarAlertas", false)) AlertsWorker.runNow(this)
        enableEdgeToEdge()
        setContent {
            TempoTheme {
                val config by vm.config.collectAsStateWithLifecycle()
                val update by vm.update.collectAsStateWithLifecycle()
                val installState by vm.installState.collectAsStateWithLifecycle()
                if (config == null) {
                    PairScreen(vm::pair, banner = update?.let { u ->
                        { UpdateCard(u, vm.updater.installedVersion, installState, vm::installUpdate) }
                    })
                } else {
                    MainScreen(vm)
                }
            }
        }
    }

    override fun onResume() {
        super.onResume()
        if (vm.config.value != null) vm.refresh()
        // Voltando das configurações com a liberação dada, o botão "Atualizar" volta.
        if (vm.installState.value == InstallState.NeedsPermission && packageManager.canRequestPackageInstalls()) {
            vm.updater.clearError()
        }
        vm.checkUpdate()
        maybeAskNotifications()
    }
}
