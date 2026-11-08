package com.tempodetela

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tempodetela.ui.AppViewModel
import com.tempodetela.ui.MainScreen
import com.tempodetela.ui.PairScreen
import com.tempodetela.ui.TempoTheme
import com.tempodetela.ui.UpdateCard
import com.tempodetela.update.InstallState

class MainActivity : ComponentActivity() {
    private val vm: AppViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
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
    }
}
