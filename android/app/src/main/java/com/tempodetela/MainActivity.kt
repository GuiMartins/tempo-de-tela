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

class MainActivity : ComponentActivity() {
    private val vm: AppViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            TempoTheme {
                val config by vm.config.collectAsStateWithLifecycle()
                if (config == null) PairScreen(vm::pair) else MainScreen(vm)
            }
        }
    }

    override fun onResume() {
        super.onResume()
        if (vm.config.value != null) vm.refresh()
    }
}
