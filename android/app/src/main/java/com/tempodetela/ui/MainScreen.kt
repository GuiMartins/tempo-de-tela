@file:OptIn(ExperimentalMaterial3Api::class)

package com.tempodetela.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material.icons.outlined.Devices
import androidx.compose.material.icons.outlined.PersonAdd
import androidx.compose.material.icons.outlined.People
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
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
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle

private enum class Tab(val label: String, val icon: ImageVector) {
    Devices("Aparelhos", Icons.Outlined.Devices),
    Persons("Pessoas", Icons.Outlined.People),
    Server("Servidor", Icons.Outlined.Dns),
}

@Composable
fun MainScreen(vm: AppViewModel) {
    val state by vm.state.collectAsStateWithLifecycle()
    val config by vm.config.collectAsStateWithLifecycle()
    var tab by rememberSaveable { mutableIntStateOf(0) }
    var openDevice by rememberSaveable { mutableStateOf<Long?>(null) }
    // -1 = cadastro de pessoa nova; null = nenhuma folha aberta.
    var openPerson by rememberSaveable { mutableStateOf<Long?>(null) }
    val snackbar = remember { SnackbarHostState() }

    LaunchedEffect(Unit) { vm.messages.collect { snackbar.showSnackbar(it) } }

    val current = Tab.entries[tab]
    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        if (current == Tab.Devices) BrandLogo(32.dp)
                        Text(if (current == Tab.Devices) "Tempo de Tela" else current.label)
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
            )
        },
        snackbarHost = { SnackbarHost(snackbar) },
        floatingActionButton = {
            if (current == Tab.Persons && state.persons.isNotEmpty()) {
                ExtendedFloatingActionButton(
                    onClick = { openPerson = -1 },
                    icon = { Icon(Icons.Outlined.PersonAdd, null) },
                    text = { Text("Nova pessoa") },
                )
            }
        },
        bottomBar = {
            NavigationBar(containerColor = MaterialTheme.colorScheme.surfaceContainerLowest, tonalElevation = 0.dp) {
                Tab.entries.forEachIndexed { i, t ->
                    NavigationBarItem(
                        selected = tab == i,
                        onClick = { tab = i },
                        icon = { Icon(t.icon, null) },
                        label = { Text(t.label) },
                        colors = NavigationBarItemDefaults.colors(
                            indicatorColor = MaterialTheme.colorScheme.primaryContainer,
                            selectedIconColor = MaterialTheme.colorScheme.primary,
                            selectedTextColor = MaterialTheme.colorScheme.primary,
                        ),
                    )
                }
            }
        },
    ) { padding ->
        val modifier = Modifier.padding(padding)
        when (current) {
            Tab.Devices -> DevicesScreen(state, vm::refresh, { openDevice = it.id }, vm::unpair, modifier)
            Tab.Persons -> PersonsScreen(state, vm::refresh, { openPerson = it.id }, { openPerson = -1 }, vm::unpair, modifier)
            Tab.Server -> ServerScreen(state, config, vm::unpair, modifier)
        }
    }

    openDevice?.let { id ->
        val device = state.devices.firstOrNull { it.id == id }
        if (device == null) {
            openDevice = null
        } else {
            DeviceSheet(
                device = device,
                persons = state.persons,
                onDismiss = { openDevice = null },
                onSave = { name, kind, personId -> vm.saveDevice(device, name, kind, personId) },
                onHidden = { vm.setHidden(device, it) },
            )
        }
    }

    openPerson?.let { id ->
        val person = if (id == -1L) null else state.persons.firstOrNull { it.id == id }
        if (id != -1L && person == null) {
            openPerson = null
        } else {
            PersonSheet(
                person = person,
                devices = state.devices,
                onDismiss = { openPerson = null },
                onSave = { name, emoji, kind, monitoring ->
                    // Depois de cadastrar, a folha continua aberta para vincular os aparelhos.
                    vm.savePerson(person?.id, name, emoji, kind, monitoring) { saved -> openPerson = saved.id }
                },
                onDelete = {
                    if (person != null) vm.deletePerson(person)
                    openPerson = null
                },
                onAssign = { deviceId, personId -> vm.assign(deviceId, personId) },
            )
        }
    }
}
